package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
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

const stockReservationsPath = "/local/v1/stock/reservations"
const flourReservations = stockReservationsPath + "?product_id=flour&location_id=production-room"

func reservationPageResponse(t *testing.T, f *httpContractFixture, path string) stock.ReservationPage {
	t.Helper()
	body := requireMaterials(t, f, "GET", path, nil, 200)
	var out stock.ReservationPage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err, string(body))
	}
	return out
}
func TestHTTPStockReservationsLifecycleMetadataAndNoWrites(t *testing.T) {
	f, reserve := materialsFixture(t)
	out := reservationPageResponse(t, f, flourReservations)
	if out.TotalCount != 0 || len(out.Items) != 0 || out.Items == nil || out.Balance.PhysicalMilli != 2000000 || out.Balance.ReservedMilli != 0 || out.Balance.FreeMilli != 2000000 {
		t.Fatal(out)
	}
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	before := traceReadState(t, f)
	out = reservationPageResponse(t, f, flourReservations)
	if out.TotalCount != 1 || out.Limit != 50 || out.HasMore || len(out.Items) != 1 || out.Balance.ReservedMilli != 1500000 || out.Balance.FreeMilli != 500000 {
		t.Fatal(out)
	}
	item := out.Items[0]
	if item.ReservationID != reserve.ReservationID || item.OrderID != reserve.OrderID || item.VersionID != "bread-v1" || item.ResponsibleID != f.owner.OwnerID || item.CreatedBy != f.owner.OwnerID || item.Unit != "g" || item.QuantityMilli != 1500000 {
		t.Fatal(item)
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("read changes data/clock")
	}
	// Details are identities of existing holds, not a new stock commitment.
	availabilityResponse(t, f, flourAvailability, 2000000, 1500000, 500000)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "release", ReservationID: reserve.ReservationID, Action: "release", Reason: "Liberar"}, 200)
	out = reservationPageResponse(t, f, flourReservations)
	if out.TotalCount != 0 || out.Balance.ReservedMilli != 0 || out.Balance.PhysicalMilli != 2000000 {
		t.Fatal(out)
	}
	reserve.OperationID, reserve.ReservationID = "reserve-again", "materials-2"
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "Consumir"}, 200)
	out = reservationPageResponse(t, f, flourReservations)
	if out.TotalCount != 0 || len(out.Items) != 0 || out.Balance.PhysicalMilli != 500000 || out.Balance.ReservedMilli != 0 || out.Balance.FreeMilli != 500000 {
		t.Fatal(out)
	}
}
func TestHTTPStockReservationsPaginationTotalsAndOtherLocation(t *testing.T) {
	f, reserve := materialsFixture(t)
	if _, err := f.db.Exec(`UPDATE stock_movements SET quantity_milli=CASE product_id WHEN 'flour' THEN 40000000 ELSE 10000000 END`); err != nil {
		t.Fatal(err)
	}
	reserve.OperationID, reserve.ReservationID = "held-op-00", "held-00"
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	for i := 1; i <= 52; i++ {
		id := fmt.Sprintf("order-%02d", i)
		in := production.OrderInput{OperationID: id, OrderID: id, VersionID: "bread-v1", LocationID: "production-room", ResponsibleID: f.owner.OwnerID, PlannedBatches: 1}
		requireMaterials(t, f, "POST", orderPath, in, 201)
		requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: id + "-approve", OrderID: id, ExpectedRevision: 1, Status: "approved", Reason: "Aprovar"}, 200)
		requireMaterials(t, f, "POST", materialsPath, production.ReserveInput{OperationID: fmt.Sprintf("held-op-%02d", i), ReservationID: fmt.Sprintf("held-%02d", i), OrderID: id, Reason: "Reservar"}, 201)
	}
	if _, err := f.db.Exec(`UPDATE production_material_reservations SET created_at='2024-02-29T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	first := reservationPageResponse(t, f, flourReservations)
	second := reservationPageResponse(t, f, flourReservations+"&offset=50")
	if first.TotalCount != 53 || len(first.Items) != 50 || !first.HasMore || first.Items[0].ReservationID != "held-00" || second.TotalCount != 53 || len(second.Items) != 3 || second.HasMore || second.Items[0].ReservationID != "held-50" {
		t.Fatal(first, second)
	}
	for _, page := range []stock.ReservationPage{first, second} {
		if page.Balance.PhysicalMilli != 40000000 || page.Balance.ReservedMilli != 27500000 || page.Balance.FreeMilli != 12500000 {
			t.Fatal(page)
		}
	}
	for _, offset := range []string{"53", "100", "9007199254740991"} {
		out := reservationPageResponse(t, f, flourReservations+"&offset="+offset)
		if out.TotalCount != 53 || out.Items == nil || len(out.Items) != 0 || out.HasMore || out.Balance.ReservedMilli != 27500000 {
			t.Fatal(out)
		}
	}
	if _, err := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'other-room','production','Other')`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	out := reservationPageResponse(t, f, stockReservationsPath+"?product_id=flour&location_id=other-room")
	if out.TotalCount != 0 || out.Balance.ReservedMilli != 0 || out.Balance.PhysicalMilli != 0 {
		t.Fatal(out)
	}
}
func TestHTTPStockReservationsStrictQueryDualPermissionAndExpiredRead(t *testing.T) {
	f, reserve := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	for _, path := range []string{stockReservationsPath, stockReservationsPath + "?product_id=flour", flourReservations + "&product_id=oil", flourReservations + "&offset=-1", flourReservations + "&offset=%2B1", flourReservations + "&offset=1.5", flourReservations + "&offset=", flourReservations + "&offset=9007199254740992", flourReservations + "&offset=999999999999999999999999", flourReservations + "&offset=0&offset=1", flourReservations + "&tenant_id=other", flourReservations + "&status=active", stockReservationsPath + "?product_id=%00&location_id=production-room"} {
		requireMaterials(t, f, "GET", path, nil, 400)
	}
	requireMaterials(t, f, "GET", stockReservationsPath+"?product_id=flour&location_id=missing", nil, 404)
	status, _ := request(t, f.app, "GET", flourReservations, "", "")
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
		if _, err := stock.ActiveReservations(context.Background(), f.db, foreign, f.device, "flour", "production-room", 0); err == nil {
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
	reservationPageResponse(t, f, flourReservations)
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
		requireMaterials(t, f, "GET", flourReservations, nil, 403)
		// The P15 aggregate keeps its original manage_stock contract.
		if role == "stock" {
			requireMaterials(t, f, "GET", flourAvailability, nil, 200)
		}
	}
}
func TestHTTPStockReservationsRejectsParentStatusOrLocationConflict(t *testing.T) {
	for _, kind := range []string{"cancelled", "location", "unit"} {
		t.Run(kind, func(t *testing.T) {
			f, reserve := materialsFixture(t)
			requireMaterials(t, f, "POST", materialsPath, reserve, 201)
			switch kind {
			case "cancelled":
				if _, err := f.db.Exec(`UPDATE production_orders SET status='cancelled'`); err != nil {
					t.Fatal(err)
				}
			case "location":
				if _, err := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'other-room','production','Other')`, f.owner.TenantID, f.owner.StoreID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`UPDATE production_orders SET location_id='other-room'`); err != nil {
					t.Fatal(err)
				}
			case "unit":
				if _, err := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='flour'`); err != nil {
					t.Fatal(err)
				}
			}
			requireMaterials(t, f, "GET", flourReservations, nil, 409)
		})
	}
}
func TestHTTPStockReservationsBackupRestoresActiveOrigins(t *testing.T) {
	f, reserve := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "origins.tytbak")
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
	out, err := stock.ActiveReservations(ctx, db, materialsActor(f), f.device, "flour", "production-room", 0)
	if err != nil || out.TotalCount != 1 || out.Balance.ReservedMilli != 1500000 || out.Items[0].ReservationID != reserve.ReservationID || out.Items[0].OrderID != reserve.OrderID || out.Items[0].VersionID != "bread-v1" {
		t.Fatal(out, err)
	}
}
func TestHTTPStockReservationsConcurrentConsumeKeepsBalanceAndPageSnapshot(t *testing.T) {
	f, reserve := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	var wg sync.WaitGroup
	wg.Add(2)
	statuses := make(chan int, 2)
	go func() {
		defer wg.Done()
		status, _ := request(t, f.app, "POST", materialsPath+"/state", orderBody(t, production.MaterialChangeInput{OperationID: "consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "Consumir"}), f.token)
		statuses <- status
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			status, body := request(t, f.app, "GET", flourReservations, "", f.token)
			var out stock.ReservationPage
			if status != 200 || json.Unmarshal(body, &out) != nil || out.TotalCount != int64(len(out.Items)) {
				statuses <- 500
				return
			}
			if !(out.TotalCount == 1 && out.Balance.PhysicalMilli == 2000000 && out.Balance.ReservedMilli == 1500000 && out.Items[0].QuantityMilli == 1500000) && !(out.TotalCount == 0 && out.Balance.PhysicalMilli == 500000 && out.Balance.ReservedMilli == 0) {
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
