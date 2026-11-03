package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb/purchases"
)

const supplierOperationPath = "/local/v1/purchase-supplier-operations/supplier-edit/receipt"

func TestHTTPPurchaseSupplierOperationOriginalRestoreAndNoWrites(t *testing.T) {
	f, _ := purchaseFixture(t)
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
	b := requireMaterials(t, f, "GET", supplierOperationPath, nil, 200)
	var original purchases.SupplierEditReceipt
	if err := json.Unmarshal(b, &original); err != nil {
		t.Fatal(err)
	}
	if original.Result.Repeated || original.Result.Revision != 1 || original.BeforeName != "Padaria" || original.Result.Name != "Padaria nova" || original.Result.Status != "inactive" {
		t.Fatal(original)
	}
	in := supplierEditBody()
	in["operation_id"] = "later"
	in["expected_revision"] = 1
	in["name"] = "Outro nome"
	in["status"] = "active"
	requireMaterials(t, f, "POST", supplierUpdatePath, in, 200)
	before := supplierHistoryResponse(t, f, 0)
	if got := requireMaterials(t, f, "GET", supplierOperationPath, nil, 200); string(got) != string(b) {
		t.Fatal("receipt changed")
	}
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
	if after := supplierHistoryResponse(t, f, 0); !reflect.DeepEqual(before, after) {
		t.Fatal("read or replay changed state")
	}
	db := restoredPurchaseDB(t, f)
	restored, err := purchases.SupplierEditOperation(context.Background(), db, materialsActor(f), f.device, "supplier-edit")
	if err != nil || !reflect.DeepEqual(restored, original) {
		t.Fatal(restored, err)
	}
	for _, q := range []string{"?offset=0", "?tenant_id=other", "?x=1&x=2"} {
		requireMaterials(t, f, "GET", supplierOperationPath+q, nil, 400)
	}
	requireMaterials(t, f, "GET", "/local/v1/purchase-supplier-operations/missing/receipt", nil, 404)
}
func TestHTTPPurchaseSupplierOperationScopeCorruptionAndRoles(t *testing.T) {
	for _, field := range []string{"tenant_id", "store_id", "device_id", "actor_id"} {
		t.Run(field, func(t *testing.T) {
			f, _ := purchaseFixture(t)
			requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
			// Deliberately corrupt only this temporary test database to exercise scoped lookup.
			if _, err := f.db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec("UPDATE purchase_supplier_edits SET " + field + "='foreign'"); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec("PRAGMA foreign_keys=ON"); err != nil {
				t.Fatal(err)
			}
			_, err := purchases.SupplierEditOperation(context.Background(), f.db, materialsActor(f), f.device, "supplier-edit")
			if !errors.Is(err, purchases.ErrNotFound) {
				t.Fatal(err)
			}
		})
	}
	for _, change := range []string{"request_json='{}'", "before_name='tampered'", "before_status='inactive'", "created_at='invalid'"} {
		t.Run(change, func(t *testing.T) {
			f, _ := purchaseFixture(t)
			requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
			if _, err := f.db.Exec("UPDATE purchase_supplier_edits SET " + change); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "GET", supplierOperationPath, nil, 409)
		})
	}
	f, _ := purchaseFixture(t)
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", supplierOperationPath, nil, 403)
}
