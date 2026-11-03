package localapi

import (
	"context"
	"crypto/rand"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/cashaccounting"
	"titansystem-backend/internal/localdb/identity"
)

func TestHTTPAccountingRejectsDamagedSourcesEvenOnEmptyPage(t *testing.T) {
	for _, damage := range []string{"closure", "journal", "binding", "closed_time", "opened_time"} {
		t.Run(damage, func(t *testing.T) {
			f, accountant, _ := accountingSetup(t)
			queries := map[string]string{"closure": `UPDATE cash_closures SET expected_cents=1 WHERE session_id='turn-one'`, "journal": `DROP TRIGGER cash_journal_no_delete; DELETE FROM cash_journal_entries WHERE session_id='turn-one' AND category='sale'`, "binding": `UPDATE cash_sessions SET operator_id=(SELECT operator_id FROM cash_sessions WHERE id='turn-one') WHERE id='turn-two'`, "closed_time": `UPDATE cash_sessions SET closed_at='not-a-date' WHERE id='turn-one'`, "opened_time": `UPDATE cash_sessions SET opened_at='2035-01-01T00:00:00Z' WHERE id='turn-one'`}
			if _, err := f.db.Exec(queries[damage]); err != nil {
				t.Fatal(err)
			}
			before := accountingCounts(t, f)
			q := accountingQuery(closedFirst, periodEnd)
			accountingReport(t, f, accountant, q, 409)
			accountingCSV(t, f, accountant, q, 409)
			for _, name := range []string{"sessions", "journal"} {
				if s, b := request(t, f.app, "GET", accountingPath+"/"+name+q+"&offset=999", "", accountant); s != 409 || strings.Contains(string(b), "summary") {
					t.Fatal(name, s, string(b))
				}
			}
			if !reflect.DeepEqual(before, accountingCounts(t, f)) {
				t.Fatal("read repaired source")
			}
		})
	}
}
func TestHTTPAccountingHistoricalOperatorsRemainHistoricalAfterRevocation(t *testing.T) {
	f, accountant, _ := accountingSetup(t)
	if _, err := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id='period-cashier'`); err != nil {
		t.Fatal(err)
	}
	r := accountingReport(t, f, accountant, accountingQuery(closedFirst, periodEnd), 200)
	if len(r.Sessions) != 2 || r.Sessions[1].OperatorID != "period-cashier" || r.Sessions[1].Report.Cash.ExpectedCents != 2100 {
		t.Fatal(r)
	}
}
func TestHTTPAccountingUnsafeSourceIDKeepsJSONButRefusesCSV(t *testing.T) {
	f, accountant, cashier := accountingSetup(t)
	if s, b := request(t, f.app, "POST", "/local/v1/cash/close", `{"session_id":"turn-three","operation_id":"close-three","declared_cents":999}`, cashier); s != 200 {
		t.Fatal(s, string(b))
	}
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"=HYPERLINK(1)","opening_cents":0}`, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"=HYPERLINK(1)","operation_id":"close-unsafe-id","declared_cents":0}`, 200)
	setAccountingClosedAt(t, f, "=HYPERLINK(1)", closedSecond)
	q := accountingQuery(closedFirst, periodEnd)
	r := accountingReport(t, f, accountant, q, 200)
	found := false
	for _, s := range r.Sessions {
		if s.Report.SessionID == "=HYPERLINK(1)" {
			found = true
		}
	}
	if !found {
		t.Fatal("canonical source ID rewritten")
	}
	accountingCSV(t, f, accountant, q, 409)
}
func TestHTTPAccountingBackupRestorationAndConcurrentReadsPreserveSourceRecords(t *testing.T) {
	f, accountant, _ := accountingSetup(t)
	q := accountingQuery(closedFirst, periodEnd)
	want := accountingReport(t, f, accountant, q, 200)
	before := accountingCounts(t, f)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := accountingReport(t, f, accountant, q, 200)
			if !reflect.DeepEqual(got.Summary, want.Summary) || !reflect.DeepEqual(got.Entries, want.Entries) {
				t.Error("parallel read changed sources")
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(before, accountingCounts(t, f)) {
		t.Fatal("reads wrote records")
	}
	ctx := context.Background()
	dir := t.TempDir()
	key := make([]byte, 32)
	rand.Read(key)
	archive := filepath.Join(dir, "cash-period.tytbak")
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
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "period-accountant"}
	got, err := cashaccounting.Read(ctx, db, a, f.device, cashaccounting.Window{From: closedFirst, To: periodEnd})
	if err != nil {
		t.Fatal(err)
	}
	got.GeneratedAt = want.GeneratedAt
	if !reflect.DeepEqual(got, want) {
		t.Fatal("restore changed accounting view", got, want)
	}
}
