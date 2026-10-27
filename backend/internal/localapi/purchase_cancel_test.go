package localapi

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/purchases"
)

const purchaseCancelPath = "/local/v1/purchase-orders/order/cancel"

func purchaseCancelBody() map[string]any {
	return map[string]any{"operation_id": "cancel-order", "expected_status": "local_not_sent", "reason": "pedido desistido"}
}
func TestHTTPPurchaseCancelPreservesSnapshotApprovalAndReplay(t *testing.T) {
	f, s := purchaseTraceFixture(t)
	before := purchaseTraceResponse(t, f)
	in := purchaseCancelBody()
	requireMaterials(t, f, "POST", purchaseCancelPath, in, 200)
	requireMaterials(t, f, "POST", purchaseCancelPath, in, 200)
	after := purchaseTraceResponse(t, f)
	if after.Order.Status != "cancelled" || after.Cancellation == nil || after.Cancellation.ActorID != f.owner.OwnerID || after.Cancellation.Reason != "pedido desistido" {
		t.Fatal(after)
	}
	after.Order.Status = "local_not_sent"
	after.Cancellation = nil
	if !reflect.DeepEqual(before, after) {
		t.Fatal(before, after)
	}
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 200)
	in["operation_id"] = "another-cancel"
	requireMaterials(t, f, "POST", purchaseCancelPath, in, 409)
	in = purchaseCancelBody()
	in["reason"] = "different"
	requireMaterials(t, f, "POST", purchaseCancelPath, in, 409)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", purchases.Input{OperationID: "replacement", OrderID: "replacement", SupplierID: "supplier", SuggestionID: s}, 409)
	requireMaterials(t, f, "GET", "/local/v1/purchase-approvals", nil, 200)
	out, err := purchases.Get(context.Background(), f.db, materialsActor(f), f.device, "order")
	if err != nil || out.Status != "cancelled" {
		t.Fatal(out, err)
	}
	list, err := purchases.Orders(context.Background(), f.db, materialsActor(f), f.device, 0)
	if err != nil || len(list) != 1 || list[0].Status != "cancelled" {
		t.Fatal(list, err)
	}
	search, err := purchases.Search(context.Background(), f.db, materialsActor(f), f.device, purchases.SearchFilter{}, 0)
	if err != nil || len(search.Items) != 1 || search.Items[0].Status != "cancelled" {
		t.Fatal(search, err)
	}
	for _, table := range []string{"stock_movements", "cash_movements", "sale_payments"} {
		var n int
		if err = f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatal(table, n, err)
		}
	}
	var n int
	if err = f.db.QueryRow(`SELECT count(*) FROM purchase_order_cancellations`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}
func TestHTTPPurchaseCancelRollbackStrictInputsAndAuthorization(t *testing.T) {
	for _, table := range []string{"purchase_order_cancellations", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f, _ := purchaseTraceFixture(t)
			if _, err := f.db.Exec("CREATE TRIGGER reject_cancel BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 409)
			if out := purchaseTraceResponse(t, f); out.Order.Status != "local_not_sent" || out.Cancellation != nil {
				t.Fatal(out)
			}
			var n int
			if err := f.db.QueryRow(`SELECT count(*) FROM purchase_order_cancellations`).Scan(&n); err != nil || n != 0 {
				t.Fatal(n, err)
			}
		})
	}
	f, _ := purchaseTraceFixture(t)
	for _, field := range []string{"tenant_id", "store_id", "order_id", "status"} {
		in := purchaseCancelBody()
		in[field] = "other"
		requireMaterials(t, f, "POST", purchaseCancelPath, in, 400)
	}
	in := purchaseCancelBody()
	in["expected_status"] = "sent"
	requireMaterials(t, f, "POST", purchaseCancelPath, in, 400)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders/missing/cancel", purchaseCancelBody(), 404)
	requireMaterials(t, f, "POST", purchaseCancelPath+"?tenant_id=other", purchaseCancelBody(), 400)
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 403)
}
func TestHTTPPurchaseCancelConcurrentReplayAndBackup(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, _ := json.Marshal(purchaseCancelBody())
			status, body := request(t, f.app, "POST", purchaseCancelPath, string(b), f.token)
			if status != 200 {
				t.Errorf("%d %s", status, body)
			}
		}()
	}
	wg.Wait()
	before := purchaseTraceResponse(t, f)
	db := restoredPurchaseDB(t, f)
	after, err := purchases.Trace(context.Background(), db, materialsActor(f), f.device, "order")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal(after, err)
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM purchase_order_cancellations`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='purchase.order.cancelled'`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}
func TestHTTPPurchaseCancelTraceRejectsCorruptEvidence(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	if _, err := f.db.Exec(`UPDATE purchase_order_cancellations SET reason='corrupted'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", purchaseTracePath, nil, 409)
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 409)
}
