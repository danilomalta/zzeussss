package localapi

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/supplycycle"
)

func cycleRejectInput() supplycycle.FullRejectionInput {
	return supplycycle.FullRejectionInput{OperationID: "reject-trip", ExpectedRevision: 3, DeliveredMilli: 3000, EvidenceReference: "registro-descarga-qa", ObservedAt: time.Now().UTC().Format(time.RFC3339), Reason: "Carga totalmente avariada"}
}
func rejectionCounts(t *testing.T, f *httpContractFixture) []int64 {
	t.Helper()
	out := cycleCounts(t, f)
	for _, table := range []string{"purchase_delivery_rejections", "purchase_receiving_exception_events", "purchase_delivery_full_rejections"} {
		var n int64
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
			t.Fatal(e)
		}
		out = append(out, n)
	}
	return out
}
func TestSupplyCycleFullRefusalClosesReleasesAndRecoversWithoutStockOrMoney(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	in := cycleRejectInput()
	before := rejectionCounts(t, f)
	first := cycleCall(t, f, "POST", cycleDelivery+"/reject", in, 200)
	if first.Delivery.State != "rejected" || first.Delivery.Revision != 4 || first.Delivery.ReceiptID != "" || first.Rejection == nil || first.RejectionEvidence.RecordedBy != f.owner.OwnerID || first.RejectionEvidence.RecordedDeviceID != f.owner.DeviceID || first.Receipt != nil || first.Payable != nil {
		t.Fatal(first)
	}
	domainSchemaFile(t, "purchase-supply-cycle.openapi.json", "DeliveryRejection", refusalJSON(t, first.Rejection))
	domainSchemaFile(t, "purchase-supply-cycle.openapi.json", "FullRejectionEvidence", refusalJSON(t, first.RejectionEvidence))
	for _, i := range []int{5, 6, 7, 8, 9, 10, 11} {
		if rejectionCounts(t, f)[i] != before[i] {
			t.Fatal("unexpected commercial effect", i)
		}
	}
	after := rejectionCounts(t, f)
	retry := cycleCall(t, f, "POST", cycleDelivery+"/reject", in, 200)
	if !retry.Repeated {
		t.Fatal(retry)
	}
	retry.Repeated = false
	recovered := cycleCall(t, f, "GET", "/local/v1/purchase-cycle-operations/reject-trip", nil, 200)
	if !reflect.DeepEqual(first, retry) || !reflect.DeepEqual(first, recovered) || !reflect.DeepEqual(after, rejectionCounts(t, f)) {
		t.Fatal("changed original or repeated effects")
	}
	raw := agendaCall(t, f, "GET", cycleDelivery, nil, 200)
	var got supplycycle.Delivery
	json.Unmarshal(raw, &got)
	if !reflect.DeepEqual(&got, first.Delivery) {
		t.Fatal(string(raw))
	}
	var cycle supplycycle.Cycle
	json.Unmarshal(agendaCall(t, f, "GET", cycleOrder+"/supply-cycle", nil, 200), &cycle)
	if cycle.AllocatedMilli != 0 || cycle.EffectiveAcceptedMilli != 0 || cycle.RemainingOrderMilli != 5000 {
		t.Fatal(cycle)
	}
	var history supplycycle.History
	json.Unmarshal(agendaCall(t, f, "GET", cycleDelivery+"/history", nil, 200), &history)
	if history.Total != 4 || len(history.Items) != 4 || !reflect.DeepEqual(history.Items[3], first) {
		t.Fatal(history)
	}
	cycleCall(t, f, "POST", cycleDelivery+"/receive", cycleReceiveInput(), 409)
	finance := cycleFinanceInput()
	finance.ExpectedRevision = 4
	cycleCall(t, f, "POST", cycleDelivery+"/payable", finance, 409)
	cycleCall(t, f, "POST", cycleDelivery+"/actions", supplycycle.ActionInput{OperationID: "cancel-rejected", ExpectedRevision: 4, Action: "cancel", EvidenceReference: "qa", ObservedAt: in.ObservedAt, Reason: "Cannot disguise refusal"}, 409)
	changed := in
	changed.Reason = "Outro motivo"
	cycleCall(t, f, "POST", cycleDelivery+"/reject", changed, 409)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-replacement", 5000), 200)
	// A backup must preserve the old physical snapshot plus its checked terminal ledger.
	db := restoredPurchaseDB(t, f)
	scope := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	device := identity.DeviceContext{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, DeviceID: f.owner.DeviceID}
	restored, e := supplycycle.GetDelivery(context.Background(), db, scope, device, "trip-one")
	if e != nil || !reflect.DeepEqual(restored, *first.Delivery) {
		t.Fatal(restored, e)
	}
	for _, q := range []string{`DELETE FROM purchase_delivery_full_rejections`, `UPDATE purchase_delivery_full_rejections SET base_revision=2`, `UPDATE purchase_deliveries SET state='received',revision=4 WHERE id='trip-one'`} {
		if _, e := f.db.Exec(q); e == nil {
			t.Fatal("mutable terminal ledger", q)
		}
	}
}
func refusalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestSupplyCycleFullRefusalExplicitLegacyBindingPreservesOriginalOperator(t *testing.T) {
	f, reviewer := cycleFixture(t)
	cycleAtArrival(t, f)
	body := map[string]any{"operation_id": "legacy-trip-refusal", "delivery_reference": "external-trip-one", "unit": "unit", "delivered_milli": 2000, "reason": "Recusa registrada antes do encerramento"}
	var legacy purchases.RejectionResult
	if e := json.Unmarshal(requireMaterials(t, f, "POST", rejectionPath, body, 201), &legacy); e != nil {
		t.Fatal(e)
	}
	in := cycleRejectInput()
	in.DeliveredMilli = 2000
	before := rejectionCounts(t, f)
	cycleCall(t, f, "POST", cycleDelivery+"/reject", in, 409)
	if !reflect.DeepEqual(before, rejectionCounts(t, f)) {
		t.Fatal("implicit binding changed effects")
	}
	in.ExistingRejectionID = legacy.Rejection.ID
	raw := refusalJSON(t, in)
	code, b := request(t, f.app, "POST", cycleDelivery+"/reject", string(raw), reviewer)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	var result supplycycle.Result
	json.Unmarshal(b, &result)
	if !reflect.DeepEqual(result.Rejection, &legacy.Rejection) || result.RejectionEvidence.RecordedBy != "cycle-reviewer" {
		t.Fatal(string(b))
	}
	after := rejectionCounts(t, f)
	for _, i := range []int{8, 9, 10, 11, 12, 13, 14} {
		if after[i] != before[i] {
			t.Fatal("replayed legacy physical effect", i)
		}
	}
	// Recovery is restricted to the new operation's original actor, not the legacy author.
	cycleCall(t, f, "GET", "/local/v1/purchase-cycle-operations/reject-trip", nil, 404)
	code, b = request(t, f.app, "GET", "/local/v1/purchase-cycle-operations/reject-trip", "", reviewer)
	if code != 200 {
		t.Fatal(code, string(b))
	}
}
func TestSupplyCycleFullRefusalRollbackEveryLinkedWrite(t *testing.T) {
	for _, table := range []string{"purchase_delivery_rejections", "purchase_receiving_exception_events", "outbox", "purchase_delivery_full_rejections", "purchase_cycle_operations", "purchase_cycle_events"} {
		t.Run(table, func(t *testing.T) {
			f, _ := cycleFixture(t)
			cycleAtArrival(t, f)
			before := rejectionCounts(t, f)
			if _, e := f.db.Exec("CREATE TRIGGER refusal_fault BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			cycleCall(t, f, "POST", cycleDelivery+"/reject", cycleRejectInput(), 409)
			if !reflect.DeepEqual(before, rejectionCounts(t, f)) {
				t.Fatal("partial refusal", table)
			}
		})
	}
}
func TestSupplyCycleFullRefusalInputRevisionScopeAndPermissions(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	in := cycleRejectInput()
	before := rejectionCounts(t, f)
	for _, change := range []func(*supplycycle.FullRejectionInput){
		func(v *supplycycle.FullRejectionInput) { v.DeliveredMilli = 0 }, func(v *supplycycle.FullRejectionInput) { v.Reason = "bad\nreason" }, func(v *supplycycle.FullRejectionInput) { v.ObservedAt = time.Now().Add(time.Hour).Format(time.RFC3339) }, func(v *supplycycle.FullRejectionInput) { v.EvidenceReference = "" },
	} {
		v := in
		change(&v)
		cycleCall(t, f, "POST", cycleDelivery+"/reject", v, 400)
	}
	for _, change := range []func(*supplycycle.FullRejectionInput){func(v *supplycycle.FullRejectionInput) { v.ExpectedRevision = 2 }, func(v *supplycycle.FullRejectionInput) { v.DeliveredMilli = 3001 }} {
		v := in
		change(&v)
		cycleCall(t, f, "POST", cycleDelivery+"/reject", v, 409)
	}
	foreign := in
	foreign.ExistingRejectionID = "absent"
	cycleCall(t, f, "POST", cycleDelivery+"/reject", foreign, 404)
	extra := map[string]any{}
	json.Unmarshal(refusalJSON(t, in), &extra)
	extra["accepted_milli"] = 0
	agendaCall(t, f, "POST", cycleDelivery+"/reject", extra, 400)
	agendaCall(t, f, "POST", cycleDelivery+"/reject?offset=0", in, 400)
	token := financeOnlyToken(t, f, "view_accounting", "manage_cash")
	code, b := request(t, f.app, "POST", cycleDelivery+"/reject", string(refusalJSON(t, in)), token)
	if code != 403 {
		t.Fatal(code, string(b))
	}
	if !reflect.DeepEqual(before, rejectionCounts(t, f)) {
		t.Fatal("rejected request wrote data")
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 100, []modules.ID{modules.Core, modules.Inventory}), 200)
	cycleCall(t, f, "POST", cycleDelivery+"/reject", in, 403)
}
func TestSupplyCycleFullRefusalReceiveRaceHasOnlyOneEffect(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for path, in := range map[string]any{"reject": cycleRejectInput(), "receive": cycleReceiveInput()} {
		wg.Add(1)
		go func(path string, in any) {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", cycleDelivery+"/"+path, string(refusalJSON(t, in)), f.token)
			codes <- code
		}(path, in)
	}
	wg.Wait()
	close(codes)
	seen := map[int]int{}
	for code := range codes {
		seen[code]++
	}
	if seen[200] != 1 || seen[409] != 1 {
		t.Fatal(seen)
	}
	var receipts, refusals int
	f.db.QueryRow(`SELECT count(*) FROM purchase_receipts`).Scan(&receipts)
	f.db.QueryRow(`SELECT count(*) FROM purchase_delivery_rejections`).Scan(&refusals)
	if receipts+refusals != 1 {
		t.Fatal(receipts, refusals)
	}
}
func TestSupplyCycleFullRefusalLegacyMismatchCannotClose(t *testing.T) {
	for _, change := range []string{"reference", "quantity"} {
		t.Run(change, func(t *testing.T) {
			f, _ := cycleFixture(t)
			cycleAtArrival(t, f)
			in := cycleRejectInput()
			body := map[string]any{"operation_id": "legacy-refusal", "delivery_reference": "other-reference", "unit": "unit", "delivered_milli": 2000, "reason": "Avaria"}
			if change == "quantity" {
				body["delivery_reference"] = "external-trip-one"
			}
			var prior purchases.RejectionResult
			json.Unmarshal(requireMaterials(t, f, "POST", rejectionPath, body, 201), &prior)
			in.ExistingRejectionID = prior.Rejection.ID
			before := rejectionCounts(t, f)
			cycleCall(t, f, "POST", cycleDelivery+"/reject", in, 409)
			if !reflect.DeepEqual(before, rejectionCounts(t, f)) {
				t.Fatal("mismatched source wrote")
			}
		})
	}
}

