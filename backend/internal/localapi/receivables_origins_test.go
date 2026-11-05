package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/receivables"
	"titansystem-backend/internal/localdb/retail"
)

// A successful cash withdrawal must never survive without its immutable
// item allocation or audit. These are actual HTTP handlers and SQLite writes.
func TestHTTPRetailRefundIgnoredAllocationRollsBack(t *testing.T) {
	for _, table := range []string{"retail_sale_refunds", "retail_operations"} {
		t.Run(table, func(t *testing.T) {
			f := salesSetup(t)
			f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
			retailEnable(t, f.httpContractFixture, 5000)
			salesRequest(t, f, salesInput(f), 201)
			var item string
			if err := f.db.QueryRow(`SELECT id FROM sale_items WHERE sale_id='sale-one'`).Scan(&item); err != nil {
				t.Fatal(err)
			}
			in := retail.RefundInput{OperationID: "p11-refund", ItemID: item, QuantityMilli: 1000, ExpectedCents: 1599, SessionID: "turn-one", Reason: "Explicit compensation"}
			raw, err := json.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec(`CREATE TRIGGER p11_ignore BEFORE INSERT ON ` + table + ` BEGIN SELECT RAISE(IGNORE); END`); err != nil {
				t.Fatal(err)
			}
			path := "/local/v1/retail/sales/sale-one/refund"
			code, _ := request(t, f.app, "POST", path, string(raw), f.token)
			want := 409
			if table == "retail_operations" {
				want = 500
			} // Existing audit refusal remains unchanged.
			if code != want {
				t.Errorf("missing compensation allocation/audit accepted: %d", code)
			}
			for _, query := range []string{
				`SELECT count(*) FROM cash_adjustments WHERE kind='withdrawal'`,
				`SELECT count(*) FROM retail_sale_refunds`,
				`SELECT count(*) FROM retail_operations WHERE operation_id='p11-refund'`,
			} {
				var n int
				if err := f.db.QueryRow(query).Scan(&n); err != nil || n != 0 {
					t.Errorf("partial financial effect: count=%d err=%v", n, err)
				}
			}
			if _, err := f.db.Exec(`DROP TRIGGER p11_ignore`); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if code, _ := request(t, f.app, "POST", path, string(raw), f.token); code != 204 {
					t.Fatalf("same-ID recovery: %d", code)
				}
			}
			var count, amount int64
			if err := f.db.QueryRow(`SELECT count(*),COALESCE(sum(amount_cents),0) FROM retail_sale_refunds`).Scan(&count, &amount); err != nil || count != 1 || amount != 1599 {
				t.Fatal("compensation duplicated", count, amount, err)
			}
			if err := f.db.QueryRow(`SELECT count(*),COALESCE(sum(amount_cents),0) FROM cash_adjustments WHERE kind='withdrawal'`).Scan(&count, &amount); err != nil || count != 1 || amount != 1599 {
				t.Fatal("withdrawal duplicated", count, amount, err)
			}
			assertStockBalance(t, f, f.shelf, 3000)
			if got := readReceipt(t, f, "sale-one"); got.Status != "committed" {
				t.Fatal("compensation cancelled sale")
			}
		})
	}
}

func TestHTTPRefundCannotOverlapCancellationOrReturn(t *testing.T) {
	for _, action := range []string{"cancel", "return"} {
		for _, concurrent := range []bool{false, true} {
			mode := "sequential"
			if concurrent {
				mode = "concurrent"
			}
			t.Run(action+"/"+mode, func(t *testing.T) {
				f := salesSetup(t)
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
				retailEnable(t, f.httpContractFixture, 5000)
				salesRequest(t, f, salesInput(f), 201)
				item := readReceipt(t, f, "sale-one").Items[0].ItemID
				raw, err := json.Marshal(retail.RefundInput{OperationID: "p11-overlap", ItemID: item, QuantityMilli: 500, ExpectedCents: 799, SessionID: "turn-one", Reason: "Explicit financial compensation"})
				if err != nil {
					t.Fatal(err)
				}
				otherPath, otherBody, success := "/local/v1/sales/sale-one/cancel", cancelBody, 200
				if action == "return" {
					otherPath, success = "/local/v1/sales/sale-one/returns", 201
					otherBody = customerReturnJSON(t, customerReturnInput(t, f, "p11-return", 2000, 3198, "restock"))
				}
				var refundStatus, otherStatus int
				refund := func() {
					refundStatus, _ = request(t, f.app, "POST", "/local/v1/retail/sales/sale-one/refund", string(raw), f.token)
				}
				other := func() { otherStatus, _ = request(t, f.app, "POST", otherPath, otherBody, f.token) }
				if concurrent {
					var wg sync.WaitGroup
					wg.Add(2)
					go func() { defer wg.Done(); refund() }()
					go func() { defer wg.Done(); other() }()
					wg.Wait()
				} else {
					refund()
					other()
				}
				if !((refundStatus == 204 && otherStatus == 409) || (concurrent && refundStatus == 409 && otherStatus == success)) {
					t.Fatalf("overlapping compensation accepted: refund=%d %s=%d", refundStatus, action, otherStatus)
				}
				var effects, cents int64
				if err := f.db.QueryRow(`SELECT (SELECT count(*) FROM retail_sale_refunds)+(SELECT count(*) FROM sale_cancellations)+(SELECT count(*) FROM customer_returns)`).Scan(&effects); err != nil || effects != 1 {
					t.Fatal("multiple compensations", effects, err)
				}
				// cash_adjustments references its cash_movements effect; do not
				// add both representations of the same withdrawal.
				if err := f.db.QueryRow(`SELECT -COALESCE(sum(amount_cents),0) FROM cash_movements WHERE amount_cents<0`).Scan(&cents); err != nil {
					t.Fatal(err)
				}
				want := int64(3198)
				if refundStatus == 204 {
					want = 799
				}
				if cents != want {
					t.Fatal("cash compensation mismatch", cents, want)
				}
			})
		}
	}
}

