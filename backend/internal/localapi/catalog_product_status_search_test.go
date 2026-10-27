package localapi

import (
	"encoding/json"
	"fmt"
	"testing"
	"titansystem-backend/internal/localdb/catalog"
)

func catalogStatusPage(t *testing.T, f *httpContractFixture, query string) catalog.SearchResult {
	t.Helper()
	b := requireMaterials(t, f, "GET", "/local/v1/catalog/search"+query, nil, 200)
	var out catalog.SearchResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestHTTPProductStatusSearchFiltersBeforePaginationAndKeepsCostsPrivate(t *testing.T) {
	f, _ := recipeFixture(t)
	setProductStatus(t, f, "flour", "inactive", 0, "inactive")
	active := catalogStatusPage(t, f, "?status=active")
	if active.Total != 2 || active.Status != "active" {
		t.Fatal(active)
	}
	for _, v := range active.Items {
		if v.Status != "active" || v.StateRevision != 0 || v.ID == "flour" {
			t.Fatal(v)
		}
	}
	off := catalogStatusPage(t, f, "?status=inactive&q=flour&unit=g&pending=barcode")
	if off.Total != 1 || len(off.Items) != 1 || off.Items[0].StateRevision != 1 || off.Items[0].Status != "inactive" {
		t.Fatal(off)
	}
	all := catalogStatusPage(t, f, "")
	if all.Total != 3 || all.Status != "all" {
		t.Fatal(all)
	}
	b := requireMaterials(t, f, "GET", "/local/v1/products", nil, 200)
	var list struct {
		Items []catalog.Product `json:"items"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range list.Items {
		if v.ID == "flour" {
			found = true
			if v.Status != "inactive" || v.StateRevision != 1 {
				t.Fatal(v)
			}
		}
	}
	if !found {
		t.Fatal(list)
	}
	for i := 0; i < 52; i++ {
		id := fmt.Sprintf("bulk-%02d", i)
		if _, err := f.db.Exec(`INSERT INTO products(tenant_id,id,sku,name,unit,price_cents,cost_cents) VALUES(?,?,?,'Bulk','unit',100,37)`, f.owner.TenantID, id, id); err != nil {
			t.Fatal(err)
		}
		setProductStatus(t, f, id, "state-"+id, 0, "inactive")
	}
	first := catalogStatusPage(t, f, "?status=inactive&q=Bulk")
	if first.Total != 52 || len(first.Items) != 50 {
		t.Fatal(first)
	}
	next := catalogStatusPage(t, f, "?status=inactive&q=Bulk&offset=50")
	if next.Total != 52 || len(next.Items) != 2 || next.Items[0].ID != "bulk-50" {
		t.Fatal(next)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	page := catalogStatusPage(t, f, "?status=inactive&q=Bulk")
	if page.CostVisible {
		t.Fatal(page)
	}
	for _, v := range page.Items {
		if v.CostCents != nil {
			t.Fatal(v)
		}
	}
}
func TestHTTPProductStatusSearchStrictParametersAndGlobalTenantScope(t *testing.T) {
	f, _ := recipeFixture(t)
	setProductStatus(t, f, "flour", "inactive", 0, "inactive")
	for _, q := range []string{"?status=bad", "?status=", "?status=active&status=inactive", "?status=active&tenant_id=other", "?store_id=other"} {
		requireMaterials(t, f, "GET", "/local/v1/catalog/search"+q, nil, 400)
	}
	if _, err := f.db.Exec(`INSERT INTO tenants VALUES('foreign','Foreign','now');INSERT INTO products(tenant_id,id,sku,name,unit,price_cents,cost_cents) VALUES('foreign','flour','Foreign','Foreign','kg',100,37);INSERT INTO catalog_product_states VALUES('foreign','flour','inactive',1)`); err != nil {
		t.Fatal(err)
	}
	page := catalogStatusPage(t, f, "?status=inactive")
	if page.Total != 1 || page.Items[0].Name != "flour" || page.Items[0].Unit != "g" {
		t.Fatal(page)
	}
}
