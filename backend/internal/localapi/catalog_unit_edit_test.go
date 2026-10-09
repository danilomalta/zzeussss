package localapi

import (
	"context"
	"testing"
	"titansystem-backend/internal/localdb/replenishment"
)

func TestHTTPCatalogUnitEditAllowsUnusedProductAndReplays(t *testing.T) {
	f, _ := recipeFixture(t)
	in := catalogEditBody("kg")
	requireMaterials(t, f, "POST", catalogEditPath, in, 200)
	requireMaterials(t, f, "POST", catalogEditPath, in, 200)
	var unit string
	if err := f.db.QueryRow(`SELECT unit FROM products WHERE id='flour'`).Scan(&unit); err != nil || unit != "kg" {
		t.Fatal(unit, err)
	}
}
func TestHTTPCatalogUnitEditRejectsHistoryAtZeroBalanceAndAnotherStore(t *testing.T) {
	f, _ := recipeFixture(t)
	for _, q := range []string{
		`INSERT INTO stores VALUES (?,'foreign-store','Other')`,
		`INSERT INTO devices VALUES (?,'foreign-store','foreign-device','Other')`,
		`INSERT INTO stock_locations VALUES (?,'foreign-store','room','production','Room')`,
	} {
		if _, err := f.db.Exec(q, f.owner.TenantID); err != nil {
			t.Fatal(err)
		}
	}
	for _, quantity := range []int64{1000, -1000} {
		id := "in"
		if quantity < 0 {
			id = "out"
		}
		if _, err := f.db.Exec(`INSERT INTO stock_movements VALUES(?,?,'foreign-store','foreign-device','flour','room',?,'test','now')`, id, f.owner.TenantID, quantity); err != nil {
			t.Fatal(err)
		}
	}
	requireMaterials(t, f, "POST", catalogEditPath, catalogEditBody("kg"), 409)
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM catalog_edit_events`).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	requireMaterials(t, f, "POST", catalogEditPath, catalogEditBody("g"), 200)
}
func TestHTTPCatalogUnitEditRejectsPolicyAndInactiveRecipe(t *testing.T) {
	f, _ := recipeFixture(t)
	if _, err := replenishment.SetPolicy(context.Background(), f.db, materialsActor(f), f.device, replenishment.PolicyInput{OperationID: "policy", ProductID: "flour", MinimumMilli: 1000, TargetMilli: 2000}); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", catalogEditPath, catalogEditBody("kg"), 409)
	other, _ := orderFixture(t)
	requireMaterials(t, other, "POST", "/local/v1/production/recipe-state", map[string]any{"operation_id": "suspend", "recipe_id": "bread-recipe", "expected_revision": 0, "status": "inactive", "reason": "Suspender"}, 200)
	requireMaterials(t, other, "POST", catalogEditPath, catalogEditBody("kg"), 409)
}
