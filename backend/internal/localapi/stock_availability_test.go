package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
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
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/stock"
)

const availabilityPath = "/local/v1/stock/availability"
const flourAvailability = availabilityPath + "?product_id=flour&location_id=production-room"

func availabilityResponse(t *testing.T, f *httpContractFixture, path string, physical, reserved, free int64) stock.Availability {
	t.Helper()
	body := requireMaterials(t, f, "GET", path, nil, 200)
	var out stock.Availability
	if err := json.Unmarshal(body, &out); err != nil || out.PhysicalMilli != physical || out.ReservedMilli != reserved || out.FreeMilli != free || out.PhysicalMilli != out.ReservedMilli+out.FreeMilli {
		t.Fatal(out, err, string(body))
	}
	return out
}
func TestHTTPStockAvailabilityReserveReleaseConsumeAndNoWrites(t *testing.T) {
	f, reserve := materialsFixture(t)
	before := traceReadState(t, f)
	out := availabilityResponse(t, f, flourAvailability, 2000000, 0, 2000000)
	if out.Unit != "g" || out.ProductID != "flour" || out.LocationID != "production-room" || !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal(out, "read mutation")
	}
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	before = traceReadState(t, f)
	availabilityResponse(t, f, flourAvailability, 2000000, 1500000, 500000)
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("reserved read mutation")
	}
	body := requireMaterials(t, f, "GET", "/local/v1/stock/balance?product_id=flour&location_id=production-room", nil, 200)
	var legacy struct {
		QuantityMilli int64 `json:"quantity_milli"`
	}
	if err := json.Unmarshal(body, &legacy); err != nil || legacy.QuantityMilli != 2000000 {
		t.Fatal(legacy, err)
	}
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "release", ReservationID: reserve.ReservationID, Action: "release", Reason: "Liberar"}, 200)
	availabilityResponse(t, f, flourAvailability, 2000000, 0, 2000000)
	reserve.OperationID, reserve.ReservationID = "reserve-again", "materials-2"
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "Consumir"}, 200)
	availabilityResponse(t, f, flourAvailability, 500000, 0, 500000)
	availabilityResponse(t, f, availabilityPath+"?product_id=oil&location_id=production-room", 100001, 0, 100001)
}
func TestHTTPStockAvailabilityOtherLocationReservationAndZeroBalance(t *testing.T) {
	f, reserve := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	if _, err := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'other-room','production','Other')`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	availabilityResponse(t, f, availabilityPath+"?product_id=flour&location_id=other-room", 0, 0, 0)
	in := production.OrderInput{OperationID: "other-order", OrderID: "other-order", VersionID: "bread-v1", LocationID: "other-room", ResponsibleID: f.owner.OwnerID, PlannedBatches: 1}
	requireMaterials(t, f, "POST", orderPath, in, 201)
	requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: "other-approve", OrderID: in.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Aprovar"}, 200)
	addCapacityMovement(t, f, "other-flour", "flour", "other-room", 1000000)
	addCapacityMovement(t, f, "other-oil", "oil", "other-room", 200002)
	requireMaterials(t, f, "POST", materialsPath, production.ReserveInput{OperationID: "other-reserve", ReservationID: "other-materials", OrderID: in.OrderID, Reason: "Reservar"}, 201)
	availabilityResponse(t, f, flourAvailability, 2000000, 1500000, 500000)
	availabilityResponse(t, f, availabilityPath+"?product_id=flour&location_id=other-room", 1000000, 500000, 500000)
}
func TestHTTPStockAvailabilityRejectsInconsistentBalanceAndKnownUnitChanges(t *testing.T) {
	for _, kind := range []string{"negative", "too-large", "overcommitted", "active-unit", "consumed-unit", "result-unit"} {
		t.Run(kind, func(t *testing.T) {
			f, reserve := materialsFixture(t)
			path := flourAvailability
			switch kind {
			case "negative":
				if _, err := f.db.Exec(`UPDATE stock_movements SET quantity_milli=-1 WHERE product_id='flour'`); err != nil {
					t.Fatal(err)
				}
			case "too-large":
				if _, err := f.db.Exec(`UPDATE stock_movements SET quantity_milli=? WHERE product_id='flour'`, stock.MaxAvailabilityQuantity+1); err != nil {
					t.Fatal(err)
				}
			case "overcommitted":
				requireMaterials(t, f, "POST", materialsPath, reserve, 201)
				if _, err := f.db.Exec(`UPDATE stock_movements SET quantity_milli=1 WHERE product_id='flour'`); err != nil {
					t.Fatal(err)
				}
			case "active-unit", "consumed-unit":
				requireMaterials(t, f, "POST", materialsPath, reserve, 201)
				if kind == "consumed-unit" {
					requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "Consumir"}, 200)
				}
				if _, err := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='flour'`); err != nil {
					t.Fatal(err)
				}
			case "result-unit":
				f, _, _ = traceFixture(t)
				path = availabilityPath + "?product_id=bread&location_id=production-room"
				if _, err := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='bread'`); err != nil {
					t.Fatal(err)
				}
			}
			requireMaterials(t, f, "GET", path, nil, 409)
		})
	}
}
func TestHTTPStockAvailabilityQueryAuthorizationAndExpiredRead(t *testing.T) {
	f, _ := materialsFixture(t)
	for _, query := range []string{"", "?product_id=flour", "?location_id=production-room", "?product_id=&location_id=production-room", "?product_id=flour&product_id=oil&location_id=production-room", "?product_id=flour&location_id=production-room&location_id=other", "?product_id=%00&location_id=production-room", "?product_id=%20flour&location_id=production-room", "?product_id=" + strings.Repeat("x", 129) + "&location_id=production-room", "?product_id=flour&location_id=production-room&tenant_id=other", "?product_id=flour&location_id=production-room&offset=0"} {
		requireMaterials(t, f, "GET", availabilityPath+query, nil, 400)
	}
	for _, query := range []string{"?product_id=missing&location_id=production-room", "?product_id=flour&location_id=missing", "?product_id=" + url.QueryEscape("x' OR 1=1 --") + "&location_id=production-room"} {
		requireMaterials(t, f, "GET", availabilityPath+query, nil, 404)
	}
	status, _ := request(t, f.app, "GET", flourAvailability, "", "")
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
		if _, err := stock.AvailableBalance(context.Background(), f.db, foreign, f.device, "flour", "production-room"); err == nil {
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
	availabilityResponse(t, f, flourAvailability, 2000000, 0, 2000000)
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("clock mutation")
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"cashier", "stock", "production"} {
		if _, err = f.db.Exec(`UPDATE memberships SET role=?`, role); err != nil {
			t.Fatal(err)
		}
		want := 403
		if role == "stock" {
			want = 200
		}
		requireMaterials(t, f, "GET", flourAvailability, nil, want)
	}
}
func TestHTTPStockAvailabilityBackupRestoresPhysicalAndActiveReservations(t *testing.T) {
	f, reserve := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "availability.tytbak")
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
	out, err := stock.AvailableBalance(ctx, db, materialsActor(f), f.device, "flour", "production-room")
	if err != nil || out.Unit != "g" || out.PhysicalMilli != 2000000 || out.ReservedMilli != 1500000 || out.FreeMilli != 500000 {
		t.Fatal(out, err)
	}
}
func TestHTTPStockAvailabilityConcurrentReserveConsumeKeepsOneSnapshot(t *testing.T) {
	f, reserve := materialsFixture(t)
	var wg sync.WaitGroup
	wg.Add(2)
	statuses := make(chan int, 2)
	go func() {
		defer wg.Done()
		status, _ := request(t, f.app, "POST", materialsPath, orderBody(t, reserve), f.token)
		if status != 201 {
			statuses <- status
			return
		}
		status, _ = request(t, f.app, "POST", materialsPath+"/state", orderBody(t, production.MaterialChangeInput{OperationID: "consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "Consumir"}), f.token)
		statuses <- status
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			status, body := request(t, f.app, "GET", flourAvailability, "", f.token)
			var out stock.Availability
			if status != 200 || json.Unmarshal(body, &out) != nil || out.PhysicalMilli != out.ReservedMilli+out.FreeMilli {
				statuses <- 500
				return
			}
			if !((out.PhysicalMilli == 2000000 && out.ReservedMilli == 0 && out.FreeMilli == 2000000) || (out.PhysicalMilli == 2000000 && out.ReservedMilli == 1500000 && out.FreeMilli == 500000) || (out.PhysicalMilli == 500000 && out.ReservedMilli == 0 && out.FreeMilli == 500000)) {
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
	availabilityResponse(t, f, flourAvailability, 500000, 0, 500000)
}
