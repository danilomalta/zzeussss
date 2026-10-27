package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/catalog"
)

func TestHTTPCatalogOperationOriginalReceiptsAndRestore(t *testing.T) {
	f, _ := recipeFixture(t)
	in := catalogEditBody("g")
	requireMaterials(t, f, "POST", catalogEditPath, in, 200)
	in["operation_id"] = "edit-next"
	in["expected_revision"] = 1
	in["name"] = "Later name"
	requireMaterials(t, f, "POST", catalogEditPath, in, 200)
	state := map[string]any{"operation_id": "inactive", "expected_revision": 0, "status": "inactive", "reason": "Decisao humana"}
	requireMaterials(t, f, "POST", "/local/v1/catalog/products/flour/state", state, 200)
	state["operation_id"] = "active"
	state["expected_revision"] = 1
	state["status"] = "active"
	requireMaterials(t, f, "POST", "/local/v1/catalog/products/flour/state", state, 200)
	cases := []struct{ kind, op string }{{"edit", "edit-op"}, {"state", "inactive"}}
	before := map[string]string{}
	for _, c := range cases {
		b := requireMaterials(t, f, "GET", "/local/v1/catalog/operations/"+c.kind+"/"+c.op, nil, 200)
		var out catalog.OperationReceipt
		if json.Unmarshal(b, &out) != nil {
			t.Fatal(string(b))
		}
		before[c.op] = string(b)
	}
	b := requireMaterials(t, f, "GET", "/local/v1/catalog/products/flour/edit-details", nil, 200)
	var details catalog.EditingDetails
	if json.Unmarshal(b, &details) != nil || details.Revision != 2 || details.Product.Name != "Later name" || details.Product.CostCents != 321 {
		t.Fatal(string(b))
	}
	ctx := context.Background()
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "catalog.tytbak")
	path := filepath.Join(t.TempDir(), "restored.sqlite")
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(ctx, archive, path, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, c := range cases {
		out, e := catalog.CatalogOperation(ctx, db, materialsActor(f), f.device, c.kind, c.op)
		if e != nil {
			t.Fatal(e)
		}
		data, _ := json.Marshal(out)
		if string(data) != before[c.op] {
			t.Fatal(string(data), before[c.op])
		}
	}
	if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
		t.Fatal(e)
	}
	requireMaterials(t, f, "GET", "/local/v1/catalog/products/flour/edit-details", nil, 403)
	requireMaterials(t, f, "GET", "/local/v1/catalog/operations/edit/edit-op", nil, 403)
}
func TestHTTPCatalogOperationRefusesForeignOperatorAndBadPayload(t *testing.T) {
	for _, field := range []string{"actor_id", "request_json"} {
		t.Run(field, func(t *testing.T) {
			f, _ := recipeFixture(t)
			requireMaterials(t, f, "POST", catalogEditPath, catalogEditBody("g"), 200)
			if _, e := f.db.Exec(`PRAGMA foreign_keys=OFF`); e != nil {
				t.Fatal(e)
			}
			value := "foreign"
			want := 404
			if field == "request_json" {
				value = `{"operation_id":"wrong"}`
				want = 409
			}
			if _, e := f.db.Exec("UPDATE catalog_edit_events SET "+field+"=?", value); e != nil {
				t.Fatal(e)
			}
			requireMaterials(t, f, "GET", "/local/v1/catalog/operations/edit/edit-op", nil, want)
		})
	}
}
