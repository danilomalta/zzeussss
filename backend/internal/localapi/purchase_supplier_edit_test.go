package localapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/purchases"
)

const supplierUpdatePath = "/local/v1/purchase-suppliers/supplier/update"

func supplierEditBody() map[string]any {
	return map[string]any{"operation_id": "supplier-edit", "expected_revision": 0, "name": "Padaria nova", "status": "inactive", "reason": "cadastro revisado"}
}
func supplierCreationReplay(t *testing.T, f *httpContractFixture) {
	t.Helper()
	requireMaterials(t, f, "POST", "/local/v1/purchase-suppliers", json.RawMessage(`{"operation_id":"supplier-op","id":"supplier","name":"Padaria","status":"active"}`), 200)
}
func supplierState(t *testing.T, f *httpContractFixture) purchases.SupplierState {
	t.Helper()
	out, err := purchases.SupplierEditState(context.Background(), f.db, materialsActor(f), f.device, "supplier")
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestHTTPPurchaseSupplierEditSnapshotsReplayAndState(t *testing.T) {
	f, s := purchaseFixture(t)
	in := supplierEditBody()
	requireMaterials(t, f, "POST", supplierUpdatePath, in, 200)
	supplierCreationReplay(t, f)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 409)
	in["expected_revision"] = 1
	in["operation_id"] = "supplier-active"
	in["status"] = "active"
	requireMaterials(t, f, "POST", supplierUpdatePath, in, 200)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	before := purchaseTraceResponse(t, f)
	if before.Order.SupplierName != "Padaria nova" {
		t.Fatal(before)
	}
	in["expected_revision"] = 2
	in["operation_id"] = "supplier-off"
	in["name"] = "Nome atual"
	in["status"] = "inactive"
	requireMaterials(t, f, "POST", supplierUpdatePath, in, 200)
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200) // original edit replay, never overwrites current state
	if state := supplierState(t, f); state.Revision != 3 || state.Name != "Nome atual" || state.Status != "inactive" {
		t.Fatal(state)
	}
	if after := purchaseTraceResponse(t, f); !reflect.DeepEqual(before, after) {
		t.Fatal(after)
	}
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 200)
	supplierCreationReplay(t, f)
	in["operation_id"] = "stale"
	in["expected_revision"] = 0
	requireMaterials(t, f, "POST", supplierUpdatePath, in, 409)
	in = supplierEditBody()
	in["name"] = "different"
	requireMaterials(t, f, "POST", supplierUpdatePath, in, 409)
	requireMaterials(t, f, "GET", "/local/v1/purchase-suppliers/supplier/edit-state", nil, 200)
}
func TestHTTPPurchaseSupplierEditRollbackAndStrictScope(t *testing.T) {
	for _, table := range []string{"purchase_supplier_revisions", "purchase_supplier_edits", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f, _ := purchaseFixture(t)
			if _, err := f.db.Exec("CREATE TRIGGER reject_supplier_edit BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 409)
			if state := supplierState(t, f); state.Revision != 0 || state.Name != "Padaria" || state.Status != "active" {
				t.Fatal(state)
			}
			var n int
			if err := f.db.QueryRow(`SELECT count(*) FROM purchase_supplier_edits`).Scan(&n); err != nil || n != 0 {
				t.Fatal(n, err)
			}
		})
	}
	f, _ := purchaseFixture(t)
	for _, field := range []string{"tenant_id", "store_id", "supplier_id", "unknown"} {
		in := supplierEditBody()
		in[field] = "foreign"
		requireMaterials(t, f, "POST", supplierUpdatePath, in, 400)
	}
	for _, rev := range []any{-1, 1.5, 2147483647} {
		in := supplierEditBody()
		in["expected_revision"] = rev
		requireMaterials(t, f, "POST", supplierUpdatePath, in, 400)
	}
	in := supplierEditBody()
	in["status"] = "deleted"
	requireMaterials(t, f, "POST", supplierUpdatePath, in, 400)
	in = supplierEditBody()
	in["operation_id"] = "supplier-op"
	requireMaterials(t, f, "POST", supplierUpdatePath, in, 409)
	requireMaterials(t, f, "GET", "/local/v1/purchase-suppliers/supplier/edit-state?tenant_id=other", nil, 400)
	requireMaterials(t, f, "POST", "/local/v1/purchase-suppliers/missing/update", supplierEditBody(), 404)
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
		if _, err := purchases.SupplierEditState(context.Background(), f.db, a, d, "supplier"); err == nil {
			t.Fatal(kind)
		}
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 403)
	requireMaterials(t, f, "GET", "/local/v1/purchase-suppliers/supplier/edit-state", nil, 403)
}
func restoredPurchaseDB(t *testing.T, f *httpContractFixture) *sql.DB {
	t.Helper()
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "purchase.tytbak")
	path := filepath.Join(dir, "restored.sqlite")
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err := backup.Verify(ctx, archive, f.device, key); err != nil {
		t.Fatal(err)
	}
	if err := backup.Restore(ctx, archive, path, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
func TestHTTPPurchaseSupplierEditBackupRestoresRevisionAndReplay(t *testing.T) {
	f, _ := purchaseFixture(t)
	requireMaterials(t, f, "POST", supplierUpdatePath, supplierEditBody(), 200)
	db := restoredPurchaseDB(t, f)
	v, err := purchases.SupplierEditState(context.Background(), db, materialsActor(f), f.device, "supplier")
	if err != nil || v.Revision != 1 || v.Status != "inactive" || v.Name != "Padaria nova" {
		t.Fatal(v, err)
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM purchase_supplier_edits`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}

	var name string
	if err = db.QueryRow(`SELECT name FROM purchase_supplier_originals`).Scan(&name); err != nil || name != "Padaria" {
		t.Fatal(name, err)
	}
}
