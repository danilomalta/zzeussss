package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
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
	"titansystem-backend/internal/localdb/inventory"
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/stock"
)

const materialsPath = "/local/v1/production/material-reservations"

func materialsFixture(t *testing.T) (*httpContractFixture, production.ReserveInput) {
	t.Helper()
	f, order := orderFixture(t)
	status, body := request(t, f.app, "POST", orderPath, orderBody(t, order), f.token)
	if status != 201 {
		t.Fatalf("order %d %s", status, body)
	}
	state := production.OrderStateInput{OperationID: "approve", OrderID: order.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Planejado"}
	status, body = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
	if status != 200 {
		t.Fatalf("approval %d %s", status, body)
	}
	addCapacityMovement(t, f, "initial-flour", "flour", order.LocationID, 2000000)
	addCapacityMovement(t, f, "initial-oil", "oil", order.LocationID, 400004)
	return f, production.ReserveInput{OperationID: "reserve", ReservationID: "materials-1", OrderID: order.OrderID, Reason: "Separar ingredientes"}
}

func materialsActor(f *httpContractFixture) identity.Scope {
	return identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
}

func requireMaterials(t *testing.T, f *httpContractFixture, method, path string, input any, want int) []byte {
	t.Helper()
	body := ""
	if input != nil {
		body = orderBody(t, input)
	}
	status, result := request(t, f.app, method, path, body, f.token)
	if status != want {
		t.Fatalf("%s %s: %d want %d: %s", method, path, status, want, result)
	}
	return result
}

func materialCount(t *testing.T, f *httpContractFixture, table string, want int) {
	t.Helper()
	var n int
	if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != want {
		t.Fatalf("%s %d want %d: %v", table, n, want, e)
	}
}

func TestHTTPProductionMaterialsReserveReleaseConsumeAndReplay(t *testing.T) {
	f, in := materialsFixture(t)
	for _, want := range []int{201, 200} {
		requireMaterials(t, f, "POST", materialsPath, in, want)
	}
	materialCount(t, f, "stock_movements", 2)
	materialCount(t, f, "production_material_events", 1)
	materialCount(t, f, "production_material_items", 2)
	var reservation production.MaterialReservation
	b := requireMaterials(t, f, "GET", materialsPath+"/"+in.ReservationID, nil, 200)
	if e := json.Unmarshal(b, &reservation); e != nil || reservation.Status != "active" || len(reservation.Items) != 2 || reservation.Items[0].QuantityMilli != 1500000 || reservation.Items[1].QuantityMilli != 300003 {
		t.Fatalf("reservation %s %v", b, e)
	}
	capacity, e := production.Capacities(context.Background(), f.db, materialsActor(f), f.device, production.CapacityInput{LocationID: "production-room", VersionIDs: []string{"bread-v1"}})
	if e != nil || capacity.Basis != "local_available_balance_after_reservations" || capacity.Alternatives[0].PossibleBatches != 1 {
		t.Fatalf("capacity %+v %v", capacity, e)
	}
	changed := in
	changed.Reason = "Different"
	requireMaterials(t, f, "POST", materialsPath, changed, 409)
	changed = in
	changed.OperationID = "reserve-again"
	changed.ReservationID = "duplicate-group"
	requireMaterials(t, f, "POST", materialsPath, changed, 409)
	cancel := production.OrderStateInput{OperationID: "cancel", OrderID: in.OrderID, ExpectedRevision: 2, Status: "cancelled", Reason: "Cancelar"}
	requireMaterials(t, f, "POST", orderPath+"/state", cancel, 409)
	change := production.MaterialChangeInput{OperationID: "release", ReservationID: in.ReservationID, Action: "release", Reason: "Replanejar"}
	for i := 0; i < 2; i++ {
		requireMaterials(t, f, "POST", materialsPath+"/state", change, 200)
	}
	materialCount(t, f, "stock_movements", 2)
	capacity, e = production.Capacities(context.Background(), f.db, materialsActor(f), f.device, production.CapacityInput{LocationID: "production-room", VersionIDs: []string{"bread-v1"}})
	if e != nil || capacity.Alternatives[0].PossibleBatches != 4 {
		t.Fatal("release did not free capacity", e)
	}
	in.OperationID, in.ReservationID = "reserve-second", "materials-2"
	requireMaterials(t, f, "POST", materialsPath, in, 201)
	change.OperationID, change.ReservationID, change.Action = "consume", in.ReservationID, "consume"
	for i := 0; i < 2; i++ {
		requireMaterials(t, f, "POST", materialsPath+"/state", change, 200)
	}
	materialCount(t, f, "stock_movements", 4)
	materialCount(t, f, "production_material_movements", 2)
	materialCount(t, f, "production_material_events", 4)
	materialCount(t, f, "outbox", 7) // recipe + create/approve + four material events
	balance, e := stock.Balance(context.Background(), f.db, materialsActor(f), f.device, "flour", "production-room")
	if e != nil || balance != 500000 {
		t.Fatal(balance, e)
	}
	balance, e = stock.Balance(context.Background(), f.db, materialsActor(f), f.device, "oil", "production-room")
	if e != nil || balance != 100001 {
		t.Fatal(balance, e)
	}
	balance, e = stock.Balance(context.Background(), f.db, materialsActor(f), f.device, "bread", "production-room")
	if e != nil || balance != 0 {
		t.Fatal("consumption produced output", balance, e)
	}
	requireMaterials(t, f, "POST", orderPath+"/state", cancel, 409)
	change.OperationID, change.Action = "late-release", "release"
	requireMaterials(t, f, "POST", materialsPath+"/state", change, 409)
	in.OperationID, in.ReservationID = "third-reserve", "materials-3"
	requireMaterials(t, f, "POST", materialsPath, in, 409)
	// Historic reserve replay returns its original result after consumption.
	in.OperationID, in.ReservationID = "reserve-second", "materials-2"
	requireMaterials(t, f, "POST", materialsPath, in, 200)
}

func TestHTTPProductionMaterialsProtectEveryStockWithdrawal(t *testing.T) {
	f, in := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, in, 201)
	ctx, a := context.Background(), materialsActor(f)
	if _, e := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'destination','backroom','Other')`, a.TenantID, a.StoreID); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"loss", "transfer"} {
		input := stock.Input{OperationID: kind, Kind: kind, ProductID: "flour", FromLocationID: "production-room", QuantityMilli: 500001, Reason: "Teste"}
		if kind == "transfer" {
			input.ToLocationID = "destination"
		}
		if _, e := stock.Record(ctx, f.db, a, f.device, input); !errors.Is(e, stock.ErrInsufficientStock) {
			t.Fatal(kind, e)
		}
	}
	if _, e := inventory.Count(ctx, f.db, a, f.device, inventory.Input{OperationID: "count", ProductID: "flour", LocationID: "production-room", CountedMilli: 1499999}); !errors.Is(e, inventory.ErrConflict) {
		t.Fatal("count", e)
	}
	// Sales can withdraw only from shelves; create that catalog location in the
	// disposable fixture without changing the reserved product or location ID.
	if _, e := f.db.Exec(`UPDATE stock_locations SET kind='shelf' WHERE id='production-room'`); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE products SET price_cents=1000 WHERE id='flour'`); e != nil {
		t.Fatal(e)
	}
	if _, e := register.Open(ctx, f.db, a, f.device, register.OpenInput{SessionID: "cash"}); e != nil {
		t.Fatal(e)
	}
	input := sale.Input{OperationID: "sale", SaleID: "sale-1", CashSessionID: "cash", Items: []sale.Item{{ProductID: "flour", LocationID: "production-room", QuantityMilli: 500001}}, Payments: []sale.Payment{{Method: "cash", AmountCents: 500001}}}
	if _, e := sale.Complete(ctx, f.db, a, f.device, input); !errors.Is(e, sale.ErrNoStock) {
		t.Fatal("sale", e)
	}
	// Duplicate sale lines are checked cumulatively against the free quantity.
	input.Items = []sale.Item{{ProductID: "flour", LocationID: "production-room", QuantityMilli: 300000}, {ProductID: "flour", LocationID: "production-room", QuantityMilli: 300000}}
	if _, e := sale.Complete(ctx, f.db, a, f.device, input); !errors.Is(e, sale.ErrNoStock) {
		t.Fatal("aggregate sale", e)
	}
	if _, e := stock.Record(ctx, f.db, a, f.device, stock.Input{OperationID: "exact-free", Kind: "loss", ProductID: "flour", FromLocationID: "production-room", QuantityMilli: 500000, Reason: "Livre"}); e != nil {
		t.Fatal("free boundary", e)
	}
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: in.ReservationID, Action: "consume", Reason: "Executar"}, 200)
	balance, e := stock.Balance(ctx, f.db, a, f.device, "flour", "production-room")
	if e != nil || balance != 0 {
		t.Fatal("consumed other stock", balance, e)
	}
}

