package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/production"
)

const orderPath = "/local/v1/production/orders"

func orderFixture(t *testing.T) (*httpContractFixture, production.OrderInput) {
	t.Helper()
	f, recipe := recipeFixture(t)
	status, b := request(t, f.app, "POST", recipePath, recipeBody(t, recipe), f.token)
	if status != 201 {
		t.Fatalf("recipe %d %s", status, b)
	}
	if _, e := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'production-room','production','Production')`, f.owner.TenantID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	return f, production.OrderInput{OperationID: "order-op", OrderID: "order-1", VersionID: recipe.VersionID, LocationID: "production-room", ResponsibleID: f.owner.OwnerID, PlannedBatches: 3}
}

func orderBody(t *testing.T, in any) string {
	t.Helper()
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func orderCounts(t *testing.T, f *httpContractFixture, orders, events int) {
	t.Helper()
	for _, c := range []struct {
		table string
		want  int
	}{{"production_orders", orders}, {"production_order_events", events}, {"outbox", 1 + events}, {"stock_movements", 0}, {"stock_operations", 0}} {
		var n int
		if e := f.db.QueryRow("SELECT count(*) FROM " + c.table).Scan(&n); e != nil || n != c.want {
			t.Fatalf("%s %d want %d: %v", c.table, n, c.want, e)
		}
	}
}

func TestHTTPProductionOrdersSnapshotReplayStatesAndHistory(t *testing.T) {
	f, in := orderFixture(t)
	for _, want := range []int{201, 200} {
		status, b := request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
		if status != want {
			t.Fatalf("create %d %s", status, b)
		}
	}
	changed := in
	changed.PlannedBatches = 4
	status, _ := request(t, f.app, "POST", orderPath, orderBody(t, changed), f.token)
	if status != 409 {
		t.Fatal(status)
	}
	changed.OperationID = "other-op"
	status, _ = request(t, f.app, "POST", orderPath, orderBody(t, changed), f.token)
	if status != 409 {
		t.Fatal(status)
	}
	// A later recipe version and catalog edit cannot rewrite a planned order.
	var recipe production.Version
	status, b := request(t, f.app, "GET", recipePath+"/"+in.VersionID, "", f.token)
	if e := json.Unmarshal(b, &recipe); e != nil || status != 200 {
		t.Fatal(status, e)
	}
	v2 := recipe.PublishInput
	v2.OperationID, v2.VersionID, v2.ExpectedRevision, v2.YieldMilli = "recipe-v2-op", "recipe-v2", 1, 20000
	status, _ = request(t, f.app, "POST", recipePath, recipeBody(t, v2), f.token)
	if status != 201 {
		t.Fatal(status)
	}
	if _, e := f.db.Exec(`UPDATE products SET unit='kg',name='Changed'`); e != nil {
		t.Fatal(e)
	}
	status, b = request(t, f.app, "GET", orderPath+"/order-1", "", f.token)
	var order production.Order
	if e := json.Unmarshal(b, &order); e != nil || status != 200 || order.PlannedOutputMilli != 30000 || order.PlannedBatches != 3 || order.Recipe.YieldMilli != 10000 || order.Recipe.Ingredients[0].Unit != "g" {
		t.Fatalf("snapshot %d %s %v", status, b, e)
	}
	state := production.OrderStateInput{OperationID: "approve-op", OrderID: in.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Plano revisado"}
	for i := 0; i < 2; i++ {
		status, b = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
		if status != 200 {
			t.Fatalf("approve %d %s", status, b)
		}
	}
	state.OperationID, state.Status = "cancel-stale", "cancelled"
	status, _ = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
	if status != 409 {
		t.Fatal("stale", status)
	}
	state.OperationID, state.ExpectedRevision = "cancel-op", 2
	status, _ = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
	if status != 200 {
		t.Fatal(status)
	}
	state.OperationID, state.ExpectedRevision, state.Status = "revive-op", 3, "approved"
	status, _ = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
	if status != 409 {
		t.Fatal("revive", status)
	}
	// Original create replay returns its original durable result after changes.
	status, b = request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
	if status != 200 || !strings.Contains(string(b), `"status":"planned"`) {
		t.Fatalf("historical replay %d %s", status, b)
	}
	status, b = request(t, f.app, "GET", orderPath+"/order-1/history?offset=1", "", f.token)
	var page struct {
		Items []production.OrderEvent `json:"items"`
	}
	if e := json.Unmarshal(b, &page); e != nil || status != 200 || len(page.Items) != 2 || page.Items[0].BeforeStatus != "planned" || page.Items[1].AfterStatus != "cancelled" {
		t.Fatalf("history %d %s %v", status, b, e)
	}
	var movements int
	if e := f.db.QueryRow(`SELECT count(*) FROM stock_movements`).Scan(&movements); e != nil || movements != 0 {
		t.Fatal("stock changed", e)
	}
}

func TestHTTPProductionOrdersAuthorizationReferencesAndBounds(t *testing.T) {
	for _, kind := range []string{"anonymous", "module", "cashier", "responsible", "location", "version", "zero", "overflow", "duplicate-json", "tenant-field"} {
		t.Run(kind, func(t *testing.T) {
			f, in := orderFixture(t)
			token, want := f.token, 400
			switch kind {
			case "anonymous":
				token, want = "", 401
			case "module":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory}), 200)
				want = 403
			case "cashier":
				if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
					t.Fatal(e)
				}
				want = 403
			case "responsible":
				in.ResponsibleID, want = "missing", 403
			case "location":
				in.LocationID, want = "foreign", 404
			case "version":
				in.VersionID, want = "foreign", 404
			case "zero":
				in.PlannedBatches = 0
			case "overflow":
				in.PlannedBatches = production.MaxQuantity
			}
			body := orderBody(t, in)
			if kind == "duplicate-json" {
				body = strings.Replace(body, `"planned_batches":3`, `"planned_batches":3,"planned_batches":3`, 1)
			}
			if kind == "tenant-field" {
				body = strings.TrimSuffix(body, "}") + `,"tenant_id":"other"}`
			}
			status, b := request(t, f.app, "POST", orderPath, body, token)
			if status != want {
				t.Fatalf("%d want %d %s", status, want, b)
			}
			orderCounts(t, f, 0, 0)
		})
	}
}

func TestHTTPProductionOrdersWriteFailuresRollBack(t *testing.T) {
	for _, table := range []string{"production_orders", "production_order_events", "outbox"} {
		for _, action := range []string{"IGNORE", "ABORT,'test'"} {
			t.Run(table+action, func(t *testing.T) {
				f, in := orderFixture(t)
				if _, e := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=1`); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec("CREATE TRIGGER fail_order BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(" + action + "); END"); e != nil {
					t.Fatal(e)
				}
				status, _ := request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
				if status == 201 || status == 200 {
					t.Fatal("ignored failure", status)
				}
				orderCounts(t, f, 0, 0)
				var clock int64
				if e := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&clock); e != nil || clock != 1 {
					t.Fatal("clock rollback", e)
				}
			})
		}
	}
	for _, table := range []string{"production_orders", "production_order_events", "outbox"} {
		t.Run("transition-"+table, func(t *testing.T) {
			f, in := orderFixture(t)
			status, _ := request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
			if status != 201 {
				t.Fatal(status)
			}
			op := "INSERT"
			if table == "production_orders" {
				op = "UPDATE"
			}
			if _, e := f.db.Exec("CREATE TRIGGER fail_change BEFORE " + op + " ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			state := production.OrderStateInput{OperationID: "change", OrderID: in.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Plano"}
			status, _ = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
			if status == 200 {
				t.Fatal("partial transition")
			}
			var revision int
			if e := f.db.QueryRow(`SELECT revision FROM production_orders`).Scan(&revision); e != nil || revision != 1 {
				t.Fatal("transition not rolled back", e)
			}
			orderCounts(t, f, 1, 1)
		})
	}
}

