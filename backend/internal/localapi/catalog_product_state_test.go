package localapi

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/production"
)

func productStatePath(id string) string { return "/local/v1/catalog/products/" + id + "/state" }
func productStateBody(op string, revision int64, status string) map[string]any {
	return map[string]any{"operation_id": op, "expected_revision": revision, "status": status, "reason": "revisao comercial"}
}
func setProductStatus(t *testing.T, f *httpContractFixture, id, op string, revision int64, status string) {
	t.Helper()
	requireMaterials(t, f, "POST", productStatePath(id), productStateBody(op, revision, status), 200)
}
func TestHTTPProductStateBlocksNewRecipesOrdersKeepsReplay(t *testing.T) {
	for _, id := range []string{"flour", "bread"} {
		t.Run(id, func(t *testing.T) {
			f, publish := recipeFixture(t)
			requireMaterials(t, f, "POST", recipePath, publish, 201)
			_, order := orderFixture(t)
			order.ResponsibleID = f.owner.OwnerID // location is created explicitly for this fixture
			if _, err := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,?,'production','Producao')`, f.owner.TenantID, f.owner.StoreID, order.LocationID); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "POST", orderPath, order, 201)
			setProductStatus(t, f, id, "inactive-product", 0, "inactive")
			requireMaterials(t, f, "POST", recipePath, publish, 200)
			requireMaterials(t, f, "POST", orderPath, order, 200)
			next := publish
			next.OperationID = "next-recipe"
			next.VersionID = "bread-v2"
			next.ExpectedRevision = 1
			requireMaterials(t, f, "POST", recipePath, next, 409)
			nextOrder := order
			nextOrder.OrderID = "new-order"
			nextOrder.OperationID = "new-order"
			requireMaterials(t, f, "POST", orderPath, nextOrder, 409)
			requireMaterials(t, f, "GET", recipePath+"/bread-v1", nil, 200)
			setProductStatus(t, f, id, "reactivate", 1, "active")
			requireMaterials(t, f, "POST", orderPath, nextOrder, 201)
			requireMaterials(t, f, "POST", productStatePath(id), productStateBody("inactive-product", 0, "inactive"), 200)
			state, err := catalog.GetProductState(context.Background(), f.db, materialsActor(f), f.device, id)
			if err != nil || state.Revision != 2 || state.Status != "active" {
				t.Fatal(state, err)
			}
		})
	}
}
func TestHTTPProductStateBlocksNewSaleKeepsReplayAndReturn(t *testing.T) {
	f := salesSetup(t)
	first := salesInput(f)
	salesRequest(t, f, first, 201)
	before := readReceipt(t, f, "sale-one")
	setProductStatus(t, f.httpContractFixture, f.product, "inactive", 0, "inactive")
	salesRequest(t, f, first, 200)
	next := first
	next.OperationID = "next-sale"
	next.SaleID = "next-sale"
	salesRequest(t, f, next, 409)
	if after := readReceipt(t, f, "sale-one"); !reflect.DeepEqual(before, after) {
		t.Fatal(after)
	}
	cancelRequest(t, f, cancelBody, 200)
	assertStockBalance(t, f, f.shelf, 5000)
}
func TestHTTPProductStateBlocksReplenishmentAndPurchases(t *testing.T) {
	f, p := restockFixture(t)
	policy := restockPolicyInput(p, "before-policy")
	restockCall(t, f, "policies", policy, 200)
	suggest := restockCall(t, f, "suggestions", restockSuggestInput(p, "before-suggest"), 200)
	sid := suggest["suggestion_id"].(string)
	setProductStatus(t, f, p, "inactive", 0, "inactive")
	restockCall(t, f, "policies", policy, 200)
	restockCall(t, f, "suggestions", restockSuggestInput(p, "before-suggest"), 200)
	restockCall(t, f, "policies", restockPolicyInput(p, "new-policy"), 409)
	restockCall(t, f, "suggestions", restockSuggestInput(p, "new-suggest"), 409)
	restockCall(t, f, "reviews", restockReviewInput(sid, "new-review", "approved"), 409)
	restockCall(t, f, "reviews", restockReviewInput(sid, "reject", "rejected"), 200)
	pf, s := purchaseFixture(t)
	var product string
	if err := pf.db.QueryRow(`SELECT product_id FROM restock_suggestions WHERE id=?`, s).Scan(&product); err != nil {
		t.Fatal(err)
	}
	setProductStatus(t, pf, product, "inactive", 0, "inactive")
	requireMaterials(t, pf, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 409)
	setProductStatus(t, pf, product, "active", 1, "active")
	requireMaterials(t, pf, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	before := purchaseTraceResponse(t, pf)
	setProductStatus(t, pf, product, "inactive-again", 2, "inactive")
	requireMaterials(t, pf, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 200)
	if after := purchaseTraceResponse(t, pf); !reflect.DeepEqual(before, after) {
		t.Fatal(after)
	}
}
func TestHTTPProductStateRollbackValidationScopeAndBackup(t *testing.T) {
	for _, table := range []string{"catalog_product_states", "catalog_product_state_events", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f, _ := recipeFixture(t)
			if _, err := f.db.Exec("CREATE TRIGGER reject_product_state BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "POST", productStatePath("flour"), productStateBody("inactive", 0, "inactive"), 409)
			state, err := catalog.GetProductState(context.Background(), f.db, materialsActor(f), f.device, "flour")
			if err != nil || state.Revision != 0 || state.Status != "active" {
				t.Fatal(state, err)
			}
		})
	}
	f, _ := recipeFixture(t)
	for _, field := range []string{"tenant_id", "store_id", "product_id"} {
		in := productStateBody("inactive", 0, "inactive")
		in[field] = "other"
		requireMaterials(t, f, "POST", productStatePath("flour"), in, 400)
	}
	for _, rev := range []any{-1, 1.5, 2147483647} {
		in := productStateBody("inactive", 0, "inactive")
		in["expected_revision"] = rev
		requireMaterials(t, f, "POST", productStatePath("flour"), in, 400)
	}
	requireMaterials(t, f, "GET", productStatePath("missing"), nil, 404)
	requireMaterials(t, f, "GET", productStatePath("flour")+"?tenant_id=other", nil, 400)
	setProductStatus(t, f, "flour", "inactive", 0, "inactive")
	requireMaterials(t, f, "POST", productStatePath("flour"), productStateBody("stale", 0, "active"), 409)
	db := restoredPurchaseDB(t, f)
	state, err := catalog.GetProductState(context.Background(), db, materialsActor(f), f.device, "flour")
	if err != nil || state.Status != "inactive" || state.Revision != 1 {
		t.Fatal(state, err)
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM catalog_product_state_events`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	a := materialsActor(f)
	a.TenantID = "foreign"
	if _, err = catalog.GetProductState(context.Background(), f.db, a, f.device, "flour"); err == nil {
		t.Fatal("foreign tenant")
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", productStatePath("flour"), productStateBody("denied", 1, "active"), 403)
	requireMaterials(t, f, "GET", productStatePath("flour"), nil, 200)
}
func TestHTTPProductStateExistingProductionMayComplete(t *testing.T) {
	f, in := resultsFixture(t)
	setProductStatus(t, f, "bread", "inactive", 0, "inactive")
	requireMaterials(t, f, "POST", resultsPath, in, 201)
	got, err := production.GetProductionResult(context.Background(), f.db, materialsActor(f), f.device, in.ResultID)
	if err != nil || got.ProducedMilli != 27000 {
		t.Fatal(got, err)
	}
}
