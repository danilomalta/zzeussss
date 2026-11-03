package localapi

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/purchases"
)

const receivingAuthPath = "/local/v1/purchase-orders/order/authorize-receiving"

func receivingAuthBody() map[string]any {
	return map[string]any{"operation_id": "receiving-auth", "reference": "document-1", "reason": "conferencia autorizada pelo comprador"}
}
func TestHTTPPurchaseReceivingAuthorizationReplayCancellationAndBackup(t *testing.T) {
	f, s := purchaseTraceFixture(t)
	before := purchaseTraceResponse(t, f)
	requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 200)
	requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 200)
	after := purchaseTraceResponse(t, f)
	if after.Order.Status != "local_not_sent" || after.Order.ReceivingStatus != "authorized" || after.ReceivingAuthorization == nil || after.ReceivingAuthorization.ActorID != f.owner.OwnerID {
		t.Fatal(after)
	}
	if !reflect.DeepEqual(before.Order.Items, after.Order.Items) || !reflect.DeepEqual(before.Approval, after.Approval) {
		t.Fatal("snapshot changed")
	}
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 409)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 200)
	changed := receivingAuthBody()
	changed["reason"] = "changed"
	requireMaterials(t, f, "POST", receivingAuthPath, changed, 409)
	changed = receivingAuthBody()
	changed["operation_id"] = "new"
	requireMaterials(t, f, "POST", receivingAuthPath, changed, 409)
	for _, table := range []string{"stock_movements", "stock_operations", "cash_movements"} {
		materialCount(t, f, table, 0)
	}
	db := restoredPurchaseDB(t, f)
	got, err := purchases.Trace(context.Background(), db, materialsActor(f), f.device, "order")
	if err != nil || !reflect.DeepEqual(after, got) {
		t.Fatal(got, err)
	}
}
func TestHTTPPurchaseReceivingAuthorizationRollbackInputScopeAndRole(t *testing.T) {
	for _, table := range []string{"purchase_receiving_authorizations", "purchase_receiving_events", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f, _ := purchaseTraceFixture(t)
			if _, err := f.db.Exec("CREATE TRIGGER reject_authorization BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 409)
			materialCount(t, f, "purchase_receiving_authorizations", 0)
			if purchaseTraceResponse(t, f).Order.ReceivingStatus != "not_authorized" {
				t.Fatal("partial decision")
			}
		})
	}
	f, _ := purchaseTraceFixture(t)
	for _, field := range []string{"tenant_id", "store_id", "order_id", "supplier_confirmation"} {
		in := receivingAuthBody()
		in[field] = "x"
		requireMaterials(t, f, "POST", receivingAuthPath, in, 400)
	}
	for _, field := range []string{"operation_id", "reference", "reason"} {
		in := receivingAuthBody()
		in[field] = "bad\nvalue"
		requireMaterials(t, f, "POST", receivingAuthPath, in, 400)
	}
	requireMaterials(t, f, "POST", receivingAuthPath+"?tenant_id=other", receivingAuthBody(), 400)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders/missing/authorize-receiving", receivingAuthBody(), 404)
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 409)
	other, _ := purchaseTraceFixture(t)
	if _, err := other.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, other.owner.TenantID, other.owner.OwnerID, other.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := other.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, other, "POST", receivingAuthPath, receivingAuthBody(), 403)
	if status, _ := request(t, other.app, "POST", receivingAuthPath, orderBody(t, receivingAuthBody()), ""); status != 401 {
		t.Fatal(status)
	}
}
func TestHTTPPurchaseReceivingAuthorizationConcurrentDecision(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, body := request(t, f.app, "POST", receivingAuthPath, orderBody(t, receivingAuthBody()), f.token)
			if status != 200 {
				t.Errorf("%d %s", status, body)
			}
		}()
	}
	wg.Wait()
	materialCount(t, f, "purchase_receiving_authorizations", 1)
	if _, err := f.db.Exec(`UPDATE purchase_receiving_authorizations SET reference='broken'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", purchaseTracePath, nil, 409)
	requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 409)
}
