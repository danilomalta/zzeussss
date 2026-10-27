package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/production"
)

const recipePath = "/local/v1/production/recipe-versions"

func recipeFixture(t *testing.T) (*httpContractFixture, production.PublishInput) {
	t.Helper()
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory, modules.Production}), 200)
	// Products in the disposable fixture only. HTTP/catalog integration is
	// exercised independently; these stable IDs keep recipe requests readable.
	for _, v := range []struct{ id, unit string }{{"flour", "g"}, {"oil", "ml"}, {"bread", "unit"}} {
		if _, err := f.db.Exec(`INSERT INTO products(tenant_id,id,sku,name,price_cents,cost_cents,unit) VALUES(?,?,?,?,0,0,?)`, f.owner.TenantID, v.id, v.id, v.id, v.unit); err != nil {
			t.Fatal(err)
		}
	}
	in := production.PublishInput{OperationID: "recipe-op", RecipeID: "bread-recipe", VersionID: "bread-v1", Name: "Pao", OutputProductID: "bread", OutputUnit: "unit", YieldMilli: 10000,
		Ingredients: []production.Ingredient{{ProductID: "flour", Unit: "g", QuantityMilli: 500000}, {ProductID: "oil", Unit: "ml", QuantityMilli: 100001}}}
	return f, in
}

