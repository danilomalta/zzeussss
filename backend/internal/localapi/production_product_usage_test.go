package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
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
	"titansystem-backend/internal/localdb/production"
)

func usagePath(id string) string { return "/local/v1/catalog/products/" + id + "/production-usage" }
func usageResponse(t *testing.T, f *httpContractFixture, id, query string) production.ProductUsage {
	t.Helper()
	body := requireMaterials(t, f, "GET", usagePath(id)+query, nil, 200)
	if strings.Contains(string(body), "cost_cents") || strings.Contains(string(body), "price_cents") {
		t.Fatal("unnecessary commercial fields")
	}
	var out production.ProductUsage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err, string(body))
	}
	return out
}
func TestHTTPProductionProductUsageHistoricalLatestAndFrozenPlans(t *testing.T) {
	f, in := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, in, 201)
	var v1 production.Version
	body := requireMaterials(t, f, "GET", recipePath+"/"+in.VersionID, nil, 200)
	if err := json.Unmarshal(body, &v1); err != nil {
		t.Fatal(err)
	}
	v2 := v1.PublishInput
	v2.OperationID, v2.VersionID, v2.ExpectedRevision, v2.YieldMilli = "v2", "bread-v2", 1, 20000
	v2.Ingredients = []production.Ingredient{{ProductID: "oil", Unit: "ml", QuantityMilli: 200001}}
	requireMaterials(t, f, "POST", recipePath, v2, 201)
	in.OperationID, in.OrderID, in.VersionID = "second-order", "second-order", v2.VersionID
	requireMaterials(t, f, "POST", orderPath, in, 201)
	before := traceReadState(t, f)
	flour := usageResponse(t, f, "flour", "")
	if flour.RecipeVersionCount != 1 || flour.LatestRecipeVersionCount != 0 || flour.OrderCount != 1 || len(flour.Items) != 1 || flour.Items[0].Role != "ingredient" || flour.Items[0].QuantityMilli != 500000 || flour.Items[0].QuantityBasis != "per_batch" || flour.Items[0].LatestVersion == nil || *flour.Items[0].LatestVersion {
		t.Fatal(flour)
	}
	bread := usageResponse(t, f, "bread", "")
	if bread.RecipeVersionCount != 2 || bread.LatestRecipeVersionCount != 1 || bread.OrderCount != 2 || bread.Items[0].Role != "output" || bread.Items[0].QuantityMilli != 10000 || *bread.Items[0].LatestVersion || !*bread.Items[1].LatestVersion || bread.Items[1].QuantityMilli != 20000 {
		t.Fatal(bread)
	}
	flour = usageResponse(t, f, "flour", "?kind=orders")
	if len(flour.Items) != 1 || flour.Items[0].ID != "order-1" || flour.Items[0].VersionID != v1.VersionID || flour.Items[0].QuantityMilli != 1500000 || flour.Items[0].QuantityBasis != "planned_total" || flour.Items[0].OrderStatus != "planned" || flour.Items[0].LatestVersion != nil {
		t.Fatal(flour)
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("read writes data/clock")
	}
	if _, err := f.db.Exec(`UPDATE products SET name='Current flour',unit='kg' WHERE id='flour'`); err != nil {
		t.Fatal(err)
	}
	flour = usageResponse(t, f, "flour", "?kind=orders")
	if flour.Product.Name != "Current flour" || flour.Product.Unit != "kg" || flour.Items[0].Unit != "g" || flour.Items[0].UnitMatchesCatalog || flour.Items[0].QuantityMilli != 1500000 {
		t.Fatal(flour)
	}
	if _, err := f.db.Exec(`INSERT INTO products(tenant_id,id,sku,name,price_cents,cost_cents,unit) VALUES(?,'unused','unused','Unused',0,0,'unit')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	unused := usageResponse(t, f, "unused", "")
	if unused.RecipeVersionCount != 0 || unused.LatestRecipeVersionCount != 0 || unused.OrderCount != 0 || unused.Items == nil || len(unused.Items) != 0 || unused.HasMore || unused.StoreID != f.owner.StoreID {
		t.Fatal(unused)
	}
}
func TestHTTPProductionProductUsagePaginationOfVersionsAndOrders(t *testing.T) {
	f, in := orderFixture(t)
	var v production.Version
	body := requireMaterials(t, f, "GET", recipePath+"/"+in.VersionID, nil, 200)
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 53; i++ {
		if i > 1 {
			next := v.PublishInput
			next.OperationID = fmt.Sprint("version-op", i)
			next.VersionID = fmt.Sprint("version-", i)
			next.ExpectedRevision = int64(i - 1)
			requireMaterials(t, f, "POST", recipePath, next, 201)
		}
		in.OperationID = fmt.Sprintf("order-op-%02d", i)
		in.OrderID = fmt.Sprintf("order-%02d", i)
		requireMaterials(t, f, "POST", orderPath, in, 201)
	}
	if _, err := f.db.Exec(`UPDATE production_orders SET created_at='2024-02-29T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"versions", "orders"} {
		first := usageResponse(t, f, "bread", "?kind="+kind)
		second := usageResponse(t, f, "bread", "?kind="+kind+"&offset=50")
		if first.RecipeVersionCount != 53 || first.LatestRecipeVersionCount != 1 || first.OrderCount != 53 || len(first.Items) != 50 || !first.HasMore || len(second.Items) != 3 || second.HasMore {
			t.Fatal(first, second)
		}
		if kind == "versions" && (!*second.Items[2].LatestVersion || first.Items[0].ID != v.VersionID) {
			t.Fatal(second)
		}
		if kind == "orders" && (first.Items[0].ID != "order-53" || second.Items[2].ID != "order-01") {
			t.Fatal(first, second)
		}
		for _, offset := range []string{"53", "100", "9007199254740991"} {
			out := usageResponse(t, f, "bread", "?kind="+kind+"&offset="+offset)
			if out.RecipeVersionCount != 53 || out.OrderCount != 53 || out.Items == nil || len(out.Items) != 0 || out.HasMore {
				t.Fatal(out)
			}
		}
	}
}
func TestHTTPProductionProductUsageAuthorizationBoundsAndExpiredHistory(t *testing.T) {
	f, in := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, in, 201)
	for _, query := range []string{"?kind=all", "?kind=", "?kind=versions&kind=orders", "?offset=-1", "?offset=%2B1", "?offset=1.5", "?offset=", "?offset=9007199254740992", "?offset=99999999999999999999999", "?offset=0&offset=1", "?tenant_id=other", "?store_id=other", "?limit=100"} {
		requireMaterials(t, f, "GET", usagePath("bread")+query, nil, 400)
	}
	requireMaterials(t, f, "GET", usagePath("missing"), nil, 404)
	status, _ := request(t, f.app, "GET", usagePath("bread"), "", "")
	if status != 401 {
		t.Fatal(status)
	}
	for _, company := range []bool{true, false} {
		foreign := materialsActor(f)
		if company {
			foreign.TenantID = "other"
		} else {
			foreign.StoreID = "other"
		}
		if _, err := production.GetProductUsage(context.Background(), f.db, foreign, f.device, "bread", "versions", 0); err == nil {
			t.Fatal("cross scope")
		}
	}
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	before := traceReadState(t, f)
	usageResponse(t, f, "bread", "")
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("clock changed")
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", usagePath("bread"), nil, 403)
}
func TestHTTPProductionProductUsageBackupRestoresCompletedOrderAndIngredients(t *testing.T) {
	f, stages, _ := traceFixture(t)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "usage.tytbak")
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
	out, err := production.GetProductUsage(ctx, db, materialsActor(f), f.device, "oil", "orders", 0)
	if err != nil || out.RecipeVersionCount != 1 || out.OrderCount != 1 || out.Items[0].ID != stages.OrderID || out.Items[0].OrderStatus != "completed" || out.Items[0].Role != "ingredient" || out.Items[0].QuantityMilli != 300003 {
		t.Fatal(out, err)
	}
}
func TestHTTPProductionProductUsageConcurrentVersionKeepsOneSnapshot(t *testing.T) {
	f, in := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, in, 201)
	var v production.Version
	body := requireMaterials(t, f, "GET", recipePath+"/"+in.VersionID, nil, 200)
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	statuses := make(chan int, 2)
	go func() {
		defer wg.Done()
		for i := 2; i <= 5; i++ {
			next := v.PublishInput
			next.OperationID = fmt.Sprint("v-op", i)
			next.VersionID = fmt.Sprint("v-", i)
			next.ExpectedRevision = int64(i - 1)
			status, _ := request(t, f.app, "POST", recipePath, recipeBody(t, next), f.token)
			if status != 201 {
				statuses <- status
				return
			}
		}
		statuses <- 200
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			status, body := request(t, f.app, "GET", usagePath("bread"), "", f.token)
			var out production.ProductUsage
			if status != 200 || json.Unmarshal(body, &out) != nil || out.RecipeVersionCount != int64(len(out.Items)) || out.LatestRecipeVersionCount != 1 {
				statuses <- 500
				return
			}
			latest := 0
			for _, link := range out.Items {
				if link.LatestVersion != nil && *link.LatestVersion {
					latest++
				}
			}
			if latest != 1 {
				statuses <- 500
				return
			}
		}
		statuses <- 200
	}()
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatal(status)
		}
	}
}
