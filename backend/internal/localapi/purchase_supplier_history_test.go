package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/purchases"
)

const supplierHistoryPath = "/local/v1/purchase-suppliers/supplier/history"

func supplierHistoryResponse(t *testing.T, f *httpContractFixture, offset int) purchases.SupplierHistoryPage {
	t.Helper()
	b := requireMaterials(t, f, "GET", fmt.Sprintf("%s?offset=%d", supplierHistoryPath, offset), nil, 200)
	var v purchases.SupplierHistoryPage
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestHTTPPurchaseSupplierHistoryPaginationAndBackup(t *testing.T) {
	f, _ := purchaseFixture(t)
	original := supplierHistoryResponse(t, f, 0)
	if original.Current.Revision != 0 || len(original.Items) != 0 || original.Origin.Name != "Padaria" || original.Origin.Creation.Kind != "supplier.created" || original.Origin.Creation.ActorID != f.owner.OwnerID {
		t.Fatal(original)
	}
	for i := 0; i < 51; i++ {
		in := supplierEditBody()
		in["expected_revision"] = i
		in["operation_id"] = fmt.Sprintf("edit-%d", i)
		in["name"] = fmt.Sprintf("Fornecedor %d", i)
		requireMaterials(t, f, "POST", supplierUpdatePath, in, 200)
	}
	page := supplierHistoryResponse(t, f, 0)
	if page.TotalCount != 51 || len(page.Items) != 50 || !page.HasMore || page.Current.Revision != 51 || page.Items[0].BeforeName != "Padaria" || page.Items[49].Revision != 50 {
		t.Fatal(page)
	}
	next := supplierHistoryResponse(t, f, 50)
	if len(next.Items) != 1 || next.HasMore || next.Items[0].Revision != 51 || next.Items[0].BeforeName != "Fornecedor 49" || next.Items[0].Name != "Fornecedor 50" {
		t.Fatal(next)
	}
	empty := supplierHistoryResponse(t, f, 51)
	if len(empty.Items) != 0 || empty.HasMore || empty.TotalCount != 51 {
		t.Fatal(empty)
	}
	db := restoredPurchaseDB(t, f)
	got, err := purchases.SupplierHistory(context.Background(), db, materialsActor(f), f.device, "supplier", 50)
	if err != nil || !reflect.DeepEqual(next, got) {
		t.Fatal(got, err)
	}
}
func TestHTTPPurchaseSupplierHistoryStrictScopeAndCorruption(t *testing.T) {
	f, _ := purchaseFixture(t)
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
	for _, q := range []string{"?offset=-1", "?offset=1.5", "?offset=", "?offset=0&offset=1", "?offset=9007199254740992", "?tenant_id=other", "?store_id=other"} {
		requireMaterials(t, f, "GET", supplierHistoryPath+q, nil, 400)
	}
	requireMaterials(t, f, "GET", "/local/v1/purchase-suppliers/missing/history", nil, 404)
	for _, kind := range []string{"tenant", "store", "device", "actor"} {
		a := materialsActor(f)
		d := f.device
		switch kind {
		case "tenant":
			a.TenantID = "other"
		case "store":
			a.StoreID = "other"
		case "device":
			d.DeviceID = "other"
		case "actor":
			a.IdentityID = "other"
		}
		if _, err := purchases.SupplierHistory(context.Background(), f.db, a, d, "supplier", 0); err == nil {
			t.Fatal(kind)
		}
	}
	for _, q := range []string{`UPDATE purchase_supplier_edits SET before_name='different'`, `UPDATE purchase_supplier_edits SET request_json='{}'`, `DELETE FROM purchase_supplier_edits`, `UPDATE purchase_suppliers SET name='manual'`, `DELETE FROM purchase_audit WHERE kind='supplier.created'`} {
		t.Run(q, func(t *testing.T) {
			f, _ := purchaseFixture(t)
			requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
			if _, err := f.db.Exec(q); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "GET", supplierHistoryPath, nil, 409)
		})
	}
}
func TestHTTPPurchaseSupplierHistoryReadWhileEditAndCombinedLifecycle(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	before := purchaseTraceResponse(t, f)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		body, _ := json.Marshal(supplierEditBody())
		status, b := request(t, f.app, "POST", supplierUpdatePath, string(body), f.token)
		if status != 200 {
			t.Errorf("edit %d %s", status, b)
		}
	}()
	for i := 0; i < 8; i++ {
		v := supplierHistoryResponse(t, f, 0)
		if v.TotalCount != v.Current.Revision || int64(len(v.Items)) != v.Current.Revision {
			t.Fatal(v)
		}
	}
	wg.Wait()
	supplierCreationReplay(t, f)
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	cancelled := purchaseTraceResponse(t, f)
	if !reflect.DeepEqual(before.Order.Items, cancelled.Order.Items) || cancelled.Order.SupplierName != "Padaria" || cancelled.Order.Status != "cancelled" {
		t.Fatal(cancelled)
	}
	history := supplierHistoryResponse(t, f, 0)
	db := restoredPurchaseDB(t, f)
	got, err := purchases.SupplierHistory(context.Background(), db, materialsActor(f), f.device, "supplier", 0)
	if err != nil || !reflect.DeepEqual(history, got) {
		t.Fatal(got, err)
	}
	trace, err := purchases.Trace(context.Background(), db, materialsActor(f), f.device, "order")
	if err != nil || !reflect.DeepEqual(cancelled, trace) {
		t.Fatal(trace, err)
	}
}
func TestHTTPPurchaseSupplierHistoryExpiryKeepsReadsBlocksEditsAndCancel(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
	before := supplierHistoryResponse(t, f, 0)
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Orders}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 403)
	requireMaterials(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 403)
	if got := supplierHistoryResponse(t, f, 0); !reflect.DeepEqual(before, got) {
		t.Fatal(got)
	}
	purchaseTraceResponse(t, f)
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", supplierHistoryPath, nil, 403)
}
