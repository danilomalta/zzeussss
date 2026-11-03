package localapi

import (
	"context"
	"crypto/rand"
	"path/filepath"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/production"
)

func TestHTTPRecipeStateLifecycleReplayAndExistingOrder(t *testing.T) {
	f, in := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, in, 201)
	state := production.RecipeStateInput{OperationID: "inactive", RecipeID: "bread-recipe", Status: "inactive", Reason: "Suspender"}
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", state, 200)
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", state, 200)
	requireMaterials(t, f, "GET", "/local/v1/production/recipes/bread-recipe/state", nil, 200)
	requireMaterials(t, f, "POST", orderPath, in, 200)
	requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID, nil, 200)
	next := in
	next.OrderID, next.OperationID = "new", "new"
	requireMaterials(t, f, "POST", orderPath, next, 409)
	state.OperationID = "stale"
	state.Status = "active"
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", state, 409)
	state.OperationID = "activate"
	state.ExpectedRevision = 1
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", state, 200)
	requireMaterials(t, f, "POST", orderPath, next, 201)
	var events int
	if err := f.db.QueryRow(`SELECT count(*) FROM production_recipe_state_events`).Scan(&events); err != nil || events != 2 {
		t.Fatal(events, err)
	}
}
func TestHTTPRecipeStateAuditFailureRollsBack(t *testing.T) {
	f, _ := orderFixture(t)
	if _, err := f.db.Exec(`CREATE TRIGGER reject_recipe_state BEFORE INSERT ON production_recipe_state_events BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", production.RecipeStateInput{OperationID: "blocked", RecipeID: "bread-recipe", Status: "inactive", Reason: "Suspender"}, 409)
	state, err := production.GetRecipeState(context.Background(), f.db, materialsActor(f), f.device, "bread-recipe")
	if err != nil || state.Status != "active" || state.Revision != 0 {
		t.Fatal(state, err)
	}
}
func TestHTTPRecipeStateBackupRestoresInactiveStateAndAudit(t *testing.T) {
	f, _ := orderFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", production.RecipeStateInput{OperationID: "suspend", RecipeID: "bread-recipe", Status: "inactive", Reason: "Suspender"}, 200)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "state.tytbak")
	restored := filepath.Join(dir, "state.sqlite")
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
	state, err := production.GetRecipeState(ctx, db, materialsActor(f), f.device, "bread-recipe")
	if err != nil || state.Status != "inactive" || state.Revision != 1 {
		t.Fatal(state, err)
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM production_recipe_state_events`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}
func TestHTTPRecipeStateRejectsInvalidInputAndScope(t *testing.T) {
	f, _ := orderFixture(t)
	for _, body := range []string{`{"operation_id":"x","recipe_id":"bread-recipe","status":"bad","reason":"x"}`, `{"operation_id":"x","recipe_id":"bread-recipe","status":"inactive","reason":""}`, `{"tenant_id":"other"}`} {
		status, _ := request(t, f.app, "POST", "/local/v1/production/recipe-state", body, f.token)
		if status != 400 {
			t.Fatal(status, body)
		}
	}
	a := materialsActor(f)
	a.StoreID = "other"
	if _, err := production.GetRecipeState(context.Background(), f.db, a, f.device, "bread-recipe"); err == nil {
		t.Fatal("cross store")
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", production.RecipeStateInput{OperationID: "denied", RecipeID: "bread-recipe", Status: "inactive", Reason: "Suspender"}, 403)
}