func recipeBody(t *testing.T, in production.PublishInput) string {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func recipeCounts(t *testing.T, f *httpContractFixture, versions int) {
	t.Helper()
	for _, c := range []struct {
		table string
		want  int
	}{{"production_recipes", 1}, {"production_recipe_versions", versions}, {"production_recipe_ingredients", 2 * versions}, {"production_recipe_audit", versions}, {"outbox", versions}, {"stock_movements", 0}} {
		want := c.want
		if versions == 0 && c.table == "production_recipes" {
			want = 0
		}
		var n int
		if err := f.db.QueryRow("SELECT COUNT(*) FROM " + c.table).Scan(&n); err != nil || n != want {
			t.Fatalf("%s %d want %d: %v", c.table, n, want, err)
		}
	}
}

func TestHTTPProductionImmutableVersionsReplayAndConflicts(t *testing.T) {
	f, in := recipeFixture(t)
	for _, want := range []int{201, 200} {
		status, body := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
		if status != want {
			t.Fatalf("publish %d %s", status, body)
		}
	}
	// Ingredient ordering is not meaningful; canonical replay is stable.
	in.Ingredients[0], in.Ingredients[1] = in.Ingredients[1], in.Ingredients[0]
	status, _ := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 200 {
		t.Fatal(status)
	}
	changed := in
	changed.Name = "Outro"
	status, _ = request(t, f.app, "POST", recipePath, recipeBody(t, changed), f.token)
	if status != 409 {
		t.Fatal(status)
	}
	changed.OperationID = "other-op"
	status, _ = request(t, f.app, "POST", recipePath, recipeBody(t, changed), f.token)
	if status != 409 {
		t.Fatal(status)
	}
	v2 := in
	v2.OperationID, v2.VersionID, v2.Name, v2.ExpectedRevision, v2.YieldMilli = "v2-op", "bread-v2", "Pao novo", 1, 20000
	status, body := request(t, f.app, "POST", recipePath, recipeBody(t, v2), f.token)
	if status != 201 {
		t.Fatalf("v2 %d %s", status, body)
	}
	// Original replay still works after the head has advanced.
	status, _ = request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 200 {
		t.Fatal(status)
	}
	stale := v2
	stale.OperationID, stale.VersionID = "v3-op", "bread-v3"
	status, _ = request(t, f.app, "POST", recipePath, recipeBody(t, stale), f.token)
	if status != 409 {
		t.Fatal(status)
	}
	if _, err := f.db.Exec(`UPDATE products SET name='Changed',unit='kg'`); err != nil {
		t.Fatal(err)
	}
	status, body = request(t, f.app, "GET", recipePath+"/bread-v1", "", f.token)
	var v production.Version
	if err := json.Unmarshal(body, &v); err != nil || status != 200 || v.Name != "Pao" || v.YieldMilli != 10000 || v.Ingredients[0].Unit != "g" || v.Ingredients[1].QuantityMilli != 100001 {
		t.Fatalf("snapshot %d %s %v", status, body, err)
	}
	status, body = request(t, f.app, "GET", recipePath+"?recipe_id=bread-recipe&limit=1&offset=1", "", f.token)
	if status != 200 || !strings.Contains(string(body), `"version_id":"bread-v2"`) || strings.Contains(string(body), `"version_id":"bread-v1"`) {
		t.Fatalf("page %d %s", status, body)
	}
	recipeCounts(t, f, 2)
}

func TestHTTPProductionInvalidExactInput(t *testing.T) {
	f, base := recipeFixture(t)
	for _, kind := range []string{"zero", "negative", "overflow", "revision-overflow", "unit", "missing-product", "duplicate-ingredient", "self", "empty", "too-many"} {
		t.Run(kind, func(t *testing.T) {
			in := base
			in.Ingredients = append([]production.Ingredient(nil), base.Ingredients...)
			switch kind {
			case "zero":
				in.YieldMilli = 0
			case "negative":
				in.Ingredients[0].QuantityMilli = -1
			case "overflow":
				in.Ingredients[0].QuantityMilli = production.MaxQuantity + 1
			case "revision-overflow":
				in.ExpectedRevision = production.MaxRevision
			case "unit":
				in.Ingredients[0].Unit = "kg"
			case "missing-product":
				in.Ingredients[0].ProductID = "missing"
			case "duplicate-ingredient":
				in.Ingredients[1] = in.Ingredients[0]
			case "self":
				in.Ingredients[0].ProductID = "bread"
			case "empty":
				in.Ingredients = nil
			case "too-many":
				in.Ingredients = make([]production.Ingredient, 101)
			}
			status, b := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
			if status != 400 {
				t.Fatalf("%d %s", status, b)
			}
		})
	}
	body := recipeBody(t, base)
	for _, bad := range []string{
		strings.Replace(body, `"quantity_milli":500000`, `"quantity_milli":0.5`, 1),
		strings.Replace(body, `"quantity_milli":500000`, `"quantity_milli":null`, 1),
		strings.Replace(body, `"quantity_milli":500000`, `"quantity_milli":500000,"quantity_milli":500000`, 1),
		strings.Replace(body, `"unit":"g"`, `"unit":"g","tenant_id":"foreign"`, 1),
		strings.Replace(body, `"expected_revision":0`, `"expected_revision":0,"expected_revision":0`, 1),
		body + `{}`, `null`, `[]`,
	} {
		status, b := request(t, f.app, "POST", recipePath, bad, f.token)
		if status != 400 {
			t.Fatalf("ambiguous %d %s input %s", status, b, bad)
		}
	}
	for _, path := range []string{recipePath + "?tenant_id=foreign", recipePath + "?limit=1&limit=2", recipePath + "?offset=-1", recipePath + "?limit=101", recipePath + "/missing?store_id=other"} {
		status, _ := request(t, f.app, "GET", path, "", f.token)
		if status != 400 {
			t.Fatal(path, status)
		}
	}
	recipeCounts(t, f, 0)
}

func TestHTTPProductionAuthorization(t *testing.T) {
	for _, kind := range []string{"anonymous", "missing-contract", "wrong-module", "cashier", "production-role", "revoked", "device", "no-verifier", "expired"} {
		t.Run(kind, func(t *testing.T) {
			f, in := recipeFixture(t)
			token, want := f.token, 403
			app := f.app
			switch kind {
			case "anonymous":
				token, want = "", 401
			case "missing-contract":
				if _, e := f.db.Exec(`DELETE FROM module_contract_state`); e != nil {
					t.Fatal(e)
				}
			case "wrong-module":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory}), 200)
			case "cashier", "production-role":
				if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				role := "cashier"
				if kind == "production-role" {
					role, want = "production", 201
				}
				if _, e := f.db.Exec(`UPDATE memberships SET role=?`, role); e != nil {
					t.Fatal(e)
				}
			case "revoked":
				if _, e := f.db.Exec(`UPDATE memberships SET status='revoked'`); e != nil {
					t.Fatal(e)
				}
				want = 401
			case "device":
				if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); e != nil {
					t.Fatal(e)
				}
				want = 401
			case "no-verifier":
				var e error
				app, e = New(f.db, f.device)
				if e != nil {
					t.Fatal(e)
				}
				want = 503
			case "expired":
				now := time.Now().Unix()
				payload, e := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
				if e != nil {
					t.Fatal(e)
				}
				if _, e = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); e != nil {
					t.Fatal(e)
				}
			}
			status, body := request(t, app, "POST", recipePath, recipeBody(t, in), token)
			if status != want {
				t.Fatalf("%d want %d %s", status, want, body)
			}
			n := 0
			if want == 201 {
				n = 1
			}
			recipeCounts(t, f, n)
		})
	}
}