func TestHTTPProductionOrdersConcurrentReplayAndRevision(t *testing.T) {
	f, in := orderFixture(t)
	body := orderBody(t, in)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, b := request(t, f.app, "POST", orderPath, body, f.token)
			if status != 200 && status != 201 {
				t.Errorf("create %d %s", status, b)
			}
		}()
	}
	wg.Wait()
	orderCounts(t, f, 1, 1)
	results := make(chan int, 2)
	for _, status := range []string{"approved", "cancelled"} {
		state := production.OrderStateInput{OperationID: status + "-op", OrderID: in.OrderID, ExpectedRevision: 1, Status: status, Reason: "Plano"}
		b := orderBody(t, state)
		wg.Add(1)
		go func(body string) {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", orderPath+"/state", body, f.token)
			results <- status
		}(b)
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
	orderCounts(t, f, 1, 2)
}

func TestHTTPProductionOrdersBackupRecoveryAndIsolation(t *testing.T) {
	f, in := orderFixture(t)
	status, _ := request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
	if status != 201 {
		t.Fatal(status)
	}
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "orders.tytbak")
	ctx := context.Background()
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Verify(ctx, archive, f.device, key); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "recovered.sqlite")
	if e := backup.Restore(ctx, archive, path, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	order, e := production.GetOrder(ctx, db, actor, f.device, in.OrderID)
	if e != nil || order.PlannedBatches != 3 || order.PlannedOutputMilli != 30000 || order.VersionID != in.VersionID || len(order.Recipe.Ingredients) != 2 {
		t.Fatalf("restore %+v %v", order, e)
	}
	history, e := production.OrderHistory(ctx, db, actor, f.device, in.OrderID, 0)
	if e != nil || len(history) != 1 {
		t.Fatal("history restore", e)
	}
	foreign := actor
	foreign.TenantID = "foreign"
	if _, e = production.GetOrder(ctx, db, foreign, f.device, in.OrderID); e == nil {
		t.Fatal("foreign tenant")
	}
	foreign = actor
	foreign.StoreID = "foreign"
	if _, e = production.ListOrders(ctx, db, foreign, f.device, 0); e == nil {
		t.Fatal("foreign store")
	}
}

func TestHTTPProductionOrdersExpiredContractBlocksReplayAndStatePreservesRead(t *testing.T) {
	f, in := orderFixture(t)
	status, _ := request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
	if status != 201 {
		t.Fatal(status)
	}
	now := time.Now().Unix()
	payload, e := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); e != nil {
		t.Fatal(e)
	}
	status, _ = request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
	if status != 403 {
		t.Fatal("expired replay", status)
	}
	state := production.OrderStateInput{OperationID: "expired-change", OrderID: in.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Plano"}
	status, _ = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
	if status != 403 {
		t.Fatal("expired state", status)
	}
	status, _ = request(t, f.app, "GET", orderPath+"/"+in.OrderID, "", f.token)
	if status != 200 {
		t.Fatal("expired read", status)
	}
	orderCounts(t, f, 1, 1)
}