func TestHTTPProductionMaterialsRollbackOnIgnoredWrites(t *testing.T) {
	for _, table := range []string{"production_material_reservations", "production_material_items", "production_material_events", "outbox", "stock_movements", "production_material_movements"} {
		t.Run(table, func(t *testing.T) {
			f, in := materialsFixture(t)
			consuming := table == "stock_movements" || table == "production_material_movements"
			if consuming {
				requireMaterials(t, f, "POST", materialsPath, in, 201)
			}
			if _, e := f.db.Exec("CREATE TRIGGER ignore_material_write BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			input := any(in)
			path := materialsPath
			if consuming {
				input = production.MaterialChangeInput{OperationID: "consume", ReservationID: in.ReservationID, Action: "consume", Reason: "Executar"}
				path += "/state"
			}
			status, _ := request(t, f.app, "POST", path, orderBody(t, input), f.token)
			if status < 400 {
				t.Fatal("ignored write succeeded", status)
			}
			materialCount(t, f, "stock_movements", 2)
			materialCount(t, f, "production_material_movements", 0)
			groups, events := 0, 0
			if consuming {
				groups, events = 1, 1
			}
			materialCount(t, f, "production_material_reservations", groups)
			materialCount(t, f, "production_material_events", events)
			if consuming {
				var state string
				if e := f.db.QueryRow(`SELECT status FROM production_material_reservations`).Scan(&state); e != nil || state != "active" {
					t.Fatal(state, e)
				}
			}
		})
	}
}

func TestHTTPProductionMaterialsConcurrentOrdersCannotShareReservedStock(t *testing.T) {
	f, in := materialsFixture(t)
	order := production.OrderInput{OperationID: "order-second", OrderID: "order-2", VersionID: "bread-v1", LocationID: "production-room", ResponsibleID: f.owner.OwnerID, PlannedBatches: 3}
	requireMaterials(t, f, "POST", orderPath, order, 201)
	requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: "approve-second", OrderID: order.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Plano"}, 200)
	second := in
	second.OperationID, second.ReservationID, second.OrderID = "reserve-other", "materials-other", order.OrderID
	bodies := []string{orderBody(t, in), orderBody(t, second)}
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, body := range bodies {
		wg.Add(1)
		go func(b string) {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", materialsPath, b, f.token)
			statuses <- status
		}(body)
	}
	wg.Wait()
	close(statuses)
	success, conflict := 0, 0
	for status := range statuses {
		if status == 201 {
			success++
		} else if status == 409 {
			conflict++
		} else {
			t.Fatal(status)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	materialCount(t, f, "production_material_reservations", 1)
	materialCount(t, f, "stock_movements", 2)
}

func TestHTTPProductionMaterialsValidationAuthorizationAndUnits(t *testing.T) {
	for _, mode := range []string{"unapproved", "empty", "unit", "role", "responsible", "overflow", "foreign-order"} {
		t.Run(mode, func(t *testing.T) {
			f, in := materialsFixture(t)
			want := 409
			switch mode {
			case "unapproved":
				if _, e := f.db.Exec(`UPDATE production_orders SET status='planned'`); e != nil {
					t.Fatal(e)
				}
			case "empty":
				if _, e := f.db.Exec(`DELETE FROM stock_movements`); e != nil {
					t.Fatal(e)
				}
			case "unit":
				if _, e := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='flour'`); e != nil {
					t.Fatal(e)
				}
			case "role":
				if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
					t.Fatal(e)
				}
				want = 403
			case "responsible":
				if _, e := f.db.Exec(`INSERT INTO identities VALUES('inactive-responsible','Responsavel','now')`); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`INSERT INTO memberships VALUES(?,'inactive-responsible','production','revoked','now')`, f.owner.TenantID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`UPDATE production_orders SET responsible_id='inactive-responsible'`); e != nil {
					t.Fatal(e)
				}
				want = 403
			case "overflow":
				if _, e := f.db.Exec(`UPDATE production_orders SET planned_batches=9007199254740991`); e != nil {
					t.Fatal(e)
				}
			case "foreign-order":
				in.OrderID = "not-in-this-store"
				want = 404
			}
			requireMaterials(t, f, "POST", materialsPath, in, want)
			materialCount(t, f, "production_material_reservations", 0)
		})
	}
	f, in := materialsFixture(t)
	for _, body := range []string{`{}`, `{"operation_id":"a","operation_id":"b","reservation_id":"r","order_id":"order-1","reason":"x"}`, `{"operation_id":"a","reservation_id":null,"order_id":"order-1","reason":"x"}`, orderBody(t, in) + ` {}`} {
		status, _ := request(t, f.app, "POST", materialsPath, body, f.token)
		if status != 400 {
			t.Fatal("strict JSON", status, body)
		}
	}
	status, _ := request(t, f.app, "POST", materialsPath, orderBody(t, in), "")
	if status != 401 {
		t.Fatal("anonymous", status)
	}
	requireMaterials(t, f, "POST", materialsPath, in, 201)
	actor := materialsActor(f)
	actor.TenantID = "other-company"
	if _, e := production.GetMaterials(context.Background(), f.db, actor, f.device, in.ReservationID); e == nil {
		t.Fatal("cross-company read")
	}
	actor = materialsActor(f)
	actor.StoreID = "other-store"
	if _, e := production.GetMaterials(context.Background(), f.db, actor, f.device, in.ReservationID); e == nil {
		t.Fatal("cross-store read")
	}
	change := production.MaterialChangeInput{OperationID: "bad-action", ReservationID: in.ReservationID, Action: "finish", Reason: "Teste"}
	requireMaterials(t, f, "POST", materialsPath+"/state", change, 400)
	requireMaterials(t, f, "GET", materialsPath+"/"+in.ReservationID+"?ignored=yes", nil, 400)
}

func TestHTTPProductionMaterialsBackupRestoresActiveAndConsumedData(t *testing.T) {
	for _, consumed := range []bool{false, true} {
		name := "active"
		if consumed {
			name = "consumed"
		}
		t.Run(name, func(t *testing.T) {
			f, in := materialsFixture(t)
			requireMaterials(t, f, "POST", materialsPath, in, 201)
			if consumed {
				requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: in.ReservationID, Action: "consume", Reason: "Executar"}, 200)
			}
			ctx := context.Background()
			key := make([]byte, 32)
			if _, e := rand.Read(key); e != nil {
				t.Fatal(e)
			}
			dir := t.TempDir()
			archive := filepath.Join(dir, "materials.tytbak")
			path := filepath.Join(dir, "recovered.sqlite")
			if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
				t.Fatal(e)
			}
			if e := backup.Verify(ctx, archive, f.device, key); e != nil {
				t.Fatal(e)
			}
			if e := backup.Restore(ctx, archive, path, f.device, key); e != nil {
				t.Fatal(e)
			}
			db, e := localdb.Open(ctx, path)
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			got, e := production.GetMaterials(ctx, db, materialsActor(f), f.device, in.ReservationID)
			if e != nil || got.Status != name || len(got.Items) != 2 || got.Items[0].QuantityMilli != 1500000 || got.Items[1].QuantityMilli != 300003 {
				t.Fatal("restored reservation", got, e)
			}
			expectedEvents, expectedLinks := 1, 0
			if consumed {
				expectedEvents, expectedLinks = 2, 2
			}
			for _, v := range []struct {
				table string
				want  int
			}{{"production_material_events", expectedEvents}, {"production_material_movements", expectedLinks}} {
				var n int
				if e := db.QueryRow("SELECT count(*) FROM " + v.table).Scan(&n); e != nil || n != v.want {
					t.Fatal(v.table, n, e)
				}
			}
			if !consumed {
				_, e = stock.Record(ctx, db, materialsActor(f), f.device, stock.Input{OperationID: "restore-loss", Kind: "loss", ProductID: "flour", FromLocationID: "production-room", QuantityMilli: 500001, Reason: "Teste"})
				if !errors.Is(e, stock.ErrInsufficientStock) {
					t.Fatal("restored hold unprotected", e)
				}
			}
			var n int
			if e := db.QueryRow(`SELECT count(*) FROM stock_movements m JOIN production_material_movements p ON p.movement_id=m.id WHERE m.quantity_milli>=0`).Scan(&n); e != nil || n != 0 {
				t.Fatal("invalid restored consumption", n, e)
			}
		})
	}
}

func TestHTTPProductionMaterialsExpiredContractPreservesReads(t *testing.T) {
	f, in := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, in, 201)
	now := time.Now().Unix()
	payload, e := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); e != nil {
		t.Fatal(e)
	}
	requireMaterials(t, f, "POST", materialsPath, in, 403)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "release", ReservationID: in.ReservationID, Action: "release", Reason: "Replanejar"}, 403)
	requireMaterials(t, f, "GET", materialsPath+"/"+in.ReservationID, nil, 200)
	materialCount(t, f, "production_material_events", 1)
	// The hold remains effective until an authorized explicit release.
	status, body := request(t, f.app, "GET", materialsPath+"/"+in.ReservationID, "", f.token)
	if status != 200 || !strings.Contains(string(body), `"status":"active"`) {
		t.Fatal(status, string(body))
	}
}

func TestHTTPProductionMaterialsReleaseAllowsCancelAndStateFailureRollsBack(t *testing.T) {
	f, in := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, in, 201)
	if _, e := f.db.Exec(`CREATE TRIGGER ignore_material_status BEFORE UPDATE ON production_material_reservations BEGIN SELECT RAISE(IGNORE); END`); e != nil {
		t.Fatal(e)
	}
	change := production.MaterialChangeInput{OperationID: "consume", ReservationID: in.ReservationID, Action: "consume", Reason: "Executar"}
	status, _ := request(t, f.app, "POST", materialsPath+"/state", orderBody(t, change), f.token)
	if status < 400 {
		t.Fatal("ignored state update", status)
	}
	materialCount(t, f, "stock_movements", 2)
	materialCount(t, f, "production_material_movements", 0)
	materialCount(t, f, "production_material_events", 1)
	if _, e := f.db.Exec(`DROP TRIGGER ignore_material_status`); e != nil {
		t.Fatal(e)
	}
	change.OperationID, change.Action = "release", "release"
	requireMaterials(t, f, "POST", materialsPath+"/state", change, 200)
	requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: "cancel", OrderID: in.OrderID, ExpectedRevision: 2, Status: "cancelled", Reason: "Replanejar"}, 200)
	requireMaterials(t, f, "POST", materialsPath, production.ReserveInput{OperationID: "after-cancel", ReservationID: "new-hold", OrderID: in.OrderID, Reason: "Teste"}, 409)
	materialCount(t, f, "stock_movements", 2)
}