func TestHTTPProductionEveryWriteFailureRollsBack(t *testing.T) {
	for _, table := range []string{"production_recipes", "production_recipe_versions", "production_recipe_ingredients", "production_recipe_audit", "outbox"} {
		for _, action := range []string{"IGNORE", "ABORT,'test'"} {
			t.Run(table+action, func(t *testing.T) {
				f, in := recipeFixture(t)
				if _, e := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=1`); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec("CREATE TRIGGER fail_recipe BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(" + action + "); END"); e != nil {
					t.Fatal(e)
				}
				status, _ := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
				if status == 200 || status == 201 {
					t.Fatal("write failure reported success", status)
				}
				recipeCounts(t, f, 0)
				var observed int64
				if e := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&observed); e != nil || observed != 1 {
					t.Fatalf("clock rollback %d %v", observed, e)
				}
			})
		}
	}
}

func TestHTTPProductionConcurrentReplayAndRevision(t *testing.T) {
	f, in := recipeFixture(t)
	body := recipeBody(t, in)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, b := request(t, f.app, "POST", recipePath, body, f.token)
			if status != 201 && status != 200 {
				t.Errorf("replay %d %s", status, b)
			}
		}()
	}
	wg.Wait()
	recipeCounts(t, f, 1)
	results := make(chan int, 2)
	for _, id := range []string{"a", "b"} {
		v := in
		v.OperationID, v.VersionID, v.ExpectedRevision = id+"-op", id+"-version", 1
		b := recipeBody(t, v)
		wg.Add(1)
		go func(body string) {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", recipePath, body, f.token)
			results <- status
		}(b)
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	recipeCounts(t, f, 2)
}

func TestHTTPProductionDurableReadIsolationAndExpiredReplay(t *testing.T) {
	f, in := recipeFixture(t)
	status, _ := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 201 {
		t.Fatal(status)
	}
	var seq int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	if e := f.db.Close(); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	v, e := production.Get(context.Background(), db, a, f.device, in.VersionID)
	if e != nil || v.Revision != 1 || v.Ingredients[1].QuantityMilli != 100001 {
		t.Fatalf("reopen %+v %v", v, e)
	}
	foreign := a
	foreign.TenantID = "foreign"
	if _, e = production.Get(context.Background(), db, foreign, f.device, in.VersionID); !errors.Is(e, identity.ErrDenied) {
		t.Fatal("foreign tenant", e)
	}
	if _, e = db.Exec(`INSERT INTO stores VALUES(?,'other','Other')`, a.TenantID); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO devices VALUES(?,'other','other-device','Other')`, a.TenantID); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`UPDATE production_recipes SET store_id='other'`); e == nil {
		t.Fatal("foreign key unexpectedly allowed orphan version")
	}
	// A fully paired second store is supplied through normal setup in identity
	// tests. Here mismatching the proven device/store must fail closed.
	foreign = a
	foreign.StoreID = "other"
	if _, e = production.Get(context.Background(), db, foreign, f.device, in.VersionID); !errors.Is(e, identity.ErrDenied) {
		t.Fatal("foreign store", e)
	}
	verifier, private := testIssuer(t)
	now := time.Now().Unix()
	payload, e := json.Marshal(entitlements.Claims{Version: 1, TenantID: a.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(private, entitlements.SigningMessage("test-issuer", payload))); e != nil {
		t.Fatal(e)
	}
	app, e := NewWithVerifier(db, f.device, verifier)
	if e != nil {
		t.Fatal(e)
	}
	status, _ = request(t, app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 403 {
		t.Fatal("expired replay", status)
	}
	status, _ = request(t, app, "GET", recipePath+"/"+in.VersionID, "", f.token)
	if status != 200 {
		t.Fatal("expired read", status)
	}
}
