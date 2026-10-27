package localapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/sale"
)

const returnShiftPath = "/local/v1/sales/sale-one/returns/cash-session"

func returnShiftBody(in sale.CustomerReturnInput) map[string]any {
	return map[string]any{"operation_id": in.OperationID, "reason": in.Reason, "expected_refund_cents": in.ExpectedRefundCents, "items": in.Items, "refund_cash_session_id": in.RefundCashSessionID}
}
func returnShiftRequest(t *testing.T, f *stockHTTPFixture, in sale.CustomerReturnInput, want int) sale.CustomerReturn {
	t.Helper()
	raw, e := json.Marshal(returnShiftBody(in))
	if e != nil {
		t.Fatal(e)
	}
	status, b := request(t, f.app, "POST", returnShiftPath, string(raw), f.token)
	if status != want {
		t.Fatalf("explicit refund %d want %d: %s", status, want, b)
	}
	var out sale.CustomerReturn
	if want == 200 || want == 201 {
		domainSchemaFile(t, "customer-returns.openapi.json", "CustomerReturn", b)
		if e = json.Unmarshal(b, &out); e != nil {
			t.Fatal(e)
		}
	}
	return out
}
func returnShiftSetup(t *testing.T, opening int64) *stockHTTPFixture {
	t.Helper()
	f := cancellationSetup(t)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-original","declared_cents":4198}`, 200)
	cashRequest(t, f.httpContractFixture, "open", fmt.Sprintf(`{"session_id":"turn-two","opening_cents":%d}`, opening), 201)
	return f
}
func returnShiftReport(t *testing.T, f *stockHTTPFixture, id string) register.ClosedReport {
	t.Helper()
	a := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	v, e := register.ReadClosedReport(context.Background(), f.db, a, f.device, id)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestHTTPCustomerReturnLaterShiftPreservesOriginalClosureAndAccounting(t *testing.T) {
	f := returnShiftSetup(t, 2000)
	before := returnShiftReport(t, f, "turn-one")
	in := customerReturnInput(t, f, "later-refund", 1000, 1599, "restock")
	in.RefundCashSessionID = "turn-two"
	qraw, _ := json.Marshal(map[string]any{"items": in.Items, "refund_cash_session_id": in.RefundCashSessionID})
	status, b := request(t, f.app, "POST", returnShiftPath+"/quote", string(qraw), f.token)
	if status != 200 {
		t.Fatalf("quote %d %s", status, b)
	}
	domainSchemaFile(t, "customer-returns.openapi.json", "ReturnQuote", b)
	var q sale.CustomerReturnQuote
	if e := json.Unmarshal(b, &q); e != nil || q.CashSessionID != "turn-two" || q.OriginalCashSessionID != "turn-one" || q.RefundCents != 1599 {
		t.Fatal(q, e)
	}
	assertCustomerReturnCount(t, f, 0)
	// Legacy routes must never infer the newly opened drawer.
	customerReturnRequest(t, f, customerReturnInput(t, f, "legacy", 1000, 1599, "restock"), 409)
	r := returnShiftRequest(t, f, in, 201)
	if r.CashSessionID != "turn-two" || r.OriginalCashSessionID != "turn-one" || r.RefundCents != 1599 {
		t.Fatal(r)
	}
	if got := returnShiftReport(t, f, "turn-one"); !reflect.DeepEqual(got, before) {
		t.Fatal("original closure changed", got, before)
	}
	h := customerReturnHistory(t, f, 200)
	if h.CashSessionID != "turn-one" || len(h.Returns) != 1 || h.RemainingCents != 1599 || h.Returns[0].ID != r.ID {
		t.Fatal(h)
	}
	assertStockBalance(t, f, f.shelf, 4000)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-two","operation_id":"close-new","declared_cents":401}`, 200)
	closed := returnShiftReport(t, f, "turn-two")
	if closed.GrossSalesCents != 0 || closed.ReturnedSalesCents != 0 || closed.NetSalesCents != 0 || closed.Cash.PriorSessionRefundCents != 1599 || closed.Cash.CustomerRefundCents != 1599 || closed.Cash.ExpectedCents != 401 {
		t.Fatal(closed)
	}
	a := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	j, e := cashjournal.Read(context.Background(), f.db, a, f.device, "turn-two", 100, 0)
	if e != nil || j.Summary.UnclassifiedCount != 0 || j.Summary.CashBalanceCents != 401 {
		t.Fatal(j, e)
	}
	again := returnShiftRequest(t, f, in, 200)
	if !again.Repeated || again.ID != r.ID || again.CreatedAt != r.CreatedAt {
		t.Fatal(again)
	}
	in.RefundCashSessionID = "turn-three"
	returnShiftRequest(t, f, in, 409)
	assertCustomerReturnCount(t, f, 1)
	setAccountingClosedAt(t, f, "turn-one", closedFirst)
	setAccountingClosedAt(t, f, "turn-two", closedSecond)
	token := periodAccountant(t, f)
	report := accountingReport(t, f, token, accountingQuery(closedFirst, periodEnd), 200)
	if report.Summary["prior_session_refund_cents"] != 1599 || report.Summary["returned_sales_cents"] != 0 || report.Summary["net_sales_cents"] != 3198 || report.Summary["unclassified_count"] != 0 {
		t.Fatal(report)
	}
	status, b = request(t, f.app, "GET", "/local/v1/accounting/cash/report.csv"+accountingQuery(closedFirst, periodEnd), "", token)
	if status != 200 || !bytes.Contains(b, []byte("prior_session_refund_cents")) {
		t.Fatalf("CSV %d %s", status, b)
	}
}
func TestHTTPCustomerReturnLaterShiftRejectsAmbiguousInput(t *testing.T) {
	f := returnShiftSetup(t, 2000)
	in := customerReturnInput(t, f, "invalid", 1000, 1599, "restock")
	in.RefundCashSessionID = "turn-two"
	raw, _ := json.Marshal(returnShiftBody(in))
	valid := string(raw)
	for _, body := range []string{`null`, `{}`, strings.Replace(valid, `"turn-two"`, `""`, 1), strings.Replace(valid, `"turn-two"`, `null`, 1), strings.Replace(valid, `"refund_cash_session_id":`, `"refund_cash_session_id":"turn-two","refund_cash_session_id":`, 1), strings.TrimSuffix(valid, "}") + `,"tenant_id":"foreign"}`} {
		status, b := request(t, f.app, "POST", returnShiftPath, body, f.token)
		if status != 400 {
			t.Fatalf("body %s: %d %s", body, status, b)
		}
	}
	for _, value := range []any{"", nil, 123} {
		raw, _ := json.Marshal(map[string]any{"items": in.Items, "refund_cash_session_id": value})
		status, b := request(t, f.app, "POST", returnShiftPath+"/quote", string(raw), f.token)
		if status != 400 {
			t.Fatalf("quote %d %s", status, b)
		}
	}
	assertCustomerReturnCount(t, f, 0)
}
func TestHTTPCustomerReturnLaterShiftRejectsUnauthorizedOrInsufficientDrawer(t *testing.T) {
	for _, mode := range []string{"insufficient", "closed", "missing", "other-device", "other-operator", "cashier", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			opening := int64(2000)
			if mode == "insufficient" {
				opening = 1000
			}
			f := returnShiftSetup(t, opening)
			in := customerReturnInput(t, f, "denied", 1000, 1599, "restock")
			in.RefundCashSessionID = "turn-two"
			want := 409
			switch mode {
			case "closed":
				cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-two","operation_id":"close-new","declared_cents":2000}`, 200)
			case "missing":
				in.RefundCashSessionID = "missing"
			case "other-device":
				if _, e := f.db.Exec(`INSERT INTO devices VALUES (?,?,'other','Other');`, f.owner.TenantID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`INSERT INTO cash_sessions SELECT tenant_id,store_id,'other','other-turn',operator_id,opened_at,NULL,opening_cents FROM cash_sessions WHERE id='turn-two'`); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`INSERT INTO cash_session_operators SELECT tenant_id,store_id,'other-turn',identity_id FROM cash_session_operators WHERE session_id='turn-two'`); e != nil {
					t.Fatal(e)
				}
				in.RefundCashSessionID = "other-turn"
			case "other-operator":
				if _, e := f.db.Exec(`DELETE FROM cash_session_operators WHERE session_id='turn-two'`); e != nil {
					t.Fatal(e)
				}
			case "cashier":
				if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE identity_id=?`, f.owner.OwnerID); e != nil {
					t.Fatal(e)
				}
				want = 403
			case "revoked":
				if _, e := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id=?`, f.owner.OwnerID); e != nil {
					t.Fatal(e)
				}
				want = 401
			}
			returnShiftRequest(t, f, in, want)
			assertCustomerReturnCount(t, f, 0)
			var stock int64
			if e := f.db.QueryRow(`SELECT sum(quantity_milli) FROM stock_movements WHERE location_id=?`, f.shelf).Scan(&stock); e != nil || stock != 3000 {
				t.Fatal(stock, e)
			}
		})
	}
}
func TestHTTPCustomerReturnLaterShiftAtomicRollback(t *testing.T) {
	for _, table := range []string{"customer_return_cross_shifts", "cash_movements", "customer_returns", "customer_return_items", "stock_movements", "cash_journal_entries", "outbox"} {
		for _, failure := range []string{"ABORT", "IGNORE"} {
			t.Run(table+failure, func(t *testing.T) {
				f := returnShiftSetup(t, 2000)
				in := customerReturnInput(t, f, "rollback", 1000, 1599, "restock")
				in.RefundCashSessionID = "turn-two"
				trigger := fmt.Sprintf(`CREATE TRIGGER reject_refund BEFORE INSERT ON %s BEGIN SELECT RAISE(%s%s); END`, table, failure, map[string]string{"ABORT": ",'injected'", "IGNORE": ""}[failure])
				if _, e := f.db.Exec(trigger); e != nil {
					t.Fatal(e)
				}
				raw, _ := json.Marshal(returnShiftBody(in))
				status, _ := request(t, f.app, "POST", returnShiftPath, string(raw), f.token)
				if status != 409 && status != 500 {
					t.Fatalf("injected failure accepted %d", status)
				}
				assertCustomerReturnCount(t, f, 0)
				assertStockBalance(t, f, f.shelf, 3000)
				var n int
				if e := f.db.QueryRow(`SELECT count(*) FROM customer_return_cross_shifts`).Scan(&n); e != nil || n != 0 {
					t.Fatal(n, e)
				}
			})
		}
	}
}
func TestHTTPCustomerReturnLaterShiftConcurrentLimits(t *testing.T) {
	f := returnShiftSetup(t, 10000)
	in := customerReturnInput(t, f, "r-one", 2000, 3198, "restock")
	in.RefundCashSessionID = "turn-two"
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, id := range []string{"r-one", "r-two"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			c := in
			c.OperationID = id
			raw, _ := json.Marshal(returnShiftBody(c))
			s, _ := request(t, f.app, "POST", returnShiftPath, string(raw), f.token)
			codes <- s
		}(id)
	}
	wg.Wait()
	close(codes)
	success := 0
	for s := range codes {
		if s == 201 {
			success++
		} else if s != 409 {
			t.Fatal(s)
		}
	}
	if success != 1 {
		t.Fatal(success)
	}
	assertCustomerReturnCount(t, f, 1)
	assertStockBalance(t, f, f.shelf, 5000)
}
func TestHTTPCustomerReturnLaterShiftRestartAndEncryptedRestore(t *testing.T) {
	f := returnShiftSetup(t, 2000)
	station := pilotPrepareCLIStation(t, f)
	in := customerReturnInput(t, f, "durable-refund", 1000, 1599, "restock")
	in.RefundCashSessionID = "turn-two"
	var seq int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	p := &pilotHTTP{t: t, service: pilotStart(t, path, f, station), client: &http.Client{Timeout: 30 * time.Second}, token: f.token}
	p.token = p.login(f.owner.OwnerID)
	var original sale.CustomerReturn
	p.call("POST", returnShiftPath, returnShiftBody(in), 201, &original)
	p.service.stop(t)
	p.service = pilotStart(t, path, f, station)
	p.call("GET", "/local/v1/me", nil, 200, nil) // Durable session remains valid until revoked or expired.
	p.token = p.login(f.owner.OwnerID)
	var after sale.CustomerReturn
	p.call("GET", "/local/v1/sale-return-operations/durable-refund", nil, 200, &after)
	if !reflect.DeepEqual(after, original) {
		t.Fatal("restart changed receipt")
	}
	p.service.stop(t)
	var restored string
	if *pilotBackupCLI != "" {
		restored = pilotCLIBackupRoundTrip(t, f, path, station)
	} else {
		key := make([]byte, 32)
		if _, e := rand.Read(key); e != nil {
			t.Fatal(e)
		}
		dir := t.TempDir()
		archive := filepath.Join(dir, "refund.tytbak")
		restored = filepath.Join(dir, "restored.sqlite")
		if e := backup.Create(context.Background(), f.db, f.device, key, archive); e != nil {
			t.Fatal(e)
		}
		if e := backup.Restore(context.Background(), archive, restored, f.device, key); e != nil {
			t.Fatal(e)
		}
	}
	db, e := localdb.Open(context.Background(), restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var contexts int
	if e = db.QueryRow(`SELECT count(*) FROM customer_return_cross_shifts`).Scan(&contexts); e != nil || contexts != 1 {
		t.Fatal(contexts, e)
	}
	p.service = pilotStart(t, restored, f, station)
	p.token = p.login(f.owner.OwnerID)
	p.call("GET", "/local/v1/sale-return-operations/durable-refund", nil, 200, &after)
	if !reflect.DeepEqual(after, original) {
		t.Fatal("restore changed IDs")
	}
	p.call("POST", returnShiftPath, returnShiftBody(in), 200, &after)
	if !after.Repeated || after.ID != original.ID {
		t.Fatal(after)
	}
	p.call("POST", "/local/v1/cash/close", map[string]any{"session_id": "turn-two", "operation_id": "close-restored", "declared_cents": 401}, 200, nil)
	var report register.ClosedReport
	p.call("GET", "/local/v1/cash/sessions/turn-two/report", nil, 200, &report)
	if report.Cash.PriorSessionRefundCents != 1599 || report.Cash.ExpectedCents != 401 {
		t.Fatal(report)
	}
	p.service.stop(t)
	t.Logf("CROSS SHIFT ACCEPTANCE: %d real HTTP calls; 3 service process lifetimes; original IDs retained after restart and encrypted restore; no duplicate cash or stock", p.steps)
}