func TestSupplyCycleFullRefusalBlocksPairedSupplierReturn(t *testing.T) {
	f, _ := returnFixture(t)
	approve := returnAction("approve", 1)
	approve.QuantityMilli = 1000
	approve.FinancialInstruction = "replacement_expected"
	returnCall(t, f, returnsPath+"/return-one/actions", approve, 200)
	cycleCall(t, f, "POST", cycleOrder+"/deliveries", cyclePlanInput("trip-two", 1000), 200)
	schedule := returnAction("schedule", 2)
	schedule.CollectionMode = "next_delivery"
	schedule.PairedDeliveryID = "trip-two"
	schedule.Deadline = "2026-12-01T00:00:00Z"
	returnCall(t, f, returnsPath+"/return-one/actions", schedule, 200)
	for i, action := range []string{"load", "arrive"} {
		cycleCall(t, f, "POST", "/local/v1/purchase-deliveries/trip-two/actions", supplycycle.ActionInput{OperationID: "second-" + action, ExpectedRevision: int64(i + 1), Action: action, EvidenceReference: "manual", ObservedAt: time.Now().UTC().Format(time.RFC3339), Reason: "Conferido"}, 200)
	}
	refusal := cycleRejectInput()
	refusal.DeliveredMilli = 1000
	cycleCall(t, f, "POST", "/local/v1/purchase-deliveries/trip-two/reject", refusal, 200)
	before := returnCounts(t, f)
	returnCall(t, f, returnsPath+"/return-one/dispatch", returnStockInput("dispatch-rejected-trip", 3, 1000), 409)
	if !reflect.DeepEqual(before, returnCounts(t, f)) {
		t.Fatal("rejected trip dispatched stock")
	}
	// Scheduling another return onto the closed trip is also blocked.
}

