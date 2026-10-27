package localapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/identity"
)

const cashJournalPath = "/local/v1/cash/sessions/turn-one/journal"

func readJournal(t *testing.T, f *stockHTTPFixture, suffix string, want int) cashjournal.Page {
	t.Helper()
	status, body := request(t, f.app, "GET", cashJournalPath+suffix, "", f.token)
	if status != want {
		t.Fatalf("journal%s: %d want %d: %s", suffix, status, want, body)
	}
	var page cashjournal.Page
	if want == 200 {
		if err := json.Unmarshal(body, &page); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-journal.openapi.json", "JournalPage", body)
	} else if bytes.Contains(body, []byte("expected_cents")) || bytes.Contains(body, []byte("debit_cents")) {
		t.Fatal("failed journal returned usable totals", string(body))
	}
	return page
}

func journalCount(t *testing.T, f *stockHTTPFixture, want int) {
	t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM cash_journal_entries`).Scan(&n); err != nil || n != want {
		t.Fatalf("postings %d want %d: %v", n, want, err)
	}
}

func journalActor(f *stockHTTPFixture) identity.Scope {
	return identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
}

func TestHTTPJournalCashCyclePreservesReversalAndBalancesNetCashOnce(t *testing.T) {
	f, _ := cashPlanSetup(t)
	readJournal(t, f, "", 409)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 200)
	cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
	cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
	cashRequest(t, f.httpContractFixture, "movements", withdrawalBody, 200)
	for i := 0; i < 2; i++ {
		if status, body := request(t, f.app, "POST", "/local/v1/sales/"+cashPlanID+"/cancel", cancelBody, f.token); status != 200 {
			t.Fatalf("refund %d %s", status, body)
		}
	}
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	page := readJournal(t, f, "", 200)
	if page.SessionID != "turn-one" || page.Scope != "cash_operational" || !page.SourceComplete || page.TotalCount != 5 || page.HasMore || page.NextOffset != nil || page.Summary != (cashjournal.Summary{CashBalanceCents: 1200, ExpectedCents: 1200, DeclaredCents: 1200, DebitCents: 8196, CreditCents: 8196}) {
		t.Fatal(page)
	}
	wantCategories := []string{"opening", "sale", "supply", "withdrawal", "sale_reversal"}
	wantDeltas := []int64{1000, 3198, 500, -300, -3198}
	for i, e := range page.Items {
		if e.Category != wantCategories[i] || e.CashDeltaCents != wantDeltas[i] || e.ActorID == nil || *e.ActorID != f.owner.OwnerID || e.OperationID == nil || len(e.Legs) != 2 || e.Legs[0].Side != "debit" || e.Legs[1].Side != "credit" || e.Legs[0].AmountCents != e.Legs[1].AmountCents {
			t.Fatal(e)
		}
		if e.CashDeltaCents > 0 && e.Legs[0].Account != "cash_drawer" || e.CashDeltaCents < 0 && e.Legs[1].Account != "cash_drawer" {
			t.Fatal(e)
		}
	}
	journalCount(t, f, 5)
	assertStockBalance(t, f, f.shelf, 5000)
	before := isolationCounts(t, f.db)
	var all []cashjournal.Entry
	for _, q := range []string{"?limit=2&offset=0", "?limit=2&offset=2", "?limit=2&offset=4"} {
		p := readJournal(t, f, q, 200)
		if p.Summary != page.Summary || p.TotalCount != 5 || !p.SourceComplete || p.HasMore != (p.Offset < 4) {
			t.Fatal(p)
		}
		if p.HasMore && (p.NextOffset == nil || *p.NextOffset != p.Offset+len(p.Items)) {
			t.Fatal(p)
		}
		all = append(all, p.Items...)
	}
	if !reflect.DeepEqual(all, page.Items) {
		t.Fatal("pagination duplicated or lost a posting")
	}
	if p := readJournal(t, f, "?offset=100", 200); p.HasMore || p.NextOffset != nil || len(p.Items) != 0 || p.Summary != page.Summary {
		t.Fatal(p)
	}
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"next-turn","opening_cents":9900}`, 201)
	if got := readJournal(t, f, "", 200); !reflect.DeepEqual(got, page) {
		t.Fatal("historical journal borrowed new shift", got)
	}
	if reflect.DeepEqual(before, isolationCounts(t, f.db)) {
		t.Fatal("next shift did not open")
	}
}