// Complements the existing order/reconciliation/reversal tests: credit is
// derived from the immutable ledger, not freed twice by a replay after reopen.
func TestHTTPReceivableCreditReversalRestartPreservesOriginAndLimit(t *testing.T) {
	f := salesSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
	retailEnable(t, f.httpContractFixture, 3198)
	in := receivables.CreateInput{OperationID: "p11-credit", ID: "p11-doc", CustomerReference: "customer-external", DocumentReference: "p11-commercial", Description: "Synthetic credit", IssuedOn: "2026-11-04", TotalCents: 3198, Installments: []receivables.Installment{{ID: "one", DueOn: "2026-12-01", AmountCents: 1599}, {ID: "two", DueOn: "2026-12-02", AmountCents: 1599}}, CreditSale: &receivables.CreditSaleInput{SaleID: "p11-sale", SessionID: "turn-one", Items: salesInput(f).Items}}
	body := billingOrderJSON(t, in)
	created := arResult(t, f.httpContractFixture, arPath, body, 201)
	paidBody := `{"operation_id":"p11-paid","installment_id":"one","session_id":"turn-one","amount_cents":1599,"expected_received_cents":0}`
	paid := arResult(t, f.httpContractFixture, arPath+"/p11-doc/receive-cash", paidBody, 200)
	reverse := receivables.ReversalInput{OperationID: "p11-reverse", OriginalOperationID: "p11-paid", OriginalDeviceID: f.device.DeviceID, SessionID: "turn-one", AmountCents: 1, Reason: "Explicit correction"}
	raw, err := json.Marshal(reverse)
	if err != nil {
		t.Fatal(err)
	}
	reversed := arResult(t, f.httpContractFixture, arPath+"/p11-doc/reverse-cash", string(raw), 200)
	if reversed.Document.ReceivedCents != 1598 || reversed.Document.OutstandingCents != 1600 {
		t.Fatal("incorrect net credit")
	}
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	f.db, err = localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.db.Close()
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithVerifier(f.db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct {
		path, body string
		original   receivables.Result
	}{
		{arPath, body, created},
		{arPath + "/p11-doc/receive-cash", paidBody, paid},
		{arPath + "/p11-doc/reverse-cash", string(raw), reversed},
	} {
		got := arResult(t, f.httpContractFixture, v.path, v.body, 200)
		if !got.Repeated || !reflect.DeepEqual(got.Document, v.original.Document) {
			t.Fatal("replay lost original IDs or snapshot")
		}
	}
	in.ID, in.OperationID, in.DocumentReference, in.CreditSale.SaleID = "p11-next", "p11-next", "p11-next", "p11-next-sale"
	in.TotalCents = 1599
	in.Installments = []receivables.Installment{{ID: "next", DueOn: "2026-12-03", AmountCents: 1599}}
	in.CreditSale.Items[0].QuantityMilli = 1000
	arResult(t, f.httpContractFixture, arPath, billingOrderJSON(t, in), 409) // Only 1598 cents free.
	arResult(t, f.httpContractFixture, arPath+"/p11-doc/receive-cash", `{"operation_id":"p11-repaid","installment_id":"one","session_id":"turn-one","amount_cents":1,"expected_received_cents":1598}`, 200)
	arResult(t, f.httpContractFixture, arPath, billingOrderJSON(t, in), 201)
	var count, cash int64
	if err := f.db.QueryRow(`SELECT count(*),COALESCE(sum(CASE WHEN kind='supply' THEN amount_cents ELSE -amount_cents END),0) FROM cash_adjustments`).Scan(&count, &cash); err != nil || count != 3 || cash != 1599 {
		t.Fatal("cash or credit duplicated", count, cash, err)
	}
	if err := f.db.QueryRow(`SELECT count(*) FROM sales WHERE status='committed'`).Scan(&count); err != nil || count != 2 {
		t.Fatal("sale origin lost", count, err)
	}
	assertStockBalance(t, f, f.shelf, 2000)
}