func TestSupplyCycleFullRefusalRequiresEveryPermissionAndOriginalContext(t *testing.T) {
	for _, missing := range []string{"view_orders", "manage_replenishment", "manage_stock"} {
		t.Run(missing, func(t *testing.T) {
			f, _ := cycleFixture(t)
			cycleAtArrival(t, f)
			permissions := []string{}
			for _, p := range []string{"view_orders", "manage_replenishment", "manage_stock"} {
				if p != missing {
					permissions = append(permissions, p)
				}
			}
			token := financeOnlyToken(t, f, permissions...)
			before := rejectionCounts(t, f)
			code, b := request(t, f.app, "POST", cycleDelivery+"/reject", string(refusalJSON(t, cycleRejectInput())), token)
			if code != 403 || !reflect.DeepEqual(before, rejectionCounts(t, f)) {
				t.Fatal(code, string(b))
			}
		})
	}
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	original := cycleCall(t, f, "POST", cycleDelivery+"/reject", cycleRejectInput(), 200)
	scope := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	device := identity.DeviceContext{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, DeviceID: f.owner.DeviceID}
	for _, field := range []string{"tenant", "store", "actor", "device"} {
		a, d := scope, device
		switch field {
		case "tenant":
			a.TenantID = "foreign"
			d.TenantID = "foreign"
		case "store":
			a.StoreID = "foreign"
			d.StoreID = "foreign"
		case "actor":
			a.IdentityID = "cycle-reviewer"
		case "device":
			d.DeviceID = "foreign"
		}
		if _, e := supplycycle.Operation(context.Background(), f.db, a, d, original.OperationID); e == nil {
			t.Fatal("foreign recovery accepted", field)
		}
	}
}

func TestSupplyCycleFullRefusalDamagedLegacyEvidenceBlocksCurrentReadButPreservesReceipt(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	first := cycleCall(t, f, "POST", cycleDelivery+"/reject", cycleRejectInput(), 200)
	if _, e := f.db.Exec(`UPDATE purchase_delivery_rejections SET reason='tampered' WHERE id=?`, first.Rejection.ID); e != nil {
		t.Fatal(e)
	}
	before := rejectionCounts(t, f)
	agendaCall(t, f, "GET", cycleDelivery, nil, 409)
	agendaCall(t, f, "GET", cycleOrder+"/supply-cycle", nil, 409)
	original := cycleCall(t, f, "GET", "/local/v1/purchase-cycle-operations/reject-trip", nil, 200)
	if !reflect.DeepEqual(first, original) || !reflect.DeepEqual(before, rejectionCounts(t, f)) {
		t.Fatal("lost original or read wrote data")
	}
}
