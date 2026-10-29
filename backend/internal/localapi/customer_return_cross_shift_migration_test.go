package localapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/sale"
)

func TestHTTPCustomerReturnLaterShiftUpgradePopulated65(t *testing.T) {
	// Reconstruct a real historical database from unchanged SQL1-65, then copy
	// only this test's disposable fixture rows. Never downgrade or edit a real DB.
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "legacy-refund", 1000, 1599, "restock")
	original := customerReturnRequest(t, f, in, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-old","declared_cents":2599}`, 200)
	before := returnShiftReport(t, f, "turn-one")
	var sequence int
	var name, source string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&sequence, &name, &source); e != nil {
		t.Fatal(e)
	}
	if e := f.db.Close(); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "historical65.sqlite")
	file, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if e = file.Close(); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT`); e != nil {
		t.Fatal(e)
	}
	files, e := filepath.Glob("../localdb/migrations/*.sql")
	if e != nil {
		t.Fatal(e)
	}
	checksums := map[int]string{}
	for _, file := range files {
		prefix, _, _ := strings.Cut(filepath.Base(file), "_")
		version, e := strconv.Atoi(prefix)
		if e != nil {
			t.Fatal(e)
		}
		if version > 65 {
			continue
		}
		body, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(body)); e != nil {
			t.Fatal(file, e)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(body))
		checksums[version] = checksum
		if _, e = db.Exec(`INSERT INTO schema_migrations VALUES(?,?,'test-fixture')`, version, checksum); e != nil {
			t.Fatal(e)
		}
	}
	if len(checksums) != 65 {
		t.Fatal("historical scripts missing")
	}
	if _, e = db.Exec(`ATTACH DATABASE ? AS fixture`, source); e != nil {
		t.Fatal(e)
	}
	rows, e := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name<>'schema_migrations' ORDER BY rowid`)
	if e != nil {
		t.Fatal(e)
	}
	var tables []string
	for rows.Next() {
		var table string
		if e = rows.Scan(&table); e != nil {
			t.Fatal(e)
		}
		tables = append(tables, table)
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	rows.Close()
	tx, e := db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`PRAGMA defer_foreign_keys=ON`); e != nil {
		t.Fatal(e)
	}
	for _, table := range tables {
		quoted := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
		if _, e = tx.Exec(`INSERT INTO main.` + quoted + ` SELECT * FROM fixture.` + quoted); e != nil {
			t.Fatal(table, e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`DETACH DATABASE fixture`); e != nil {
		t.Fatal(e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	db, e = localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	after, e := sale.ReadCustomerReturnOperation(context.Background(), db, a, f.device, in.OperationID)
	if e != nil || !reflect.DeepEqual(after, original) {
		t.Fatal("legacy receipt changed", e)
	}
	closed, e := register.ReadClosedReport(context.Background(), db, a, f.device, "turn-one")
	if e != nil || !reflect.DeepEqual(closed, before) {
		t.Fatal("legacy closure changed", e)
	}
	for version, expected := range checksums {
		var actual string
		if e = db.QueryRow(`SELECT checksum FROM schema_migrations WHERE version=?`, version).Scan(&actual); e != nil || actual != expected {
			t.Fatal("historical checksum changed", version, e)
		}
	}
	var version int
	if e = db.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); e != nil || version != 74 {
		t.Fatal(version, e)
	}
	t.Log("Populated schema65 -> 67 preserves original return, closed report, IDs and all 65 historical SQL checksums")
}

func TestHTTPCustomerReturnLaterShiftImmutableProvenanceAndDamagedCash(t *testing.T) {
	f := returnShiftSetup(t, 2000)
	in := customerReturnInput(t, f, "proof-refund", 1000, 1599, "restock")
	in.RefundCashSessionID = "turn-two"
	r := returnShiftRequest(t, f, in, 201)
	for _, query := range []string{`UPDATE customer_return_cross_shifts SET actor_identity_id='other'`, `DELETE FROM customer_return_cross_shifts`} {
		if _, e := f.db.Exec(query); e == nil {
			t.Fatal("immutable provenance changed")
		}
	}
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-two","operation_id":"close-proof","declared_cents":401}`, 200)
	if _, e := f.db.Exec(`UPDATE cash_movements SET amount_cents=-1 WHERE id=?`, r.CashMovementID); e != nil {
		t.Fatal(e)
	}
	customerReturnHistory(t, f, 409)
	returnShiftRequest(t, f, in, 409)
	status, _ := request(t, f.app, "GET", "/local/v1/cash/sessions/turn-two/report", "", f.token)
	if status != 409 {
		t.Fatalf("damaged cash report %d", status)
	}
}

func TestHTTPCustomerReturnLaterShiftCombinesEarlierPartialReturn(t *testing.T) {
	f := cancellationSetup(t)
	legacy := customerReturnInput(t, f, "first-in-original", 1000, 1599, "no_restock")
	customerReturnRequest(t, f, legacy, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-mixed-old","declared_cents":2599}`, 200)
	before := returnShiftReport(t, f, "turn-one")
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"turn-two","opening_cents":2000}`, 201)
	later := customerReturnInput(t, f, "remaining-later", 1000, 1599, "restock")
	later.RefundCashSessionID = "turn-two"
	returnShiftRequest(t, f, later, 201)
	h := customerReturnHistory(t, f, 200)
	if h.RemainingCents != 0 || len(h.Returns) != 2 || h.Items[0].RemainingMilli != 0 {
		t.Fatal(h)
	}
	if got := returnShiftReport(t, f, "turn-one"); !reflect.DeepEqual(got, before) || got.ReturnedSalesCents != 1599 || got.NetSalesCents != 1599 {
		t.Fatal(got)
	}
	later.OperationID = "excess-after-two"
	returnShiftRequest(t, f, later, 409)
	assertStockBalance(t, f, f.shelf, 4000)
}
