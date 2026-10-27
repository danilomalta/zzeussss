package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/sale"
)

func customerReturnInput(t *testing.T, f *stockHTTPFixture, operation string, quantity, amount int64, disposition string) sale.CustomerReturnInput {
	t.Helper()
	r := readReceipt(t, f, "sale-one")
	return sale.CustomerReturnInput{OperationID: operation, SaleID: "sale-one", Reason: "Cliente devolveu mercadoria conferida", ExpectedRefundCents: amount, Items: []sale.ReturnItemInput{{SaleItemID: r.Items[0].ItemID, QuantityMilli: quantity, Disposition: disposition}}}
}
func customerReturnJSON(t *testing.T, in sale.CustomerReturnInput) string {
	t.Helper()
	b, e := json.Marshal(map[string]any{"operation_id": in.OperationID, "reason": in.Reason, "expected_refund_cents": in.ExpectedRefundCents, "items": in.Items})
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func customerReturnRequest(t *testing.T, f *stockHTTPFixture, in sale.CustomerReturnInput, want int) sale.CustomerReturn {
	t.Helper()
	status, b := request(t, f.app, "POST", "/local/v1/sales/"+in.SaleID+"/returns", customerReturnJSON(t, in), f.token)
	if status != want {
		t.Fatalf("return status %d want %d: %s", status, want, b)
	}
	var v sale.CustomerReturn
	if status == 200 || status == 201 {
		domainSchemaFile(t, "customer-returns.openapi.json", "CustomerReturn", b)
		if e := json.Unmarshal(b, &v); e != nil {
			t.Fatal(e)
		}
	}
	return v
}
func customerReturnHistory(t *testing.T, f *stockHTTPFixture, want int) sale.CustomerReturnHistory {
	t.Helper()
	status, b := request(t, f.app, "GET", "/local/v1/sales/sale-one/returns", "", f.token)
	if status != want {
		t.Fatalf("history %d want %d: %s", status, want, b)
	}
	var h sale.CustomerReturnHistory
	if status == 200 {
		domainSchemaFile(t, "customer-returns.openapi.json", "ReturnHistory", b)
		if e := json.Unmarshal(b, &h); e != nil {
			t.Fatal(e)
		}
	}
	return h
}
func assertCustomerReturnCount(t *testing.T, f *stockHTTPFixture, want int) {
	t.Helper()
	for _, q := range []string{`SELECT count(*) FROM customer_returns`, `SELECT count(*) FROM customer_return_items`, `SELECT count(*) FROM outbox WHERE event_type='sale.customer_returned'`, `SELECT count(*) FROM cash_movements WHERE reason LIKE 'devolucao-cliente:%'`} {
		var n int
		if e := f.db.QueryRow(q).Scan(&n); e != nil || n != want {
			t.Fatalf("%s: %d want %d err %v", q, n, want, e)
		}
	}
}

func TestHTTPCustomerReturnPartialCashStockAndOfficialClosure(t *testing.T) {
	f := cancellationSetup(t)
	original := readReceipt(t, f, "sale-one")
	in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
	qb, _ := json.Marshal(map[string]any{"items": in.Items})
	status, b := request(t, f.app, "POST", "/local/v1/sales/sale-one/returns/quote", string(qb), f.token)
	if status != 200 {
		t.Fatalf("quote %d %s", status, b)
	}
	domainSchemaFile(t, "customer-returns.openapi.json", "ReturnQuote", b)
	var q sale.CustomerReturnQuote
	json.Unmarshal(b, &q)
	if q.RefundCents != 1599 {
		t.Fatal(q)
	}
	assertCustomerReturnCount(t, f, 0)
	r := customerReturnRequest(t, f, in, 201)
	if r.RefundCents != 1599 || r.Repeated || r.FiscalAuthorized || r.Items[0].ReturnMovementID == nil {
		t.Fatal(r)
	}
	repeated := customerReturnRequest(t, f, in, 200)
	if !repeated.Repeated || repeated.ID != r.ID || repeated.CreatedAt != r.CreatedAt {
		t.Fatal(repeated)
	}
	assertStockBalance(t, f, f.shelf, 4000)
	assertCustomerReturnCount(t, f, 1)
	h := customerReturnHistory(t, f, 200)
	if h.RefundedCents != 1599 || h.RemainingCents != 1599 || h.Items[0].RemainingMilli != 1000 || !h.Complete {
		t.Fatal(h)
	}
	after := readReceipt(t, f, "sale-one")
	if after.Status != "committed" || after.TotalCents != original.TotalCents || after.Items[0] != original.Items[0] || after.Payments[0] != original.Payments[0] {
		t.Fatal(after)
	}
	cancelRequest(t, f, cancelBody, 409)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-partial-return","declared_cents":2599}`, 200)
	a := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	closed, e := register.ReadClosedReport(context.Background(), f.db, a, f.device, "turn-one")
	if e != nil {
		t.Fatal(e)
	}
	if closed.GrossSalesCents != 3198 || closed.CancelledSalesCents != 0 || closed.ReturnedSalesCents != 1599 || closed.NetSalesCents != 1599 || closed.Cash.CustomerRefundCents != 1599 || closed.Cash.ExpectedCents != 2599 || closed.Payments[0].ConfirmedCents != 3198 {
		t.Fatal(closed)
	}
	j, e := cashjournal.Read(context.Background(), f.db, a, f.device, "turn-one", 100, 0)
	if e != nil || j.Summary.UnclassifiedCount != 0 || j.Summary.CashBalanceCents != 2599 {
		t.Fatal(j, e)
	}
	customerReturnRequest(t, f, in, 200)
	in.OperationID = "return-after-close"
	customerReturnRequest(t, f, in, 409)
}

func TestHTTPCustomerReturnDamagedGoodsNeverReenterStock(t *testing.T) {
	f := cancellationSetup(t)
	first := customerReturnInput(t, f, "damaged-one", 1000, 1599, "no_restock")
	r := customerReturnRequest(t, f, first, 201)
	if r.Items[0].ReturnMovementID != nil {
		t.Fatal(r)
	}
	assertStockBalance(t, f, f.shelf, 3000)
	first.OperationID = "restock-remainder"
	first.Items[0].Disposition = "restock"
	customerReturnRequest(t, f, first, 201)
	assertStockBalance(t, f, f.shelf, 4000)
	h := customerReturnHistory(t, f, 200)
	if h.RemainingCents != 0 || h.Items[0].RemainingMilli != 0 || len(h.Returns) != 2 {
		t.Fatal(h)
	}
	first.OperationID = "third-refund"
	customerReturnRequest(t, f, first, 409)
	cancelRequest(t, f, cancelBody, 409)
}

func TestHTTPCustomerReturnRejectsExcessMismatchedTotalAndOperationReuse(t *testing.T) {
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "return-one", 3000, 4797, "restock")
	customerReturnRequest(t, f, in, 409)
	in.Items[0].QuantityMilli = 1000
	in.ExpectedRefundCents = 1600
	customerReturnRequest(t, f, in, 409)
	in.ExpectedRefundCents = 1599
	in.OperationID = "sale-operation-one"
	customerReturnRequest(t, f, in, 409)
	in.OperationID = "return-one"
	customerReturnRequest(t, f, in, 201)
	in.Reason = "outro motivo"
	customerReturnRequest(t, f, in, 409)
	other := salesInput(f)
	other.OperationID = "return-one"
	other.SaleID = "other-sale"
	salesRequest(t, f, other, 409)
	cancelRequest(t, f, `{"operation_id":"return-one","reason":"outro uso"}`, 409)
	assertCustomerReturnCount(t, f, 1)
}

