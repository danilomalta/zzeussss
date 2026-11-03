package backup

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/production"
)

func TestProductionRecipeSnapshotRestoresVersionAuditAndOutbox(t *testing.T) {
	db, device, key, dir := fixture(t)
	ctx := context.Background()
	actor := identity.Scope{TenantID: device.TenantID, StoreID: device.StoreID}
	if err := db.QueryRow(`SELECT identity_id FROM memberships WHERE tenant_id=? AND role='owner'`, device.TenantID).Scan(&actor.IdentityID); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"backup-test": public})
	if err != nil {
		t.Fatal(err)
	}
	license, err := entitlementstore.New(db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: device.TenantID, Revision: 1, IssuedAt: now - 60, NotBefore: now - 60, ExpiresAt: now + 3600, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if err != nil {
		t.Fatal(err)
	}
	envelope := entitlements.Envelope{KeyID: "backup-test", Payload: payload, Signature: ed25519.Sign(private, entitlements.SigningMessage("backup-test", payload))}
	if _, err = license.Install(ctx, actor, device, envelope); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ id, unit string }{{"flour", "g"}, {"bread", "unit"}} {
		if _, err = db.Exec(`INSERT INTO products(tenant_id,id,sku,name,price_cents,cost_cents,unit) VALUES(?,?,?,?,0,0,?)`, device.TenantID, p.id, p.id, p.id, p.unit); err != nil {
			t.Fatal(err)
		}
	}
	in := production.PublishInput{OperationID: "backup-recipe-op", RecipeID: "bread-recipe", VersionID: "bread-v1", Name: "Pao", OutputProductID: "bread", OutputUnit: "unit", YieldMilli: 10000, Ingredients: []production.Ingredient{{ProductID: "flour", Unit: "g", QuantityMilli: 500001}}}
	if _, err = production.Publish(ctx, db, license, actor, device, in); err != nil {
		t.Fatal(err)
	}
	original, err := production.Get(ctx, db, actor, device, in.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "recipe.tytbak")
	if err = Create(ctx, db, device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err = Verify(ctx, archive, device, key); err != nil {
		t.Fatal(err)
	}
	restoredPath := filepath.Join(dir, "recipe-restored.sqlite")
	if err = Restore(ctx, archive, restoredPath, device, key); err != nil {
		t.Fatal(err)
	}
	restored, err := localdb.Open(ctx, restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	got, err := production.Get(ctx, restored, actor, device, in.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	wantBody, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	gotBody, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != string(wantBody) {
		t.Fatalf("recipe changed after restore: %s", gotBody)
	}
	for _, query := range []string{
		`SELECT count(*) FROM production_recipe_audit WHERE version_id='bread-v1'`,
		`SELECT count(*) FROM production_recipe_ingredients WHERE version_id='bread-v1' AND quantity_milli=500001`,
		`SELECT count(*) FROM outbox WHERE aggregate_id='bread-v1' AND event_type='production.recipe.published' AND status='pending'`,
	} {
		var n int
		if err = restored.QueryRow(query).Scan(&n); err != nil || n != 1 {
			t.Fatalf("restored record count %d: %v", n, err)
		}
	}
}
