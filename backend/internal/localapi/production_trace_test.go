package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/production"
)

func traceResponse(t *testing.T, f *httpContractFixture, id string, offset int) production.OrderTrace {
	t.Helper()
	body := requireMaterials(t, f, "GET", orderPath+"/"+id+fmt.Sprintf("/trace?lot_offset=%d", offset), nil, 200)
	var out production.OrderTrace
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(string(body), err)
	}
	return out
}
func traceReadState(t *testing.T, f *httpContractFixture) []int64 {
	t.Helper()
	out := []int64{}
	for _, table := range []string{"stock_movements", "outbox", "production_order_events", "production_material_events", "production_results", "production_losses", "production_lots", "production_quality_reviews"} {
		var n int64
		if err := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	var clock int64
	if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&clock); err != nil {
		t.Fatal(err)
	}
	return append(out, clock)
}
func traceFixture(t *testing.T) (*httpContractFixture, production.StagePlanInput, production.LotInput) {
	t.Helper()
	f, stages, reserve := stagesFixture(t)
	requireMaterials(t, f, "POST", stagePlansPath, stages, 201)
	consumeStages(t, f, reserve)
	finishStages(t, f, stages)
	result := stageCompletion(stages)
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	requireMaterials(t, f, "POST", lossesPath, production.LossInput{OperationID: "loss", LossID: "loss-1", ResultID: result.ResultID, QuantityMilli: 2000, Reason: "Classificar diferenca"}, 201)
	lot := production.LotInput{OperationID: "lot", LotID: "lot-1", ResultID: result.ResultID, QuantityMilli: 20000, Code: "BREAD-A", ManufacturedOn: "2024-02-29", ExpiresOn: "", Reason: "Identificar produto"}
	requireMaterials(t, f, "POST", lotsPath, lot, 201)
	requireMaterials(t, f, "POST", qualityPath, production.QualityInput{OperationID: "quality", LotID: lot.LotID, ExpectedRevision: 0, Status: "failed", Criterion: "Criterio informado", Reason: "Parecer humano"}, 201)
	return f, stages, lot
}
func TestHTTPProductionTracePlannedReservedReleasedAndConsumed(t *testing.T) {
	f, order := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, order, 201)
	out := traceResponse(t, f, order.OrderID, 0)
	if out.Order.Status != "planned" || out.Order.PlannedOutputMilli != 30000 || out.Stages != nil || out.Result != nil || out.Losses != nil || out.Lots != nil || out.Materials.Current != nil || len(out.Ingredients) != 2 {
		t.Fatal(out)
	}
	quantities := map[string]int64{"flour": 1500000, "oil": 300003}
	for _, v := range out.Ingredients {
		if v.PlannedMilli != quantities[v.ProductID] || v.ReservedMilli != 0 || v.ConsumedMilli != 0 {
			t.Fatal(v)
		}
	}
	requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: "approve", OrderID: order.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Plano"}, 200)
	addCapacityMovement(t, f, "flour-stock", "flour", "production-room", 2000000)
	addCapacityMovement(t, f, "oil-stock", "oil", "production-room", 400004)
	reserve := production.ReserveInput{OperationID: "reserve", ReservationID: "materials-1", OrderID: order.OrderID, Reason: "Separar"}
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	out = traceResponse(t, f, order.OrderID, 0)
	if out.Materials.ActiveCount != 1 || out.Materials.ConsumedCount != 0 || out.Materials.Current.Status != "active" {
		t.Fatal(out.Materials)
	}
	for _, v := range out.Ingredients {
		if v.ReservedMilli != v.PlannedMilli || v.ConsumedMilli != 0 {
			t.Fatal(v)
		}
	}
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "release", ReservationID: reserve.ReservationID, Action: "release", Reason: "Replanejar"}, 200)
	out = traceResponse(t, f, order.OrderID, 0)
	if out.Materials.ReleasedCount != 1 || out.Materials.Current != nil {
		t.Fatal(out.Materials)
	}
	reserve.OperationID, reserve.ReservationID = "reserve-again", "materials-2"
	consumeStages(t, f, reserve)
	out = traceResponse(t, f, order.OrderID, 0)
	if out.Materials.ReleasedCount != 1 || out.Materials.ConsumedCount != 1 || out.Materials.ActiveCount != 0 || out.Result != nil {
		t.Fatal(out)
	}
	for _, v := range out.Ingredients {
		if v.ConsumedMilli != v.PlannedMilli || v.ReservedMilli != 0 {
			t.Fatal(v)
		}
	}
}
func TestHTTPProductionTraceCompletedStagesLossesLotsQualityAndNoWrites(t *testing.T) {
	f, stages, lot := traceFixture(t)
	before := traceReadState(t, f)
	out := traceResponse(t, f, stages.OrderID, 0)
	if out.Order.Status != "completed" || out.Order.Recipe.VersionID != out.Order.VersionID || out.Stages == nil || len(out.Stages.Items) != 2 || out.Stages.Items[1].Status != "completed" || out.Result == nil || out.Result.ProducedMilli != 27000 || out.Result.ShortfallMilli != 3000 || out.Result.ActorID != f.owner.OwnerID || out.Losses.RecordedMilli != 2000 || out.Losses.UnclassifiedMilli != 1000 {
		t.Fatal(out)
	}
	if out.Lots == nil || out.Lots.TotalCount != 1 || out.Lots.AssignedMilli != 20000 || out.Lots.UnassignedMilli != 7000 || out.Lots.HasMore || len(out.Lots.Items) != 1 || out.Lots.Items[0].Quality.Status != "failed" || out.Lots.Items[0].Quality.Latest.LotSnapshot.Code != lot.Code {
		t.Fatal(out.Lots)
	}
	after := traceReadState(t, f)
	for i, v := range before {
		if after[i] != v {
			t.Fatal("read mutated data or contract clock", before, after)
		}
	}
	unchangedProductionAfterLoss(t, f)
	requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: "void", LotID: lot.LotID, ExpectedRevision: 1, Reason: "Corrigir"}, 200)
	out = traceResponse(t, f, stages.OrderID, 0)
	if out.Lots.AssignedMilli != 0 || out.Lots.UnassignedMilli != 27000 || out.Lots.Items[0].Lot.Status != "voided" || out.Lots.Items[0].Quality.LotStatus != "voided" || out.Lots.Items[0].Quality.Status != "failed" || out.Lots.Items[0].Quality.Latest.LotSnapshot.Status != "recorded" {
		t.Fatal(out.Lots)
	}
	// Catalog changes do not rewrite the recipe, output or historical lot units.
	if _, err := f.db.Exec(`UPDATE products SET name='Changed',unit='kg' WHERE id='bread'`); err != nil {
		t.Fatal(err)
	}
	out = traceResponse(t, f, stages.OrderID, 0)
	if out.Order.Recipe.OutputUnit != "unit" || out.Result.Unit != "unit" || out.Lots.Items[0].Lot.Unit != "unit" {
		t.Fatal(out)
	}
}
func TestHTTPProductionTracePaginationUsesGlobalTotals(t *testing.T) {
	f, result := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	for i := 0; i < 27; i++ {
		lot := production.LotInput{OperationID: fmt.Sprint("op", i), LotID: fmt.Sprint("lot", i), ResultID: result.ResultID, QuantityMilli: 1000, Code: fmt.Sprint("CODE", i), ManufacturedOn: "2024-02-29", ExpiresOn: "", Reason: "Identificar"}
		requireMaterials(t, f, "POST", lotsPath, lot, 201)
	}
	// Recorded + voided rows both page, while only recorded quantities contribute.
	for i := 0; i < 26; i++ {
		requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: fmt.Sprint("void", i), LotID: fmt.Sprint("lot", i), ExpectedRevision: 1, Reason: "Corrigir"}, 200)
		lot := production.LotInput{OperationID: fmt.Sprint("replace", i), LotID: fmt.Sprint("replacement", i), ResultID: result.ResultID, QuantityMilli: 1000, Code: fmt.Sprint("NEW", i), ManufacturedOn: "2024-02-29", ExpiresOn: "", Reason: "Identificar"}
		requireMaterials(t, f, "POST", lotsPath, lot, 201)
	}
	for _, offset := range []int{0, 50, 53, 100} {
		out := traceResponse(t, f, result.OrderID, offset)
		if out.Lots.TotalCount != 53 || out.Lots.AssignedMilli != 27000 || out.Lots.UnassignedMilli != 0 || out.Lots.Offset != int64(offset) {
			t.Fatal(out.Lots)
		}
		want := 0
		if offset == 0 {
			want = 50
		} else if offset == 50 {
			want = 3
		}
		if len(out.Lots.Items) != want || out.Lots.HasMore != (offset == 0) {
			t.Fatal(out.Lots)
		}
		for _, v := range out.Lots.Items {
			if v.Quality.Status != "not_assessed" || v.Quality.Latest != nil {
				t.Fatal(v)
			}
		}
	}
}
func TestHTTPProductionTraceAuthorizationScopeQueryAndInconsistency(t *testing.T) {
	f, stages, _ := traceFixture(t)
	path := orderPath + "/" + stages.OrderID + "/trace"
	for _, query := range []string{"?lot_offset=-1", "?offset=0", "?lot_offset=0&lot_offset=1", "?lot_offset=9007199254740992", "?lot_offset=0.1"} {
		requireMaterials(t, f, "GET", path+query, nil, 400)
	}
	requireMaterials(t, f, "GET", orderPath+"/outside/trace", nil, 404)
	status, _ := request(t, f.app, "GET", path, "", "")
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
		if _, err := production.GetOrderTrace(context.Background(), f.db, foreign, f.device, stages.OrderID, 0); err == nil {
			t.Fatal("isolation")
		}
	}
	// Historical read is authorized without invoking the write contract/clock.
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	traceResponse(t, f, stages.OrderID, 0)
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", path, nil, 403)
	f, stages, _ = traceFixture(t)
	// Deliberate inconsistent frozen quantity only in a disposable fixture.
	if _, err = f.db.Exec(`UPDATE production_orders SET planned_output_milli=1`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", orderPath+"/"+stages.OrderID+"/trace", nil, 409)
	f, stages, _ = traceFixture(t)
	if _, err = f.db.Exec(`UPDATE stock_movements SET quantity_milli=-1 WHERE id IN (SELECT movement_id FROM production_material_movements)`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", orderPath+"/"+stages.OrderID+"/trace", nil, 409)
}
func TestHTTPProductionTraceBackupRestoresCompleteChain(t *testing.T) {
	f, stages, lot := traceFixture(t)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "trace.tytbak")
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
	out, err := production.GetOrderTrace(ctx, db, materialsActor(f), f.device, stages.OrderID, 0)
	if err != nil || out.Order.Status != "completed" || out.Materials.ConsumedCount != 1 || len(out.Ingredients) != 2 || out.Stages.Items[1].Status != "completed" || out.Result.ProducedMilli != 27000 || out.Losses.RecordedMilli != 2000 || len(out.Lots.Items) != 1 || out.Lots.Items[0].Lot.Code != lot.Code || out.Lots.Items[0].Quality.Status != "failed" {
		t.Fatal(out, err)
	}
}
func TestHTTPProductionTraceConcurrentAssessmentUsesOneSnapshot(t *testing.T) {
	f, stages, lot := traceFixture(t)
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 8; i++ {
			verdict := "passed"
			if i%2 == 0 {
				verdict = "failed"
			}
			input := production.QualityInput{OperationID: fmt.Sprint("review", i), LotID: lot.LotID, ExpectedRevision: int64(i), Status: verdict, Criterion: "Criterio", Reason: "Reavaliacao"}
			status, _ := request(t, f.app, "POST", qualityPath, orderBody(t, input), f.token)
			if status != 201 {
				statuses <- status
				return
			}
		}
		statuses <- 200
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 16; i++ {
			status, body := request(t, f.app, "GET", orderPath+"/"+stages.OrderID+"/trace", "", f.token)
			if status != 200 {
				statuses <- status
				return
			}
			var out production.OrderTrace
			if err := json.Unmarshal(body, &out); err != nil {
				statuses <- 500
				return
			}
			q := out.Lots.Items[0].Quality
			if q.Latest == nil || q.Revision != q.Latest.Revision || q.Status != q.Latest.Status || q.LotStatus != out.Lots.Items[0].Lot.Status || out.Lots.AssignedMilli != 20000 {
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
	materialCount(t, f, "production_quality_reviews", 9)
	unchangedProductionAfterLoss(t, f)
}
