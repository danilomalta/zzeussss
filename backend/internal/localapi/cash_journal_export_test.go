package localapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"errors"
	"io"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/cashjournal"
)

const cashCSVPath = cashJournalPath + ".csv"

func exportJournal(t *testing.T, f *stockHTTPFixture, path string, want int) ([]byte, []map[string]string) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	resp, err := f.app.Test(req, 10000)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("CSV %d want %d: %s", resp.StatusCode, want, data)
	}
	if want != 200 {
		if resp.Header.Get("Content-Disposition") != "" || bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) || bytes.Contains(data, []byte("format_version")) {
			t.Fatal("partial download on failure", string(data))
		}
		return data, nil
	}
	if resp.Header.Get("Content-Type") != "text/csv; charset=utf-8" || resp.Header.Get("Content-Disposition") != `attachment; filename="titan-caixa-lancamentos.csv"` || resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(resp.Header)
	}
	if !bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) || !bytes.Contains(data, []byte("\r\n")) {
		t.Fatal("missing UTF8 BOM/CRLF")
	}
	r := csv.NewReader(bytes.NewReader(data[3:]))
	r.Comma = ';'
	records, err := r.ReadAll()
	if err != nil || len(records) < 2 {
		t.Fatal("invalid rectangular CSV", err)
	}
	var rows []map[string]string
	for _, values := range records[1:] {
		row := map[string]string{}
		for i, name := range records[0] {
			row[name] = values[i]
		}
		rows = append(rows, row)
	}
	return data, rows
}

func TestHTTPJournalExportReconcilesRealCycleWithoutDuplicatingCashOrTroco(t *testing.T) {
	f, _ := cashPlanSetup(t)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
	cashRequest(t, f.httpContractFixture, "movements", withdrawalBody, 200)
	if status, body := request(t, f.app, "POST", "/local/v1/sales/"+cashPlanID+"/cancel", cancelBody, f.token); status != 200 {
		t.Fatal(status, string(body))
	}
	cashRequest(t, f.httpContractFixture, "close", strings.Replace(cashCloseBody, "1200", "1202", 1), 200)
	page := readJournal(t, f, "", 200)
	before := isolationCounts(t, f.db)
	want, rows := exportJournal(t, f, cashCSVPath, 200)
	if len(rows) != 6 || rows[0]["row_type"] != "summary" || rows[0]["total_entries"] != "5" || rows[0]["expected_cents"] != "1200" || rows[0]["declared_cents"] != "1202" || rows[0]["difference_cents"] != "2" || rows[0]["total_debit_cents"] != "8196" || rows[0]["total_credit_cents"] != "8196" {
		t.Fatal(rows)
	}
	var net, turnover int64
	for i, row := range rows[1:] {
		e := page.Items[i]
		if row["row_type"] != "entry" || row["scope"] != "cash_operational" || row["format_version"] != "1" || row["session_id"] != page.SessionID || row["entry_id"] != e.ID || row["source_id"] != e.SourceID || row["source_kind"] != e.SourceKind || row["category"] != e.Category || row["operation_id"] != *e.OperationID || row["actor_id"] != *e.ActorID || row["debit_account"] != e.Legs[0].Account || row["credit_account"] != e.Legs[1].Account {
			t.Fatal(row, e)
		}
		delta, err := strconv.ParseInt(row["cash_delta_cents"], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		net += delta
		amount, err := strconv.ParseInt(row["amount_cents"], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		turnover += amount
		if row["total_debit_cents"] != "" || row["declared_cents"] != "" {
			t.Fatal("summary duplicated in entries")
		}
	}
	if net != 1200 || turnover != 8196 {
		t.Fatal(net, turnover)
	}
	for i := 0; i < 2; i++ {
		got, _ := exportJournal(t, f, cashCSVPath, 200)
		if !bytes.Equal(got, want) {
			t.Fatal("repeat export changed bytes")
		}
	}
	if !reflect.DeepEqual(before, isolationCounts(t, f.db)) {
		t.Fatal("export wrote business data")
	}
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"second-turn","opening_cents":9900}`, 201)
	if got, _ := exportJournal(t, f, cashCSVPath, 200); !bytes.Equal(got, want) {
		t.Fatal("export borrowed current drawer")
	}
}

func TestHTTPJournalExportScopeBlindClosingAndUnsupportedRequests(t *testing.T) {
	f := salesSetup(t)
	exportJournal(t, f, cashCSVPath, 409)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	for _, q := range []string{"?limit=1", "?offset=50", "?tenant_id=other", "?download=true"} {
		exportJournal(t, f, cashCSVPath+q, 400)
	}
	if status, _ := request(t, f.app, "GET", cashCSVPath, `{}`, f.token); status != 400 {
		t.Fatal(status)
	}
	exportJournal(t, f, "/local/v1/cash/sessions/missing/journal.csv", 404)
	other := journalActor(f)
	other.IdentityID = "another-cashier"
	policyPerson(t, f.db, f.owner, other.IdentityID, "cashier")
	if _, err := cashjournal.ExportCSV(context.Background(), f.db, other, f.device, "turn-one"); !errors.Is(err, cashjournal.ErrNotFound) {
		t.Fatal("foreign cashier exported", err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	exportJournal(t, f, cashCSVPath, 200)
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := request(t, app, "GET", cashCSVPath, "", f.token); status != 200 {
		t.Fatal("history required verifier", status)
	}
	if _, err = f.db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE tenant_id=?`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	exportJournal(t, f, cashCSVPath, 401)
}

// This is a bounded-volume fixture, not proof that 10000 API sales occurred.
// Every synthetic source has its audit and immutable posting linked correctly.
func bulkJournalSources(t *testing.T, f *stockHTTPFixture, count int) {
	t.Helper()
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`WITH RECURSIVE nums(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM nums WHERE n<?)
 INSERT INTO cash_movements SELECT 'bulk-'||n,?,?, 'turn-one',1,'supply:volume','2026-10-17T10:00:00Z' FROM nums`, count, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO cash_adjustments SELECT tenant_id,store_id,?,id,'turn-one',?,'supply',1,'volume',id,created_at FROM cash_movements WHERE id LIKE 'bulk-%'`, f.device.DeviceID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO cash_journal_entries(tenant_id,store_id,device_id,session_id,entry_id,source_kind,source_id,movement_id,cash_delta_cents,occurred_at,category,operation_id,actor_identity_id,counter_account)
 SELECT tenant_id,store_id,device_id,session_id,entry_id,source_kind,source_id,movement_id,cash_delta_cents,occurred_at,category,operation_id,actor_identity_id,counter_account FROM cash_journal_sources WHERE source_id LIKE 'bulk-%' ORDER BY source_id`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPJournalExportIncludesEveryPageAndEnforcesWholeFileLimit(t *testing.T) {
	for _, count := range []int{75, cashjournal.MaxExportEntries - 1, cashjournal.MaxExportEntries} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			f := salesSetup(t)
			bulkJournalSources(t, f, count)
			cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
			want := 200
			if count+1 > cashjournal.MaxExportEntries {
				want = 413
			}
			_, rows := exportJournal(t, f, cashCSVPath, want)
			if want == 200 {
				if len(rows) != count+2 || rows[0]["total_entries"] != strconv.Itoa(count+1) {
					t.Fatal("export truncated to page", len(rows), count)
				}
				ids := map[string]bool{}
				for _, row := range rows[1:] {
					if ids[row["entry_id"]] {
						t.Fatal("duplicate export row")
					}
					ids[row["entry_id"]] = true
				}
			}
			// Export bound must not change the JSON history pagination contract.
			page := readJournal(t, f, "?limit=1", 200)
			if len(page.Items) != 1 || page.TotalCount != int64(count+1) || !page.HasMore {
				t.Fatal(page)
			}
		})
	}
}

