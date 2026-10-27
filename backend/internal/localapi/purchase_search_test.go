package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/replenishment"
)

const purchaseSearchPath = "/local/v1/purchase-order-search"

func purchaseSearchPage(t *testing.T, f *httpContractFixture, query string) purchases.SearchPage {
	t.Helper()
	body := requireMaterials(t, f, "GET", purchaseSearchPath+query, nil, 200)
	var out purchases.SearchPage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err, string(body))
	}
	return out
}
func purchaseSearchAdd(t *testing.T, f *httpContractFixture, id, product, supplier string) {
	t.Helper()
	// Seed disposable historical snapshots for read pagination. This does not
	// change or bypass approval rules in production code.
	for _, q := range []string{
		`INSERT INTO restock_suggestions SELECT tenant_id,store_id,device_id,?, ?,actor_identity_id,product_id,observed_milli,minimum_milli,target_milli,policy_revision,recommended_milli,status,created_at FROM restock_suggestions WHERE id=(SELECT suggestion_id FROM purchase_orders WHERE id='order')`,
		`INSERT INTO restock_reviews SELECT tenant_id,store_id,device_id,?, ?,reviewer_identity_id,decision,reason,decided_at FROM restock_reviews WHERE suggestion_id=(SELECT suggestion_id FROM purchase_orders WHERE id='order')`,
	} {
		if _, err := f.db.Exec(q, id+"-suggestion", id+"-suggestion"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.db.Exec(`INSERT INTO purchase_orders SELECT tenant_id,store_id,?,device_id,?,created_by,?,(SELECT name FROM purchase_suppliers WHERE tenant_id=o.tenant_id AND store_id=o.store_id AND id=?),?,approved_by,approved_at,status,created_at FROM purchase_orders o WHERE id='order'`, id, id+"-op", supplier, supplier, id+"-suggestion"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO purchase_order_items SELECT tenant_id,store_id,?,product_id,sku,name,unit,quantity_milli FROM purchase_order_items WHERE order_id='order' AND product_id=?`, id, product); err != nil {
		t.Fatal(err)
	}
}
func purchaseSearchState(t *testing.T, f *httpContractFixture) []int64 {
	t.Helper()
	out := traceReadState(t, f)
	for _, table := range []string{"purchase_orders", "purchase_order_items", "purchase_audit", "restock_suggestions", "restock_reviews"} {
		var n int64
		if err := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}
func TestHTTPPurchaseSearchSnapshotsFiltersAndNoWrites(t *testing.T) {
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	initial := purchaseSearchPage(t, f, "")
	if initial.TotalCount != 1 || len(initial.Items) != 1 || len(initial.Items[0].Items) != 1 {
		t.Fatal(initial)
	}
	product := initial.Items[0].Items[0].ProductID
	for _, statement := range []string{`UPDATE products SET name='Changed',sku='Changed',unit='kg'`, `UPDATE purchase_suppliers SET name='Changed',status='inactive'`, `UPDATE restock_suggestions SET recommended_milli=9999`} {
		if _, err := f.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	before := purchaseSearchState(t, f)
	for _, query := range []string{"", "?supplier_id=supplier", "?product_id=" + product, "?supplier_id=supplier&product_id=" + product} {
		out := purchaseSearchPage(t, f, query)
		if out.TotalCount != 1 || out.Limit != 50 || out.Offset != 0 || out.HasMore || !reflect.DeepEqual(initial.Items, out.Items) {
			t.Fatal(out)
		}
		v := out.Items[0]
		if v.Status != "local_not_sent" || v.SupplierName != "Padaria" || v.ApprovedBy != f.owner.OwnerID || v.ApprovedAt == "" || v.Items[0].Name != "Arroz" || v.Items[0].Quantity != 5000 {
			t.Fatal(v)
		}
	}
	for _, query := range []string{"?supplier_id=missing", "?product_id=missing", "?product_id=" + url.QueryEscape("' OR 1=1 --"), "?supplier_id=missing&product_id=" + product} {
		out := purchaseSearchPage(t, f, query)
		if out.TotalCount != 0 || out.Items == nil || len(out.Items) != 0 || out.HasMore {
			t.Fatal(out)
		}
	}
	for _, unit := range []string{"unit", "kg", "g", "liter", "ml", "meter"} {
		// All existing catalog units remain valid purchase snapshots, without conversion.
		if _, err := f.db.Exec(`UPDATE purchase_order_items SET unit=?,quantity_milli=?`, unit, purchases.MaxQuantity); err != nil {
			t.Fatal(err)
		}
		out := purchaseSearchPage(t, f, "")
		if out.Items[0].Items[0].Unit != unit || out.Items[0].Items[0].Quantity != purchases.MaxQuantity {
			t.Fatal(out)
		}
	}
	if !reflect.DeepEqual(before, purchaseSearchState(t, f)) {
		t.Fatal("read changes data or clock")
	}
}
func TestHTTPPurchaseSearchPaginationAndCombinedFilters(t *testing.T) {
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	product := purchaseSearchPage(t, f, "").Items[0].Items[0].ProductID
	for i := 0; i < 52; i++ {
		purchaseSearchAdd(t, f, fmt.Sprintf("order-%02d", i), product, "supplier")
	}
	requireMaterials(t, f, "POST", "/local/v1/purchase-suppliers", purchases.SupplierInput{OperationID: "supplier2-op", ID: "supplier2", Name: "Other", Status: "active"}, 201)
	purchaseSearchAdd(t, f, "order-other", product, "supplier2")
	if _, err := f.db.Exec(`UPDATE purchase_orders SET created_at='2024-02-29T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	first := purchaseSearchPage(t, f, "?supplier_id=supplier&product_id="+product)
	second := purchaseSearchPage(t, f, "?supplier_id=supplier&product_id="+product+"&offset=50")
	if first.TotalCount != 53 || len(first.Items) != 50 || !first.HasMore || first.Items[0].ID != "order-51" || second.TotalCount != 53 || len(second.Items) != 3 || second.HasMore || second.Items[2].ID != "order" {
		t.Fatal(first, second)
	}
	seen := map[string]bool{}
	for _, page := range []purchases.SearchPage{first, second} {
		for _, order := range page.Items {
			if seen[order.ID] || len(order.Items) != 1 || order.Items[0].ProductID != product || order.Items[0].Quantity != 5000 || order.SupplierID != "supplier" {
				t.Fatal(order)
			}
			seen[order.ID] = true
		}
	}
	if len(seen) != 53 {
		t.Fatal(seen)
	}
	for _, offset := range []string{"53", "100", "9007199254740991"} {
		out := purchaseSearchPage(t, f, "?supplier_id=supplier&offset="+offset)
		if out.TotalCount != 53 || out.Items == nil || len(out.Items) != 0 || out.HasMore {
			t.Fatal(out)
		}
	}
	if out := purchaseSearchPage(t, f, "?supplier_id=supplier2&product_id="+product); out.TotalCount != 1 || out.Items[0].ID != "order-other" {
		t.Fatal(out)
	}
}
func TestHTTPPurchaseSearchQueryAuthorizationExpiryAndIsolation(t *testing.T) {
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	for _, query := range []string{"?supplier_id=", "?product_id=", "?supplier_id=%20supplier", "?product_id=%00", "?product_id=bad%0A", "?product_id=" + strings.Repeat("x", 129), "?supplier_id=x&supplier_id=y", "?offset=", "?offset=-1", "?offset=%2B1", "?offset=1.1", "?offset=9007199254740992", "?offset=99999999999999999999999", "?offset=0&offset=1", "?tenant_id=other", "?store_id=other", "?status=sent"} {
		requireMaterials(t, f, "GET", purchaseSearchPath+query, nil, 400)
	}
	status, _ := request(t, f.app, "GET", purchaseSearchPath, "", "")
	if status != 401 {
		t.Fatal(status)
	}
	for _, tenant := range []bool{true, false} {
		a := materialsActor(f)
		if tenant {
			a.TenantID = "other"
		} else {
			a.StoreID = "other"
		}
		if _, err := purchases.Search(context.Background(), f.db, a, f.device, purchases.SearchFilter{}, 0); err == nil {
			t.Fatal("cross scope")
		}
	}
	// Independent databases keep the same local IDs separate.
	other, os := purchaseFixture(t)
	requireMaterials(t, other, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(os)), 201)
	if _, err := other.db.Exec(`UPDATE purchase_order_items SET name='Foreign'`); err != nil {
		t.Fatal(err)
	}
	otherPage := purchaseSearchPage(t, other, "")
	if otherPage.Items[0].Items[0].Name != "Foreign" || purchaseSearchPage(t, f, "").Items[0].Items[0].Name == "Foreign" {
		t.Fatal("tenant leak")
	}
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Orders}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	before := purchaseSearchState(t, f)
	purchaseSearchPage(t, f, "")
	if !reflect.DeepEqual(before, purchaseSearchState(t, f)) {
		t.Fatal("expired read changes clock/data")
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"stock", "production", "supplier", "cashier", "accountant", "employee"} {
		if _, err = f.db.Exec(`UPDATE memberships SET role=?`, role); err != nil {
			t.Fatal(err)
		}
		want := 200
		if role == "cashier" || role == "accountant" || role == "employee" {
			want = 403
		}
		requireMaterials(t, f, "GET", purchaseSearchPath, nil, want)
	}
}
func TestHTTPPurchaseSearchRejectsIncompleteSnapshot(t *testing.T) {
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	if _, err := f.db.Exec(`DELETE FROM purchase_order_items`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", purchaseSearchPath, nil, 409)
}
func TestHTTPPurchaseSearchExcludesAnotherStoreInSameDatabase(t *testing.T) {
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	product := purchaseSearchPage(t, f, "").Items[0].Items[0].ProductID
	purchaseSearchAdd(t, f, "foreign-order", product, "supplier")
	tx, err := f.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`PRAGMA defer_foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO stores VALUES(?,'other','Other')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO devices VALUES(?,'other','other-device','Other')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO restock_policies SELECT tenant_id,'other',product_id,minimum_milli,target_milli,revision,changed_by,changed_at FROM restock_policies`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`INSERT INTO purchase_suppliers SELECT tenant_id,'other',id,'other-device','other-supplier',created_by,name,status,created_at FROM purchase_suppliers`); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE restock_suggestions SET store_id='other',device_id='other-device' WHERE id='foreign-order-suggestion'`,
		`UPDATE restock_reviews SET store_id='other',device_id='other-device' WHERE suggestion_id='foreign-order-suggestion'`,
		`UPDATE purchase_orders SET store_id='other',device_id='other-device' WHERE id='foreign-order'`,
		`UPDATE purchase_order_items SET store_id='other' WHERE order_id='foreign-order'`,
	} {
		if _, err = tx.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "?supplier_id=supplier", "?product_id=" + product, "?supplier_id=supplier&product_id=" + product} {
		out := purchaseSearchPage(t, f, query)
		if out.TotalCount != 1 || len(out.Items) != 1 || out.Items[0].ID != "order" {
			t.Fatal("foreign store leak", out)
		}
	}
}
func TestHTTPPurchaseSearchBackupRestoresSnapshot(t *testing.T) {
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	before := purchaseSearchPage(t, f, "?supplier_id=supplier")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "purchases.tytbak")
	restored := filepath.Join(dir, "restored.sqlite")
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
	out, err := purchases.Search(ctx, db, materialsActor(f), f.device, purchases.SearchFilter{SupplierID: "supplier"}, 0)
	if err != nil || !reflect.DeepEqual(before, out) {
		t.Fatal(out, err)
	}
}
func TestHTTPPurchaseSearchConcurrentCreateKeepsCountAndItemsTogether(t *testing.T) {
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	var productBody map[string]any
	if err := json.Unmarshal([]byte(testProductBody), &productBody); err != nil {
		t.Fatal(err)
	}
	productBody["sku"] = "CONCURRENT"
	productBody["operation_id"] = "concurrent-product"
	b := requireMaterials(t, f, "POST", "/local/v1/products", productBody, 201)
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &created); err != nil {
		t.Fatal(err)
	}
	product := created.ID
	if _, err := replenishment.SetPolicy(context.Background(), f.db, materialsActor(f), f.device, replenishment.PolicyInput{OperationID: "concurrent-policy", ProductID: product, MinimumMilli: 1000, TargetMilli: 5000}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a := materialsActor(f)
	suggestion, err := replenishment.Suggest(ctx, f.db, a, f.device, replenishment.SuggestInput{OperationID: "concurrent-suggest", ProductID: product})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = replenishment.Review(ctx, f.db, a, f.device, replenishment.ReviewInput{OperationID: "concurrent-review", SuggestionID: suggestion.SuggestionID, Decision: "approved", Reason: "Conferido"}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	result := make(chan error, 1)
	go func() {
		defer wg.Done()
		body, _ := json.Marshal(purchases.Input{OperationID: "concurrent-op", OrderID: "concurrent", SupplierID: "supplier", SuggestionID: suggestion.SuggestionID})
		status, body := request(t, f.app, "POST", "/local/v1/purchase-orders", string(body), f.token)
		if status != 201 {
			result <- fmt.Errorf("create %d %s", status, body)
		} else {
			result <- nil
		}
	}()
	for i := 0; i < 15; i++ {
		out := purchaseSearchPage(t, f, "")
		if out.TotalCount < 1 || out.TotalCount > 2 || int64(len(out.Items)) != out.TotalCount {
			t.Fatal(out)
		}
		for _, order := range out.Items {
			if len(order.Items) != 1 || order.Items[0].Quantity != 5000 {
				t.Fatal(order)
			}
		}
	}
	wg.Wait()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