func TestHTTPJournalScopeValidationAndHistoryWithoutPOSLicense(t *testing.T) {
	f := salesSetup(t)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	for _, q := range []string{"?limit=0", "?limit=101", "?limit=-1", "?limit=1.5", "?offset=-1", "?offset=1000000001", "?offset=9999999999999999999999", "?limit=2&limit=3", "?tenant_id=other", "?offset=", "?limit=%2B1"} {
		readJournal(t, f, q, 400)
	}
	if status, _ := request(t, f.app, "GET", cashJournalPath, `{}`, f.token); status != 400 {
		t.Fatal(status)
	}
	if status, _ := request(t, f.app, "GET", cashJournalPath, "", ""); status != 401 {
		t.Fatal(status)
	}
	if status, _ := request(t, f.app, "GET", "/local/v1/cash/sessions/missing/journal", "", f.token); status != 404 {
		t.Fatal(status)
	}
	policyPerson(t, f.db, f.owner, "another-cashier", "cashier")
	other := journalActor(f)
	other.IdentityID = "another-cashier"
	if _, err := cashjournal.Read(context.Background(), f.db, other, f.device, "turn-one", 50, 0); !errors.Is(err, cashjournal.ErrNotFound) {
		t.Fatal("other operator read history", err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	readJournal(t, f, "", 200)
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := request(t, app, "GET", cashJournalPath, "", f.token); status != 200 {
		t.Fatal("history required POS verifier", status)
	}
	if _, err = f.db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	readJournal(t, f, "", 401)
}

func TestHTTPJournalRejectsHistoryMutationAndNeverRepairsReads(t *testing.T) {
	for _, damage := range []string{"source_amount", "source_actor", "missing_posting", "closure"} {
		t.Run(damage, func(t *testing.T) {
			f := salesSetup(t)
			cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
			cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
			readJournal(t, f, "", 200)
			for _, q := range []string{`UPDATE cash_journal_entries SET cash_delta_cents=7`, `DELETE FROM cash_journal_entries`} {
				if _, err := f.db.Exec(q); err == nil {
					t.Fatal("journal history editable", q)
				}
			}
			switch damage {
			case "source_amount":
				if _, err := f.db.Exec(`UPDATE cash_movements SET amount_cents=1`); err != nil {
					t.Fatal(err)
				}
			case "source_actor":
				policyPerson(t, f.db, f.owner, "other-manager", "manager")
				if _, err := f.db.Exec(`UPDATE cash_adjustments SET actor_identity_id='other-manager'`); err != nil {
					t.Fatal(err)
				}
			case "missing_posting":
				if _, err := f.db.Exec(`INSERT INTO cash_movements VALUES ('untracked',?,?, 'turn-one',1,'legacy','now')`, f.owner.TenantID, f.owner.StoreID); err != nil {
					t.Fatal(err)
				}
			case "closure":
				if _, err := f.db.Exec(`UPDATE cash_closures SET expected_cents=1`); err != nil {
					t.Fatal(err)
				}
			}
			before := isolationCounts(t, f.db)
			readJournal(t, f, "", 409)
			readJournal(t, f, "", 409)
			journalCount(t, f, 2)
			if !reflect.DeepEqual(before, isolationCounts(t, f.db)) {
				t.Fatal("read repaired source facts")
			}
		})
	}
}

func TestHTTPJournalInsertFailureRollsBackSourceStockAuditAndOutbox(t *testing.T) {
	for _, category := range []string{"opening", "sale", "supply", "sale_reversal"} {
		for _, failure := range []string{"ABORT,'injected'", "IGNORE"} {
			t.Run(category+failure, func(t *testing.T) {
				f, _ := cashPlanSetup(t)
				if category == "sale_reversal" {
					planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
				}
				before := isolationCounts(t, f.db)
				if _, err := f.db.Exec(`CREATE TRIGGER injected BEFORE INSERT ON cash_journal_entries WHEN NEW.category='` + category + `' BEGIN SELECT RAISE(` + failure + `); END`); err != nil {
					t.Fatal(err)
				}
				if category == "opening" {
					cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
					before = isolationCounts(t, f.db)
				}
				act := func(want int) {
					switch category {
					case "opening":
						cashRequest(t, f.httpContractFixture, "open", `{"session_id":"new-turn","opening_cents":0}`, want)
					case "sale":
						planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, want)
					case "supply":
						cashRequest(t, f.httpContractFixture, "movements", supplyBody, want)
					case "sale_reversal":
						status, body := request(t, f.app, "POST", "/local/v1/sales/"+cashPlanID+"/cancel", cancelBody, f.token)
						if status != want {
							t.Fatalf("refund %d want %d %s", status, want, body)
						}
					}
				}
				act(500)
				if !reflect.DeepEqual(before, isolationCounts(t, f.db)) {
					t.Fatal("failed journal left source or outbox")
				}
				wantCount := 1
				wantStock := int64(5000)
				if category == "sale_reversal" {
					wantCount = 2
					wantStock = 3000
				}
				journalCount(t, f, wantCount)
				assertStockBalance(t, f, f.shelf, wantStock)
				if category == "sale" {
					cashFinalCounts(t, f, 0)
				}
				if category == "sale_reversal" {
					if r := readReceipt(t, f, cashPlanID); r.Status != "committed" {
						t.Fatal(r)
					}
				}
				if _, err := f.db.Exec(`DROP TRIGGER injected`); err != nil {
					t.Fatal(err)
				}
				want := 200
				if category == "opening" || category == "sale" {
					want = 201
				}
				act(want)
				journalCount(t, f, wantCount+1)
			})
		}
	}
}

func TestHTTPJournalBackupRestartAndConcurrentReplayPreserveIDs(t *testing.T) {
	f, _ := cashPlanSetup(t)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":4198}`, 200)
	want := readJournal(t, f, "", 200)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 200)
			if got := readJournal(t, f, "", 200); !reflect.DeepEqual(got, want) {
				t.Error("concurrent replay changed postings")
			}
		}()
	}
	wg.Wait()
	journalCount(t, f, 2)
}

