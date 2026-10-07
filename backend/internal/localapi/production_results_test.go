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
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/stock"
)

const resultsPath = "/local/v1/production/results"

func resultsFixture(t *testing.T) (*httpContractFixture, production.ResultInput) {
	t.Helper()
	f, reserve := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "Executar"}, 200)
	return f, production.ResultInput{OperationID: "complete", ResultID: "result-1", OrderID: reserve.OrderID, ExpectedRevision: 2, ProducedMilli: 27000, Reason: "27 unidades boas, rendimento menor que o plano"}
}

func TestHTTPProductionResultsActualYieldReplayAndOrderCompletion(t *testing.T) {
	f, in := resultsFixture(t)
	ctx := context.Background()
	actor := materialsActor(f)
	for _, want := range []int{201, 200} {
		requireMaterials(t, f, "POST", resultsPath, in, want)
	}
	var result production.ProductionResult
	body := requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID, nil, 200)
	if err := json.Unmarshal(body, &result); err != nil || result.PlannedMilli != 30000 || result.ProducedMilli != 27000 || result.ShortfallMilli != 3000 || result.Unit != "unit" || result.Status != "completed" || result.Revision != 3 || result.ActorID != actor.IdentityID || result.MovementID == "" || result.Reason != in.Reason {
		t.Fatalf("result %s %v", body, err)
	}
	for _, item := range []struct {
		product string
		want    int64
	}{{"bread", 27000}, {"flour", 500000}, {"oil", 100001}} {
		n, err := stock.Balance(ctx, f.db, actor, f.device, item.product, "production-room")
		if err != nil || n != item.want {
			t.Fatal(item.product, n, err)
		}
	}
	materialCount(t, f, "stock_movements", 5)
	materialCount(t, f, "production_results", 1)
	materialCount(t, f, "outbox", 6)
	var order production.Order
	body = requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID, nil, 200)
	if err := json.Unmarshal(body, &order); err != nil || order.Status != "completed" || order.CompletionID != in.ResultID || order.Revision != 3 || order.PlannedOutputMilli != 30000 || order.Recipe.YieldMilli != 10000 {
		t.Fatal("order", string(body), err)
	}
	orders, err := production.ListOrders(ctx, f.db, actor, f.device, 0)
	if err != nil || len(orders) != 1 || orders[0].Status != "completed" || orders[0].CompletionID != in.ResultID {
		t.Fatal("list", orders, err)
	}
	history, err := production.OrderHistory(ctx, f.db, actor, f.device, in.OrderID, 0)
	if err != nil || len(history) != 3 || history[2].Kind != "completed" || history[2].Revision != 3 || history[2].Reason != in.Reason {
		t.Fatal("history", history, err)
	}
	history, err = production.OrderHistory(ctx, f.db, actor, f.device, in.OrderID, 2)
	if err != nil || len(history) != 1 || history[0].AfterStatus != "completed" {
		t.Fatal("history paging", history, err)
	}
	changed := in
	changed.ProducedMilli = 28000
	requireMaterials(t, f, "POST", resultsPath, changed, 409)
	changed = in
	changed.OperationID, changed.ResultID = "duplicate-result", "other-result"
	requireMaterials(t, f, "POST", resultsPath, changed, 409)
	requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: "cancel-completed", OrderID: in.OrderID, ExpectedRevision: 3, Status: "cancelled", Reason: "Teste"}, 409)
	requireMaterials(t, f, "POST", materialsPath, production.ReserveInput{OperationID: "reserve-completed", ReservationID: "after-completion", OrderID: in.OrderID, Reason: "Teste"}, 409)
	materialCount(t, f, "stock_movements", 5)
	// A later recipe version must not rewrite the actual completion or its plan.
	var v production.Version
	body = requireMaterials(t, f, "GET", recipePath+"/bread-v1", nil, 200)
	if err = json.Unmarshal(body, &v); err != nil {
		t.Fatal(err)
	}
	v.OperationID, v.VersionID, v.ExpectedRevision, v.YieldMilli = "v2-op", "v2", 1, 15000
	requireMaterials(t, f, "POST", recipePath, v.PublishInput, 201)
	got, err := production.GetProductionResult(ctx, f.db, actor, f.device, in.ResultID)
	if err != nil || got.PlannedMilli != 30000 || got.ProducedMilli != 27000 {
		t.Fatal(got, err)
	}
}

