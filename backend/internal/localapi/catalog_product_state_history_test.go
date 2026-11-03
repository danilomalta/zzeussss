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
	"titansystem-backend/internal/localdb/catalog"
)

func productHistoryPath(id string) string {
	return "/local/v1/catalog/products/" + id + "/state-history"
}
func productHistoryResponse(t *testing.T, f *httpContractFixture, id string, offset int) catalog.ProductStateHistory {
	t.Helper()
	b := requireMaterials(t, f, "GET", fmt.Sprintf("%s?offset=%d", productHistoryPath(id), offset), nil, 200)
	var out catalog.ProductStateHistory
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestHTTPProductStateHistoryPaginationGlobalStoresAndBackup(t *testing.T) {
	f, _ := recipeFixture(t)
	initial := productHistoryResponse(t, f, "flour", 0)
	if initial.Current.Status != "active" || initial.TotalCount != 0 || len(initial.Items) != 0 {
		t.Fatal(initial)
	}
	for i := 0; i < 51; i++ {
		status := "inactive"
		if i%2 != 0 {
			status = "active"
		}
		setProductStatus(t, f, "flour", fmt.Sprintf("state-%d", i), int64(i), status)
	}
	page := productHistoryResponse(t, f, "flour", 0)
	if page.TotalCount != 51 || len(page.Items) != 50 || !page.HasMore || page.Current.Status != "inactive" || page.Items[0].BeforeStatus != "active" || page.Items[49].Revision != 50 {
		t.Fatal(page)
	}
	next := productHistoryResponse(t, f, "flour", 50)
	if len(next.Items) != 1 || next.HasMore || next.Items[0].Revision != 51 || next.Items[0].BeforeStatus != "active" || next.Items[0].AfterStatus != "inactive" {
		t.Fatal(next)
	}
	if _, err := f.db.Exec(`INSERT INTO stores VALUES(?,'other','Other');INSERT INTO devices VALUES(?,'other','other-device','Other');UPDATE catalog_product_state_events SET store_id='other',device_id='other-device' WHERE revision=51`, f.owner.TenantID, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	next = productHistoryResponse(t, f, "flour", 50)
	if next.Items[0].StoreID != "other" || next.Items[0].DeviceID != "other-device" {
		t.Fatal(next)
	}
	db := restoredPurchaseDB(t, f)
	got, err := catalog.ProductStateAudit(context.Background(), db, materialsActor(f), f.device, "flour", 50)
	if err != nil || !reflect.DeepEqual(next, got) {
		t.Fatal(got, err)
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM catalog_product_state_events`).Scan(&n); err != nil || n != 51 {
		t.Fatal(n, err)
	}
}
func TestHTTPProductStateHistoryStrictParametersCorruptionAndExpiry(t *testing.T) {
	f, _ := recipeFixture(t)
	setProductStatus(t, f, "flour", "inactive", 0, "inactive")
	for _, q := range []string{"?offset=-1", "?offset=1.5", "?offset=", "?offset=0&offset=1", "?offset=9007199254740992", "?tenant_id=other", "?store_id=other"} {
		requireMaterials(t, f, "GET", productHistoryPath("flour")+q, nil, 400)
	}
	requireMaterials(t, f, "GET", productHistoryPath("missing"), nil, 404)
	before := productHistoryResponse(t, f, "flour", 0)
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", productStatePath("flour"), productStateBody("activate", 1, "active"), 403)
	if got := productHistoryResponse(t, f, "flour", 0); !reflect.DeepEqual(before, got) {
		t.Fatal(got)
	}
	a := materialsActor(f)
	a.TenantID = "foreign"
	if _, err = catalog.ProductStateAudit(context.Background(), f.db, a, f.device, "flour", 0); err == nil {
		t.Fatal("foreign")
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", productHistoryPath("flour"), nil, 403)
	requireMaterials(t, f, "GET", productStatePath("flour"), nil, 200)
	for _, q := range []string{`UPDATE catalog_product_state_events SET before_status='inactive'`, `UPDATE catalog_product_state_events SET request_json='{}'`, `DELETE FROM catalog_product_state_events`, `UPDATE catalog_product_states SET status='active'`} {
		t.Run(q, func(t *testing.T) {
			f, _ := recipeFixture(t)
			setProductStatus(t, f, "flour", "inactive", 0, "inactive")
			if _, err := f.db.Exec(q); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "GET", productHistoryPath("flour"), nil, 409)
		})
	}
}
func TestHTTPProductStateHistoryConcurrentInactivationAndSaleIsAtomic(t *testing.T) {
	f := salesSetup(t)
	in := salesInput(f)
	var wg sync.WaitGroup
	results := make(chan int, 1)
	wg.Add(2)
	go func() {
		defer wg.Done()
		status, b := request(t, f.app, "POST", productStatePath(f.product), string(mustProductStateJSON(t, "inactive", 0, "inactive")), f.token)
		if status != 200 {
			t.Errorf("state %d %s", status, b)
		}
	}()
	go func() {
		defer wg.Done()
		status, b := request(t, f.app, "POST", "/local/v1/sales", salesJSON(t, in), f.token)
		if status != 201 && status != 409 {
			t.Errorf("sale %d %s", status, b)
		}
		results <- status
	}()
	wg.Wait()
	status := <-results
	want := int64(5000)
	if status == 201 {
		want = 3000
	}
	assertStockBalance(t, f, f.shelf, want)
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM sales`).Scan(&count); err != nil || count != map[bool]int{true: 1, false: 0}[status == 201] {
		t.Fatal(count, err)
	}
	audit := productHistoryResponse(t, f.httpContractFixture, f.product, 0)
	if audit.TotalCount != 1 || audit.Current.Status != "inactive" {
		t.Fatal(audit)
	}
	next := in
	next.OperationID = "next"
	next.SaleID = "next"
	salesRequest(t, f, next, 409)
	db := restoredPurchaseDB(t, f.httpContractFixture)
	var restoredBalance int64
	if err := db.QueryRow(`SELECT COALESCE(sum(quantity_milli),0) FROM stock_movements WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, f.owner.TenantID, f.owner.StoreID, f.product, f.shelf).Scan(&restoredBalance); err != nil || restoredBalance != want {
		t.Fatal(restoredBalance, err)
	}
	var restoredSales int
	if err := db.QueryRow(`SELECT count(*) FROM sales`).Scan(&restoredSales); err != nil || restoredSales != count {
		t.Fatal(restoredSales, err)
	}
	state, err := catalog.ProductStateAudit(context.Background(), db, materialsActor(f.httpContractFixture), f.device, f.product, 0)
	if err != nil || !reflect.DeepEqual(audit, state) {
		t.Fatal(state, err)
	}
}
func mustProductStateJSON(t *testing.T, op string, rev int64, status string) []byte {
	t.Helper()
	b, err := json.Marshal(productStateBody(op, rev, status))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
