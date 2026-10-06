package localapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/catalog"
)

func seedSearchProduct(t *testing.T, f *httpContractFixture, id, name, barcode, unit string, cost int64) {
	t.Helper()
	_, err := f.db.Exec(`INSERT INTO products(tenant_id,id,sku,name,barcode,unit,price_cents,cost_cents) VALUES(?,?,?,?,?,?,1000,?)`, f.owner.TenantID, id, id, name, func() any {
		if barcode == "" {
			return nil
		}
		return barcode
	}(), unit, cost)
	if err != nil {
		t.Fatal(err)
	}
}
func readSearch(t *testing.T, f *httpContractFixture, query string) catalog.SearchResult {
	t.Helper()
	status, body := request(t, f.app, "GET", "/local/v1/catalog/search?"+query, "", f.token)
	if status != 200 {
		t.Fatalf("search %s: %d %s", query, status, body)
	}
	var result catalog.SearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestHTTPCatalogSearchAcrossPagesAndAccentApproximation(t *testing.T) {
	f := httpContractSetup(t)
	for i := 0; i < 55; i++ {
		seedSearchProduct(t, f, fmt.Sprintf("P%02d", i), "Produto comum", "", "unit", 0)
	}
	seedSearchProduct(t, f, "ZZ", "Café especial", "7890123456789", "kg", 300)
	r := readSearch(t, f, "q=cafe")
	if r.Total != 1 || r.Items[0].ID != "ZZ" || r.Items[0].Approximate {
		t.Fatalf("accent %+v", r)
	}
	r = readSearch(t, f, "q=cafee")
	if r.Total != 1 || !r.Items[0].Approximate {
		t.Fatalf("approximation %+v", r)
	}
	r = readSearch(t, f, "q="+url.QueryEscape("cafe espec"))
	if r.Total != 1 {
		t.Fatal("multiple terms")
	}
	r = readSearch(t, f, "q=7890123456788")
	if r.Total != 0 {
		t.Fatal("numeric code autocorrected")
	}
	r = readSearch(t, f, "")
	if r.Total != 56 || len(r.Items) != 50 || !r.CostVisible {
		t.Fatalf("first page %+v", r)
	}
	r = readSearch(t, f, "offset=50")
	if len(r.Items) != 6 || r.Items[5].ID != "ZZ" {
		t.Fatalf("second page %+v", r)
	}
	r = readSearch(t, f, "q=zz&unit=kg")
	if r.Total != 1 {
		t.Fatal("sku/unit")
	}
	r = readSearch(t, f, "q=inexistente")
	if r.Items == nil || r.Total != 0 {
		t.Fatal("empty array")
	}
}
func TestHTTPCatalogSearchFiltersUseActualStorePoliciesAndRestrictCost(t *testing.T) {
	f := httpContractSetup(t)
	seedSearchProduct(t, f, "A", "Com política", "", "unit", 0)
	seedSearchProduct(t, f, "B", "Com código", "12345", "kg", 200)
	_, err := f.db.Exec(`INSERT INTO restock_policies(tenant_id,store_id,product_id,minimum_milli,target_milli,revision,changed_by,changed_at) VALUES(?,?,?,1000,2000,1,?,'now')`, f.owner.TenantID, f.owner.StoreID, "A", f.owner.OwnerID)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct{ query, id string }{{"pending=barcode", "A"}, {"pending=cost", "A"}, {"pending=minimum", "B"}} {
		r := readSearch(t, f, x.query)
		if r.Total != 1 || r.Items[0].ID != x.id {
			t.Fatalf("filter %s: %+v", x.query, r)
		}
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	status, body := request(t, f.app, "GET", "/local/v1/catalog/search", "", f.token)
	if status != 200 || bytes.Contains(body, []byte("cost_cents")) || bytes.Contains(body, []byte(`"cost_visible":true`)) {
		t.Fatalf("cost leaked %d %s", status, body)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/catalog/search?pending=cost", "", f.token)
	if status != 403 {
		t.Fatalf("cost filter: %d", status)
	}
}
func TestHTTPCatalogSearchRejectsAmbiguityAndRequiresCurrentSession(t *testing.T) {
	f := httpContractSetup(t)
	for _, q := range []string{"q=a&q=b", "tenant_id=other", "unit=box", "pending=ncm", "offset=-1", "offset=1.5", "offset=1000000001", "q=" + strings.Repeat("x", 241), "q=a+b+c+d+e+f+g+h+i"} {
		status, _ := request(t, f.app, "GET", "/local/v1/catalog/search?"+q, "", f.token)
		if status != 400 {
			t.Fatalf("%s: %d", q, status)
		}
	}
	status, _ := request(t, f.app, "GET", "/local/v1/catalog/search", "", "")
	if status != 401 {
		t.Fatal("anonymous")
	}
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, f.owner.TenantID, f.owner.DeviceID); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/catalog/search", "", f.token)
	if status != 401 {
		t.Fatalf("revoked %d", status)
	}
}
func TestHTTPCatalogSearchCannotReadForeignCompany(t *testing.T) {
	f := httpContractSetup(t)
	if _, err := f.db.Exec(`INSERT INTO tenants VALUES ('other','Outra','now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO products(tenant_id,id,sku,name,unit,price_cents,cost_cents) VALUES('other','foreign','foreign','Segredo','unit',999,999)`); err != nil {
		t.Fatal(err)
	}
	r := readSearch(t, f, "q=segredo")
	if r.Total != 0 {
		t.Fatal("foreign product visible")
	}
}