func TestHTTPJournalExportRefusesInconsistencyAndUnsafeTextWithoutPartialDownload(t *testing.T) {
	for _, damage := range []string{"source", "orphan_entry", "precision", "text"} {
		t.Run(damage, func(t *testing.T) {
			f := salesSetup(t)
			want := 409
			supply := supplyBody
			if damage == "text" {
				supply = strings.Replace(supply, "supply-one", "=1+1", 1)
				want = 422
			}
			if damage == "precision" {
				supply = strings.Replace(supply, "500", "9007199254740992", 1)
				want = 422
			}
			cashRequest(t, f.httpContractFixture, "movements", supply, 200)
			cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
			if damage == "source" {
				if _, err := f.db.Exec(`UPDATE cash_movements SET amount_cents=1`); err != nil {
					t.Fatal(err)
				}
			}
			if damage == "orphan_entry" {
				// Deliberately damaged temporary file: exercise the reverse
				// source check rather than a missing-posting check.
				if _, err := f.db.Exec(`PRAGMA foreign_keys=OFF; DELETE FROM cash_movements; PRAGMA foreign_keys=ON`); err != nil {
					t.Fatal(err)
				}
			}
			before := isolationCounts(t, f.db)
			exportJournal(t, f, cashCSVPath, want)
			if !reflect.DeepEqual(before, isolationCounts(t, f.db)) {
				t.Fatal("export repaired source")
			}
			if damage == "text" {
				readJournal(t, f, "", 200)
			}
		})
	}
}

func TestHTTPJournalExportSurvivesBackupReopenAndConcurrentReads(t *testing.T) {
	f, _ := cashPlanSetup(t)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	want, _ := exportJournal(t, f, cashCSVPath, 200)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, _ := exportJournal(t, f, cashCSVPath, 200); !bytes.Equal(got, want) {
				t.Error("concurrent export changed")
			}
		}()
	}
	wg.Wait()
	ctx := context.Background()
	dir := t.TempDir()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "export.tytbak")
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "restored.sqlite")
	if err := backup.Restore(ctx, archive, path, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := cashjournal.ExportCSV(ctx, db, journalActor(f), f.device, "turn-one")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("restored/reopened export changed", err)
	}
}
