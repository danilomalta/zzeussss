package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/replenishment"
)

const receivingTracePath = "/local/v1/purchase-orders/order/receiving-trace"

func receivingTraceResponse(t *testing.T, f *httpContractFixture, path string) purchases.ReceivingTracePage {
	t.Helper()
	b := requireMaterials(t, f, "GET", path, nil, 200)
	var out purchases.ReceivingTracePage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestHTTPPurchaseReceivingTracePaginationExactTotalsExpiryAndScope(t *testing.T) {
	f := receivingFixture(t)
	empty := receivingTraceResponse(t, f, receivingTracePath)
	if empty.AcceptedMilli != 0 || empty.RemainingMilli != 5000 || empty.Items == nil || empty.Authorization == nil {
		t.Fatal(empty)
	}
	for i := 0; i < 51; i++ {
		in := receivingBody()
		in["operation_id"] = fmt.Sprintf("page-%02d", i)
		in["delivery_reference"] = fmt.Sprintf("page-%02d", i)
		in["accepted_milli"] = 1
		in["delivered_milli"] = 2
		receivedResponse(t, f, in, 201)
	}
	before := traceReadState(t, f)
	page := receivingTraceResponse(t, f, receivingTracePath)
	if page.TotalCount != 51 || len(page.Items) != 50 || !page.HasMore || page.AcceptedMilli != 51 || page.DeliveredMilli != 102 || page.RejectedMilli != 51 || page.RemainingMilli != 4949 || page.CommercialStatus != "local_not_sent" {
		t.Fatal(page)
	}
	last := receivingTraceResponse(t, f, receivingTracePath+"?offset=50")
	if len(last.Items) != 1 || last.HasMore || last.AcceptedMilli != 51 {
		t.Fatal(last)
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("read writes")
	}
	for _, query := range []string{"?offset=", "?offset=-1", "?offset=1&offset=1", "?offset=9007199254740992", "?offset=0.5", "?tenant_id=other"} {
		requireMaterials(t, f, "GET", receivingTracePath+query, nil, 400)
	}
	requireMaterials(t, f, "GET", "/local/v1/purchase-orders/missing/receiving-trace", nil, 404)
	for _, kind := range []string{"tenant", "store", "actor", "device"} {
		a := materialsActor(f)
		d := f.device
		switch kind {
		case "tenant":
			a.TenantID = "other"
		case "store":
			a.StoreID = "other"
		case "actor":
			a.IdentityID = "other"
		case "device":
			d.DeviceID = "other"
		}
		if _, err := purchases.ReceivingTrace(context.Background(), f.db, a, d, "order", 0); err == nil {
			t.Fatal(kind)
		}
	}
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Orders}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	receivingTraceResponse(t, f, receivingTracePath)
	requireMaterials(t, f, "POST", receivingPath, receivingBody(), 403)
	db := restoredPurchaseDB(t, f)
	restored, err := purchases.ReceivingTrace(context.Background(), db, materialsActor(f), f.device, "order", 0)
	if err != nil || !reflect.DeepEqual(page, restored) {
		t.Fatal(restored, err)
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", receivingTracePath, nil, 403)
}
func TestHTTPPurchaseReceivingTraceRejectsBrokenEvidence(t *testing.T) {
	for _, damage := range []string{`DELETE FROM purchase_receiving_events WHERE kind='received'`, `UPDATE stock_movements SET reason='broken'`, `UPDATE purchase_receipts SET accepted_milli=1999`, `UPDATE stock_operations SET quantity_milli=1999`, `DELETE FROM stock_operation_movements`, `UPDATE purchase_receiving_authorizations SET reason='broken'`} {
		t.Run(damage, func(t *testing.T) {
			f := receivingFixture(t)
			receivedResponse(t, f, receivingBody(), 201)
			if _, err := f.db.Exec(damage); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "GET", receivingTracePath+"?offset=50", nil, 409)
			requireMaterials(t, f, "POST", receivingPath, receivingBody(), 409)
		})
	}
}
func TestHTTPPurchaseReceivingTraceWithProductionReservationsConsumptionAndBackup(t *testing.T) {
	f, reserve := materialsFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory, modules.Production, modules.Orders}), 200)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	a := materialsActor(f)
	ctx := context.Background()
	if _, err := replenishment.SetPolicy(ctx, f.db, a, f.device, replenishment.PolicyInput{OperationID: "receiving-policy", ProductID: "flour", MinimumMilli: 2500000, TargetMilli: 3000000}); err != nil {
		t.Fatal(err)
	}
	suggestion, err := replenishment.Suggest(ctx, f.db, a, f.device, replenishment.SuggestInput{OperationID: "receiving-suggestion", ProductID: "flour"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = replenishment.Review(ctx, f.db, a, f.device, replenishment.ReviewInput{OperationID: "receiving-review", SuggestionID: suggestion.SuggestionID, Decision: "approved", Reason: "comprar ingrediente"}); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", "/local/v1/purchase-suppliers", map[string]any{"operation_id": "receiving-supplier", "id": "supplier", "name": "Farinha", "status": "active"}, 201)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", purchases.Input{OperationID: "buy-flour", OrderID: "order", SupplierID: "supplier", SuggestionID: suggestion.SuggestionID}, 201)
	requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 200)
	in := receivingBody()
	in["location_id"] = "production-room"
	in["unit"] = "g"
	in["accepted_milli"] = 1000
	in["delivered_milli"] = 1100
	receivedResponse(t, f, in, 201)
	availabilityResponse(t, f, flourAvailability, 2001000, 1500000, 501000)
	next := receivingBody()
	next["operation_id"] = "concurrent-receipt"
	next["delivery_reference"] = "concurrent-delivery"
	next["location_id"] = "production-room"
	next["unit"] = "g"
	next["accepted_milli"] = 1000
	next["delivered_milli"] = 1000
	consume := production.MaterialChangeInput{OperationID: "receive-consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "consumir ingredientes"}
	var wg sync.WaitGroup
	for _, action := range []func(){func() { receivedResponse(t, f, next, 201) }, func() { requireMaterials(t, f, "POST", materialsPath+"/state", consume, 200) }} {
		wg.Add(1)
		go func(fn func()) { defer wg.Done(); fn() }(action)
	}
	wg.Wait()
	availabilityResponse(t, f, flourAvailability, 502000, 0, 502000)
	trace := receivingTraceResponse(t, f, receivingTracePath)
	if trace.AcceptedMilli != 2000 || trace.RejectedMilli != 100 {
		t.Fatal(trace)
	}
	db := restoredPurchaseDB(t, f)
	out, err := purchases.ReceivingTrace(ctx, db, a, f.device, "order", 0)
	if err != nil || !reflect.DeepEqual(trace, out) {
		t.Fatal(out, err)
	}
	var balance int64
	var status string
	if err = db.QueryRow(`SELECT SUM(quantity_milli) FROM stock_movements WHERE product_id='flour'`).Scan(&balance); err != nil || balance != 502000 {
		t.Fatal(balance, err)
	}
	if err = db.QueryRow(`SELECT status FROM production_material_reservations WHERE id=?`, reserve.ReservationID).Scan(&status); err != nil || status != "consumed" {
		t.Fatal(status, err)
	}
}
