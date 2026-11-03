package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb/purchases"
)

const purchaseCancelOperationPath = "/local/v1/purchase-cancellation-operations/cancel-order/receipt"

func TestHTTPPurchaseCancelOperationOriginalRestoreAndNoWrites(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	b := requireMaterials(t, f, "GET", purchaseCancelOperationPath, nil, 200)
	var original purchases.CancelReceipt
	if err := json.Unmarshal(b, &original); err != nil {
		t.Fatal(err)
	}
	if original.Result.Repeated || original.Result.Status != "cancelled" || original.Input.OrderID != "order" || original.Result.Cancellation.ActorID != f.owner.OwnerID {
		t.Fatal(original)
	}
	before := purchaseTraceResponse(t, f)
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	if got := requireMaterials(t, f, "GET", purchaseCancelOperationPath, nil, 200); string(got) != string(b) {
		t.Fatal("original receipt changed")
	}
	if after := purchaseTraceResponse(t, f); !reflect.DeepEqual(before, after) {
		t.Fatal(after)
	}
	for _, table := range []string{"stock_movements", "cash_movements", "sale_payments"} {
		var n int
		if err := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatal(table, n, err)
		}
	}
	db := restoredPurchaseDB(t, f)
	got, err := purchases.CancelOperation(context.Background(), db, materialsActor(f), f.device, "cancel-order")
	if err != nil || !reflect.DeepEqual(got, original) {
		t.Fatal(got, err)
	}
	for _, q := range []string{"?offset=0", "?tenant_id=other", "?x=1&x=2"} {
		requireMaterials(t, f, "GET", purchaseCancelOperationPath+q, nil, 400)
	}
	requireMaterials(t, f, "GET", "/local/v1/purchase-cancellation-operations/missing/receipt", nil, 404)
}
func TestHTTPPurchaseCancelOperationScopeCorruptionAndRoles(t *testing.T) {
	for _, field := range []string{"tenant_id", "store_id", "device_id", "actor_id"} {
		t.Run(field, func(t *testing.T) {
			f, _ := purchaseTraceFixture(t)
			requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
			// Deliberately corrupt only this temporary fixture to exercise scoped lookup.
			if _, err := f.db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec("UPDATE purchase_order_cancellations SET " + field + "='foreign'"); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec("PRAGMA foreign_keys=ON"); err != nil {
				t.Fatal(err)
			}
			_, err := purchases.CancelOperation(context.Background(), f.db, materialsActor(f), f.device, "cancel-order")
			if !errors.Is(err, purchases.ErrNotFound) {
				t.Fatal(err)
			}
		})
	}
	for _, change := range []string{"request_json='{}'", "reason='tampered'", "created_at='invalid'", "request_json=request_json || ' '"} {
		t.Run(change, func(t *testing.T) {
			f, _ := purchaseTraceFixture(t)
			requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
			if _, err := f.db.Exec("UPDATE purchase_order_cancellations SET " + change); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "GET", purchaseCancelOperationPath, nil, 409)
		})
	}
	f, _ := purchaseTraceFixture(t)
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", purchaseCancelOperationPath, nil, 403)
}
