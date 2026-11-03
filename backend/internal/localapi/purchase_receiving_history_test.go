package localapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/replenishment"
)

const receivingHistoryPath = "/local/v1/purchase-orders/order/receiving-history"

func receivingHistoryResponse(t *testing.T, f *httpContractFixture, path string) purchases.ReceivingHistoryPage {
	t.Helper()
	b := requireMaterials(t, f, "GET", path, nil, 200)
	var out purchases.ReceivingHistoryPage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestHTTPPurchaseReceivingHistoryExactStringsPaginationAndBackup(t *testing.T) {
	f := receivingFixture(t)
	// Rejections can accumulate beyond JavaScript's safe integer range. Preserve decimals.
	for i := 0; i < 2; i++ {
		in := rejectionBody()
		in["operation_id"] = fmt.Sprintf("reject-max-%d", i)
		in["delivery_reference"] = fmt.Sprintf("reject-max-%d", i)
		in["delivered_milli"] = purchases.MaxQuantity
		requireMaterials(t, f, "POST", rejectionPath, in, 201)
	}
	original := receivedResponse(t, f, receivingBody(), 201)
	requireMaterials(t, f, "POST", receiptVoidPath(original.Receipt.ID), receiptVoidBody(), 200)
	next := receivingBody()
	next["operation_id"] = "corrected"
	next["delivery_reference"] = "corrected"
	receivedResponse(t, f, next, 201)
	for i := 0; i < 47; i++ {
		in := rejectionBody()
		in["operation_id"] = fmt.Sprintf("reject-small-%d", i)
		in["delivery_reference"] = fmt.Sprintf("reject-small-%d", i)
		in["delivered_milli"] = 1
		requireMaterials(t, f, "POST", rejectionPath, in, 201)
	}
	before := traceReadState(t, f)
	out := receivingHistoryResponse(t, f, receivingHistoryPath)
	if out.TotalCount != 52 || len(out.Items) != 50 || !out.HasMore || out.Totals.FullyRejectedMilliExact != "18014398509482029" || out.Totals.RecordedAcceptedMilliExact != "4000" || out.Totals.VoidedAcceptedMilliExact != "2000" || out.Totals.EffectiveAcceptedMilli != 2000 || out.Totals.RemainingMilli != 3000 {
		t.Fatal(out)
	}
	last := receivingHistoryResponse(t, f, receivingHistoryPath+"?offset=50")
	if len(last.Items) != 2 || last.HasMore || !reflect.DeepEqual(out.Totals, last.Totals) {
		t.Fatal(last)
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("history writes")
	}
	for _, q := range []string{"?offset=", "?offset=-1", "?offset=0&offset=0", "?tenant_id=other", "?offset=9007199254740992"} {
		requireMaterials(t, f, "GET", receivingHistoryPath+q, nil, 400)
	}
	db := restoredPurchaseDB(t, f)
	after, err := purchases.ReceivingHistory(context.Background(), db, materialsActor(f), f.device, "order", 0)
	if err != nil || !reflect.DeepEqual(out, after) {
		t.Fatal(after, err)
	}
	// Validate evidence outside the selected page, rather than returning a partial success.
	if _, err = f.db.Exec(`UPDATE purchase_delivery_rejections SET reason='broken' WHERE operation_id='reject-max-0'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", receivingHistoryPath+"?offset=50", nil, 409)
}
func productionReceivingCorrectionFixture(t *testing.T) (*httpContractFixture, production.ReserveInput, purchases.ReceiptResult) {
	t.Helper()
	f, reserve := materialsFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory, modules.Production, modules.Orders}), 200)
	addCapacityMovement(t, f, "lower-flour", "flour", "production-room", -501000)
	a := materialsActor(f)
	ctx := context.Background()
	if _, err := replenishment.SetPolicy(ctx, f.db, a, f.device, replenishment.PolicyInput{OperationID: "policy-correction", ProductID: "flour", MinimumMilli: 2500000, TargetMilli: 3000000}); err != nil {
		t.Fatal(err)
	}
	s, err := replenishment.Suggest(ctx, f.db, a, f.device, replenishment.SuggestInput{OperationID: "suggest-correction", ProductID: "flour"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = replenishment.Review(ctx, f.db, a, f.device, replenishment.ReviewInput{OperationID: "review-correction", SuggestionID: s.SuggestionID, Decision: "approved", Reason: "comprar"}); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", "/local/v1/purchase-suppliers", map[string]any{"operation_id": "supplier-correction", "id": "supplier", "name": "Farinha", "status": "active"}, 201)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", purchases.Input{OperationID: "purchase-correction", OrderID: "order", SupplierID: "supplier", SuggestionID: s.SuggestionID}, 201)
	requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 200)
	in := receivingBody()
	in["unit"] = "g"
	in["location_id"] = "production-room"
	in["delivered_milli"] = 2000
	receipt := receivedResponse(t, f, in, 201)
	return f, reserve, receipt
}
func TestHTTPPurchaseReceivingHistoryVoidProtectsProductionReservationAndRestoresCycle(t *testing.T) {
	f, reserve, receipt := productionReceivingCorrectionFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	availabilityResponse(t, f, flourAvailability, 1501000, 1500000, 1000)
	requireMaterials(t, f, "POST", receiptVoidPath(receipt.Receipt.ID), receiptVoidBody(), 409)
	materialCount(t, f, "purchase_receipt_voids", 0)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "release-correction", ReservationID: reserve.ReservationID, Action: "release", Reason: "corrigir lancamento"}, 200)
	requireMaterials(t, f, "POST", receiptVoidPath(receipt.Receipt.ID), receiptVoidBody(), 200)
	availabilityResponse(t, f, flourAvailability, 1499000, 0, 1499000)
	before := receivingHistoryResponse(t, f, receivingHistoryPath)
	db := restoredPurchaseDB(t, f)
	after, err := purchases.ReceivingHistory(context.Background(), db, materialsActor(f), f.device, "order", 0)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal(after, err)
	}
}
func TestHTTPPurchaseReceivingHistoryReservationVersusVoidIsAtomic(t *testing.T) {
	f, reserve, receipt := productionReceivingCorrectionFixture(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		status, body := request(t, f.app, "POST", materialsPath, orderBody(t, reserve), f.token)
		if status != 201 && status != 409 {
			t.Errorf("reserve %d %s", status, body)
		}
		statuses <- status
	}()
	go func() {
		defer wg.Done()
		status, body := request(t, f.app, "POST", receiptVoidPath(receipt.Receipt.ID), orderBody(t, receiptVoidBody()), f.token)
		if status != 200 && status != 409 {
			t.Errorf("void %d %s", status, body)
		}
		statuses <- status
	}()
	wg.Wait()
	close(statuses)
	success, conflict := 0, 0
	for s := range statuses {
		if s == 200 || s == 201 {
			success++
		} else if s == 409 {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	out := reservationPageResponse(t, f, flourReservations)
	if out.Balance.PhysicalMilli < out.Balance.ReservedMilli || out.Balance.FreeMilli < 0 {
		t.Fatal(out)
	}
	receivingHistoryResponse(t, f, receivingHistoryPath)
}