func TestHTTPProductionResultsZeroAndExactYield(t *testing.T) {
	for _, quantity := range []int64{0, 30000} {
		name := "zero"
		if quantity > 0 {
			name = "exact"
		}
		t.Run(name, func(t *testing.T) {
			f, in := resultsFixture(t)
			in.ProducedMilli = quantity
			in.Reason = "Resultado medido"
			requireMaterials(t, f, "POST", resultsPath, in, 201)
			got, err := production.GetProductionResult(context.Background(), f.db, materialsActor(f), f.device, in.ResultID)
			if err != nil || got.ShortfallMilli != 30000-quantity || (quantity == 0 && got.MovementID != "") || (quantity > 0 && got.MovementID == "") {
				t.Fatal(got, err)
			}
			expected := 4
			if quantity > 0 {
				expected = 5
			}
			materialCount(t, f, "stock_movements", expected)
			n, err := stock.Balance(context.Background(), f.db, materialsActor(f), f.device, "bread", "production-room")
			if err != nil || n != quantity {
				t.Fatal(n, err)
			}
		})
	}
}

func TestHTTPProductionResultsValidationAndConsumedProof(t *testing.T) {
	for _, kind := range []string{"negative", "over-plan", "fraction", "stale", "foreign-order", "unit", "overflow", "active", "released", "movement", "role", "responsible", "module"} {
		t.Run(kind, func(t *testing.T) {
			f, in := resultsFixture(t)
			want := 409
			switch kind {
			case "negative":
				in.ProducedMilli = -1
				want = 400
			case "over-plan":
				in.ProducedMilli = 31000
				want = 400
			case "fraction":
				in.ProducedMilli = 27001
				want = 400
			case "stale":
				in.ExpectedRevision = 1
			case "foreign-order":
				in.OrderID = "other-company-order"
				want = 404
			case "unit":
				if _, err := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='bread'`); err != nil {
					t.Fatal(err)
				}
			case "overflow":
				addCapacityMovement(t, f, "output-full", "bread", "production-room", production.MaxQuantity)
			case "active":
				if _, err := f.db.Exec(`UPDATE production_material_reservations SET status='active'`); err != nil {
					t.Fatal(err)
				}
			case "released":
				if _, err := f.db.Exec(`UPDATE production_material_reservations SET status='released'`); err != nil {
					t.Fatal(err)
				}
			case "movement":
				if _, err := f.db.Exec(`UPDATE stock_movements SET quantity_milli=-1 WHERE reason LIKE 'production-consume:%'`); err != nil {
					t.Fatal(err)
				}
			case "role":
				if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
					t.Fatal(err)
				}
				want = 403
			case "responsible":
				if _, err := f.db.Exec(`INSERT INTO identities VALUES('inactive','Responsavel','now')`); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`INSERT INTO memberships VALUES(?,'inactive','production','revoked','now')`, f.owner.TenantID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`UPDATE production_orders SET responsible_id='inactive'`); err != nil {
					t.Fatal(err)
				}
				want = 403
			case "module":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory}), 200)
				want = 403
			}
			requireMaterials(t, f, "POST", resultsPath, in, want)
			materialCount(t, f, "production_results", 0)
			var revision int
			if err := f.db.QueryRow(`SELECT revision FROM production_orders`).Scan(&revision); err != nil || revision != 2 {
				t.Fatal(revision, err)
			}
		})
	}
	f, in := resultsFixture(t)
	for _, body := range []string{`{}`, strings.Replace(orderBody(t, in), `"produced_milli":27000`, `"produced_milli":27000,"produced_milli":27000`, 1), strings.TrimSuffix(orderBody(t, in), "}") + `,"tenant_id":"other"}`, strings.Replace(orderBody(t, in), `"produced_milli":27000`, `"produced_milli":null`, 1), strings.Replace(orderBody(t, in), `"produced_milli":27000`, `"produced_milli":27000.0`, 1)} {
		status, _ := request(t, f.app, "POST", resultsPath, body, f.token)
		if status != 400 {
			t.Fatal("strict JSON", status, body)
		}
	}
	status, _ := request(t, f.app, "POST", resultsPath, orderBody(t, in), "")
	if status != 401 {
		t.Fatal("anonymous", status)
	}
	requireMaterials(t, f, "POST", resultsPath+"?ignored=1", in, 400)
}

func TestHTTPProductionResultsRollbackOnIgnoredOrAbortedWrite(t *testing.T) {
	for _, table := range []string{"production_results", "production_orders", "stock_movements", "outbox"} {
		for _, action := range []string{"IGNORE", "ABORT,'test'"} {
			t.Run(table+action, func(t *testing.T) {
				f, in := resultsFixture(t)
				operation := "INSERT"
				if table == "production_orders" {
					operation = "UPDATE"
				}
				if _, err := f.db.Exec("CREATE TRIGGER fail_result BEFORE " + operation + " ON " + table + " BEGIN SELECT RAISE(" + action + "); END"); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=1`); err != nil {
					t.Fatal(err)
				}
				status, _ := request(t, f.app, "POST", resultsPath, orderBody(t, in), f.token)
				if status < 400 {
					t.Fatal("ignored write", status)
				}
				materialCount(t, f, "production_results", 0)
				materialCount(t, f, "stock_movements", 4)
				materialCount(t, f, "outbox", 5)
				var revision int
				var clock int64
				if err := f.db.QueryRow(`SELECT revision FROM production_orders`).Scan(&revision); err != nil || revision != 2 {
					t.Fatal("revision rollback", revision, err)
				}
				if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&clock); err != nil || clock != 1 {
					t.Fatal("clock rollback", clock, err)
				}
			})
		}
	}
}