func TestHTTPCustomerReturnRequiresAuthorizationAndContract(t *testing.T) {
	for _, mode := range []string{"anonymous", "cashier", "revoked", "expired-pos", "other-device"} {
		t.Run(mode, func(t *testing.T) {
			f := cancellationSetup(t)
			in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
			want := 403
			switch mode {
			case "anonymous":
				f.token = ""
				want = 401
			case "cashier":
				_, e := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID)
				if e != nil {
					t.Fatal(e)
				}
				_, e = f.db.Exec(`UPDATE memberships SET role='cashier' WHERE identity_id=?`, f.owner.OwnerID)
				if e != nil {
					t.Fatal(e)
				}
			case "revoked":
				_, e := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id=?`, f.owner.OwnerID)
				if e != nil {
					t.Fatal(e)
				}
				want = 401
			case "expired-pos":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
			case "other-device":
				_, e := f.db.Exec(`INSERT INTO devices VALUES (?,?,'different-device','Outro')`, f.owner.TenantID, f.owner.StoreID)
				if e != nil {
					t.Fatal(e)
				}
				_, e = f.db.Exec(`UPDATE sales SET device_id='different-device' WHERE id='sale-one'`)
				if e != nil {
					t.Fatal(e)
				}
				want = 404
			}
			customerReturnRequest(t, f, in, want)
			assertCustomerReturnCount(t, f, 0)
		})
	}
}

func TestHTTPCustomerReturnStrictBodyRejectsForgedContextAndConfirmation(t *testing.T) {
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
	valid := customerReturnJSON(t, in)
	for _, body := range []string{`null`, `{}`, valid + ` {}`, strings.Replace(valid, `"operation_id":`, `"operation_id":"other","operation_id":`, 1), strings.Replace(valid, `"expected_refund_cents":1599`, `"expected_refund_cents":1599.5`, 1), strings.TrimSuffix(valid, "}") + `,"provider_confirmed":true}`, strings.TrimSuffix(valid, "}") + `,"tenant_id":"foreign"}`, strings.Replace(valid, `"disposition":"restock"`, `"disposition":null`, 1), strings.Replace(valid, `"quantity_milli":1000`, `"quantity_milli":0`, 1)} {
		status, b := request(t, f.app, "POST", "/local/v1/sales/sale-one/returns", body, f.token)
		if status != 400 {
			t.Fatalf("body %s status %d %s", body, status, b)
		}
	}
	assertCustomerReturnCount(t, f, 0)
}

func TestHTTPCustomerReturnFailureRollsBackCashStockHistoryAndOutbox(t *testing.T) {
	for _, table := range []string{"cash_movements", "customer_returns", "stock_movements", "customer_return_items", "cash_journal_entries", "outbox"} {
		for _, failure := range []string{"ABORT", "IGNORE"} {
			t.Run(table+failure, func(t *testing.T) {
				f := cancellationSetup(t)
				in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
				trigger := fmt.Sprintf(`CREATE TRIGGER fail_refund BEFORE INSERT ON %s BEGIN SELECT RAISE(%s%s); END`, table, failure, map[string]string{"ABORT": ",'injected_failure'", "IGNORE": ""}[failure])
				if _, e := f.db.Exec(trigger); e != nil {
					t.Fatal(e)
				}
				status, _ := request(t, f.app, "POST", "/local/v1/sales/sale-one/returns", customerReturnJSON(t, in), f.token)
				if status != 409 && status != 500 {
					t.Fatalf("failure accepted: %d", status)
				}
				assertCustomerReturnCount(t, f, 0)
				assertStockBalance(t, f, f.shelf, 3000)
				assertCancellationUnchanged(t, f)
			})
		}
	}
}

func TestHTTPCustomerReturnConcurrentReplaysAndCompetingReturns(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			f := cancellationSetup(t)
			in := customerReturnInput(t, f, "return-one", 2000, 3198, "restock")
			body := customerReturnJSON(t, in)
			var wg sync.WaitGroup
			statuses := make(chan int, 2)
			for i := 0; i < 2; i++ {
				other := body
				if !same && i == 1 {
					other = strings.Replace(body, "return-one", "return-two", 1)
				}
				wg.Add(1)
				go func(b string) {
					defer wg.Done()
					status, _ := request(t, f.app, "POST", "/local/v1/sales/sale-one/returns", b, f.token)
					statuses <- status
				}(other)
			}
			wg.Wait()
			close(statuses)
			created, repeated, conflicts := 0, 0, 0
			for status := range statuses {
				switch status {
				case 201:
					created++
				case 200:
					repeated++
				case 409:
					conflicts++
				default:
					t.Fatalf("concurrency %d", status)
				}
			}
			if created != 1 || same && repeated != 1 || !same && conflicts != 1 {
				t.Fatalf("created %d repeated %d conflicts %d", created, repeated, conflicts)
			}
			assertCustomerReturnCount(t, f, 1)
			assertStockBalance(t, f, f.shelf, 5000)
		})
	}
}

func TestHTTPCustomerReturnRecoveryAfterRestartAndClosedDrawer(t *testing.T) {
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
	r := customerReturnRequest(t, f, in, 201)
	var seq int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	f.db.Close()
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	v, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	app, e := NewWithVerifier(db, f.device, v)
	if e != nil {
		t.Fatal(e)
	}
	f.db, f.app = db, app
	status, b := request(t, f.app, "GET", "/local/v1/sale-return-operations/return-one", "", f.token)
	if status != 200 {
		t.Fatalf("receipt %d %s", status, b)
	}
	var got sale.CustomerReturn
	json.Unmarshal(b, &got)
	if got.ID != r.ID || got.CreatedAt != r.CreatedAt {
		t.Fatal(got)
	}
	customerReturnRequest(t, f, in, 200)
	assertCustomerReturnCount(t, f, 1)
	for _, table := range []string{"customer_returns", "customer_return_items"} {
		if _, e = f.db.Exec("UPDATE " + table + " SET refund_cents=1"); e == nil {
			t.Fatal("immutable table allowed update", table)
		}
	}
}

func TestHTTPCustomerReturnCancelledSaleAndInsufficientDrawerCannotRefund(t *testing.T) {
	for _, cancelled := range []bool{true, false} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			f := cancellationSetup(t)
			in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
			if cancelled {
				cancelRequest(t, f, cancelBody, 200)
			} else {
				if _, e := f.db.Exec(`INSERT INTO cash_movements VALUES('test-drawdown',?,?,'turn-one',-3000,'test-only','now')`, f.owner.TenantID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
			}
			customerReturnRequest(t, f, in, 409)
			assertCustomerReturnCount(t, f, 0)
		})
	}
}

func TestHTTPCustomerReturnHistoricalPricesAndRoundingRemainExact(t *testing.T) {
	f := salesSetup(t)
	if _, e := f.db.Exec(`UPDATE products SET unit='kg',price_cents=1 WHERE id=?`, f.product); e != nil {
		t.Fatal(e)
	}
	in := salesInput(f)
	in.Items[0].QuantityMilli = 1500
	in.Payments[0].AmountCents = 2
	salesRequest(t, f, in, 201)
	if _, e := f.db.Exec(`UPDATE products SET price_cents=99999 WHERE id=?`, f.product); e != nil {
		t.Fatal(e)
	}
	for i, quantity := range []int64{1000, 500} {
		r := customerReturnInput(t, f, fmt.Sprintf("round-%d", i), quantity, 1, "restock")
		customerReturnRequest(t, f, r, 201)
	}
	h := customerReturnHistory(t, f, 200)
	if h.RefundedCents != 2 || h.RemainingCents != 0 || h.Items[0].RemainingMilli != 0 {
		t.Fatal(h)
	}
}
