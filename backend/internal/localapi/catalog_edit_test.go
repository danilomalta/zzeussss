package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/catalog"
)

func catalogEditBody(unit string) map[string]any {
	return map[string]any{"operation_id": "edit-op", "expected_revision": 0, "sku": "Edited", "name": "Edited name", "unit": unit, "barcode": "", "price_cents": 1234, "cost_cents": 321}
}

const catalogEditPath = "/local/v1/catalog/products/flour/update"

func TestHTTPCatalogEditRevisionReplayAndConflicts(t *testing.T) {
	f, _ := recipeFixture(t)
	in := catalogEditBody("g")
	for i := 0; i < 2; i++ {
		requireMaterials(t, f, "POST", catalogEditPath, in, 200)
	}
	state, err := catalog.EditState(context.Background(), f.db, materialsActor(f), f.device, "flour")
	if err != nil || state.Revision != 1 {
		t.Fatal(state, err)
	}
	var n int
	if err = f.db.QueryRow(`SELECT count(*) FROM catalog_edit_events`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	in["name"] = "different"
	requireMaterials(t, f, "POST", catalogEditPath, in, 409)
	in["operation_id"] = "stale"
	requireMaterials(t, f, "POST", catalogEditPath, in, 409)
	in["expected_revision"] = 1
	requireMaterials(t, f, "POST", catalogEditPath, in, 200)
	in["operation_id"] = "collision"
	in["expected_revision"] = 2
	in["sku"] = "oil"
	requireMaterials(t, f, "POST", catalogEditPath, in, 409)
	requireMaterials(t, f, "GET", "/local/v1/catalog/products/flour/edit-state", nil, 200)
}
func TestHTTPCatalogEditAuditRollbackAndReadAuthorization(t *testing.T) {
	f, _ := recipeFixture(t)
	if _, err := f.db.Exec(`CREATE TRIGGER reject_edit BEFORE INSERT ON catalog_edit_events BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", catalogEditPath, catalogEditBody("g"), 409)
	var name string
	if err := f.db.QueryRow(`SELECT name FROM products WHERE id='flour'`).Scan(&name); err != nil || name != "flour" {
		t.Fatal(name, err)
	}
	state, err := catalog.EditState(context.Background(), f.db, materialsActor(f), f.device, "flour")
	if err != nil || state.Revision != 0 {
		t.Fatal(state, err)
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", catalogEditPath, catalogEditBody("g"), 403)
	requireMaterials(t, f, "GET", "/local/v1/catalog/products/flour/edit-state", nil, 403)
}
func TestHTTPCatalogEditKeepsPurchasedItemSnapshot(t *testing.T) {
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	before := purchaseTraceResponse(t, f)
	id := before.Order.Items[0].ProductID
	requireMaterials(t, f, "POST", "/local/v1/catalog/products/"+id+"/update", catalogEditBody(before.Order.Items[0].Unit), 200)
	if after := purchaseTraceResponse(t, f); !reflect.DeepEqual(before, after) {
		t.Fatal(after)
	}
}
func TestHTTPCatalogEditBackupRestoresMetadataRevisionAudit(t *testing.T) {
	f, _ := recipeFixture(t)
	requireMaterials(t, f, "POST", catalogEditPath, catalogEditBody("g"), 200)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "catalog.tytbak")
	restored := filepath.Join(dir, "catalog.sqlite")
	ctx := context.Background()
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err := backup.Verify(ctx, archive, f.device, key); err != nil {
		t.Fatal(err)
	}
	if err := backup.Restore(ctx, archive, restored, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	state, err := catalog.EditState(ctx, db, materialsActor(f), f.device, "flour")
	if err != nil || state.Revision != 1 {
		t.Fatal(state, err)
	}
	var price, n int64
	if err = db.QueryRow(`SELECT price_cents FROM products WHERE id='flour'`).Scan(&price); err != nil || price != 1234 {
		t.Fatal(price, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM catalog_edit_events`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}
func TestHTTPCatalogEditStrictInputsAndScope(t *testing.T) {
	f, _ := recipeFixture(t)
	for _, field := range []string{"tenant_id", "product_id", "unknown"} {
		in := catalogEditBody("g")
		in[field] = "other"
		requireMaterials(t, f, "POST", catalogEditPath, in, 400)
	}
	in := catalogEditBody("g")
	in["price_cents"] = 1.5
	requireMaterials(t, f, "POST", catalogEditPath, in, 400)
	in["price_cents"] = -1
	requireMaterials(t, f, "POST", catalogEditPath, in, 400)
	a := materialsActor(f)
	a.TenantID = "foreign"
	if _, err := catalog.EditState(context.Background(), f.db, a, f.device, "flour"); err == nil {
		t.Fatal("cross tenant")
	}
	requireMaterials(t, f, "POST", "/local/v1/catalog/products/missing/update", catalogEditBody("g"), 404)
}
func TestHTTPCatalogEditKeepsUnitOfProductUsedByRecipe(t *testing.T) {
	f, _ := orderFixture(t)
	requireMaterials(t, f, "POST", catalogEditPath, catalogEditBody("kg"), 409)
	var unit string
	if err := f.db.QueryRow(`SELECT unit FROM products WHERE id='flour'`).Scan(&unit); err != nil || unit != "g" {
		t.Fatal(unit, err)
	}
}