func TestHTTPProductionResultsConcurrentCompletionCannotDuplicateStock(t *testing.T) {
	for _, same := range []bool{false, true} {
		name := "different-operations"
		if same {
			name = "same-operation"
		}
		t.Run(name, func(t *testing.T) {
			f, in := resultsFixture(t)
			other := in
			if !same {
				other.OperationID, other.ResultID = "other-complete", "other-result"
			}
			bodies := []string{orderBody(t, in), orderBody(t, other)}
			statuses := make(chan int, 2)
			var wg sync.WaitGroup
			for _, body := range bodies {
				wg.Add(1)
				go func(b string) {
					defer wg.Done()
					status, _ := request(t, f.app, "POST", resultsPath, b, f.token)
					statuses <- status
				}(body)
			}
			wg.Wait()
			close(statuses)
			created, second := 0, 0
			for status := range statuses {
				if status == 201 {
					created++
				} else if (same && status == 200) || (!same && status == 409) {
					second++
				} else {
					t.Fatal(status)
				}
			}
			if created != 1 || second != 1 {
				t.Fatal(created, second)
			}
			materialCount(t, f, "production_results", 1)
			materialCount(t, f, "stock_movements", 5)
		})
	}
}

func TestHTTPProductionResultsBackupRestoresCompletionAndIsolation(t *testing.T) {
	f, in := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, in, 201)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "results.tytbak")
	path := filepath.Join(dir, "recovered.sqlite")
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err := backup.Verify(ctx, archive, f.device, key); err != nil {
		t.Fatal(err)
	}
	if err := backup.Restore(ctx, archive, path, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := materialsActor(f)
	got, err := production.GetProductionResult(ctx, db, actor, f.device, in.ResultID)
	if err != nil || got.ProducedMilli != 27000 || got.ShortfallMilli != 3000 || got.MovementID == "" || got.ReservationID != "materials-1" {
		t.Fatal("restored result", got, err)
	}
	order, err := production.GetOrder(ctx, db, actor, f.device, in.OrderID)
	if err != nil || order.Status != "completed" || order.CompletionID != in.ResultID || order.Revision != 3 || len(order.Recipe.Ingredients) != 2 {
		t.Fatal("restored order", order, err)
	}
	history, err := production.OrderHistory(ctx, db, actor, f.device, in.OrderID, 0)
	if err != nil || len(history) != 3 || history[2].AfterStatus != "completed" {
		t.Fatal(history, err)
	}
	balance, err := stock.Balance(ctx, db, actor, f.device, "bread", "production-room")
	if err != nil || balance != 27000 {
		t.Fatal(balance, err)
	}
	foreign := actor
	foreign.TenantID = "other-company"
	if _, err = production.GetProductionResult(ctx, db, foreign, f.device, in.ResultID); err == nil {
		t.Fatal("company isolation")
	}
	foreign = actor
	foreign.StoreID = "other-store"
	if _, err = production.GetProductionResult(ctx, db, foreign, f.device, in.ResultID); err == nil {
		t.Fatal("store isolation")
	}
}

func TestHTTPProductionResultsExpiredContractBlocksWritesPreservesHistory(t *testing.T) {
	f, in := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, in, 201)
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", resultsPath, in, 403)
	requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID, nil, 200)
	requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID, nil, 200)
	materialCount(t, f, "production_results", 1)
	materialCount(t, f, "stock_movements", 5)
}