func TestHTTPJournalBackupRestoresImmutablePostings(t *testing.T) {
	f, _ := cashPlanSetup(t)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":4200}`, 200)
	want := readJournal(t, f, "", 200)
	if want.Summary.DifferenceCents != 2 || want.TotalCount != 2 {
		t.Fatal("count difference was auto posted", want)
	}
	ctx := context.Background()
	dir := t.TempDir()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "journal.tytbak")
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
	got, err := cashjournal.Read(ctx, db, journalActor(f), f.device, "turn-one", 50, 0)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("restored journal %+v %v", got, err)
	}
	if _, err = db.Exec(`DELETE FROM cash_journal_entries`); err == nil {
		t.Fatal("restore lost immutability")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err = cashjournal.Read(ctx, db, journalActor(f), f.device, "turn-one", 50, 0)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("restart changed journal", err)
	}
}

func TestHTTPJournalWholeSessionPrecisionAndZeroOpening(t *testing.T) {
	f := salesSetup(t)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"zero","opening_cents":0}`, 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"zero","operation_id":"close-zero","declared_cents":0}`, 200)
	p, err := cashjournal.Read(context.Background(), f.db, journalActor(f), f.device, "zero", 1, 0)
	if err != nil || p.TotalCount != 1 || p.Summary.DebitCents != 0 || p.Items[0].Legs[0].AmountCents != 0 {
		t.Fatal(p, err)
	}
	// Net balance fits, but total debits/credits cannot be represented exactly
	// by the browser. The entire response, including pages, must fail closed.
	amount := "4503599627370496"
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"large","opening_cents":0}`, 201)
	body := `{"session_id":"large","operation_id":"big-in","kind":"supply","amount_cents":` + amount + `,"reason":"precision test"}`
	cashRequest(t, f.httpContractFixture, "movements", body, 200)
	cashRequest(t, f.httpContractFixture, "movements", strings.Replace(strings.Replace(body, "big-in", "big-out", 1), "supply", "withdrawal", 1), 200)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"large","operation_id":"close-large","declared_cents":0}`, 200)
	if status, body := request(t, f.app, "GET", "/local/v1/cash/sessions/large/journal?limit=1&offset=2", "", f.token); status != 422 || bytes.Contains(body, []byte("summary")) {
		t.Fatalf("imprecise summary %d %s", status, body)
	}
}
