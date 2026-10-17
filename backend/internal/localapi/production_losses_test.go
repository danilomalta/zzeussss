package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
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

const lossesPath = "/local/v1/production/losses"

func lossesFixture(t *testing.T) (*httpContractFixture, production.LossInput) {
	t.Helper()
	f, result := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	return f, production.LossInput{OperationID: "record-loss", LossID: "loss-1", ResultID: result.ResultID, QuantityMilli: 2000, Reason: "Duas unidades rejeitadas na conferencia"}
}

func lossSummary(t *testing.T, f *httpContractFixture, id string, recorded, unclassified int64) production.ResultLosses {
	t.Helper()
	body := requireMaterials(t, f, "GET", resultsPath+"/"+id+"/losses", nil, 200)
	var out production.ResultLosses
	if err := json.Unmarshal(body, &out); err != nil || out.RecordedLossMilli != recorded || out.UnclassifiedShortfallMilli != unclassified {
		t.Fatalf("summary %s %v", body, err)
	}
	return out
}

func unchangedProductionAfterLoss(t *testing.T, f *httpContractFixture) {
	t.Helper()
	ctx := context.Background()
	actor := materialsActor(f)
	materialCount(t, f, "stock_movements", 5)
	materialCount(t, f, "production_material_movements", 2)
	for _, v := range []struct {
		product string
		want    int64
	}{{"bread", 27000}, {"flour", 500000}, {"oil", 100001}} {
		got, err := stock.Balance(ctx, f.db, actor, f.device, v.product, "production-room")
		if err != nil || got != v.want {
			t.Fatal(v.product, got, err)
		}
	}
	result, err := production.GetProductionResult(ctx, f.db, actor, f.device, "result-1")
	if err != nil || result.ProducedMilli != 27000 || result.PlannedMilli != 30000 || result.ShortfallMilli != 3000 {
		t.Fatal(result, err)
	}
	order, err := production.GetOrder(ctx, f.db, actor, f.device, "order-1")
	if err != nil || order.Status != "completed" || order.Revision != 3 {
		t.Fatal(order, err)
	}
}

func TestHTTPProductionLossesRecordVoidHistoryAndNoDoubleStockChange(t *testing.T) {
	f, in := lossesFixture(t)
	summary := lossSummary(t, f, in.ResultID, 0, 3000)
	if len(summary.Items) != 0 || summary.ProducedMilli != 27000 || summary.Unit != "unit" {
		t.Fatal(summary)
	}
	for _, want := range []int{201, 200} {
		requireMaterials(t, f, "POST", lossesPath, in, want)
	}
	summary = lossSummary(t, f, in.ResultID, 2000, 1000)
	if len(summary.Items) != 1 || summary.Items[0].QuantityMilli != 2000 || summary.Items[0].Status != "recorded" || summary.Items[0].CreatedBy != f.owner.OwnerID {
		t.Fatal(summary)
	}
	materialCount(t, f, "production_loss_events", 1)
	materialCount(t, f, "outbox", 7)
	changed := in
	changed.Reason = "Different"
	requireMaterials(t, f, "POST", lossesPath, changed, 409)
	changed = in
	changed.OperationID = "reused-id"
	requireMaterials(t, f, "POST", lossesPath, changed, 409)
	changed.LossID = "loss-over-budget"
	requireMaterials(t, f, "POST", lossesPath, changed, 409)
	second := in
	second.OperationID, second.LossID, second.QuantityMilli = "second-op", "loss-2", 1000
	requireMaterials(t, f, "POST", lossesPath, second, 201)
	lossSummary(t, f, in.ResultID, 3000, 0)
	void := production.VoidLossInput{OperationID: "void-loss", LossID: in.LossID, ExpectedRevision: 2, Reason: "Corrigir classificacao"}
	requireMaterials(t, f, "POST", lossesPath+"/void", void, 409)
	void.ExpectedRevision = 1
	for i := 0; i < 2; i++ {
		requireMaterials(t, f, "POST", lossesPath+"/void", void, 200)
	}
	summary = lossSummary(t, f, in.ResultID, 1000, 2000)
	if len(summary.Items) != 2 || summary.Items[0].Status != "voided" || summary.Items[0].Revision != 2 {
		t.Fatal(summary)
	}
	var got production.ProductionLoss
	body := requireMaterials(t, f, "GET", lossesPath+"/"+in.LossID, nil, 200)
	if err := json.Unmarshal(body, &got); err != nil || got.Status != "voided" || got.Reason != in.Reason {
		t.Fatal(string(body), err)
	}
	history, err := production.ProductionLossHistory(context.Background(), f.db, materialsActor(f), f.device, in.LossID)
	if err != nil || len(history) != 2 || history[0].Kind != "recorded" || history[1].Kind != "voided" || history[1].Reason != void.Reason || history[1].ActorID != f.owner.OwnerID {
		t.Fatal(history, err)
	}
	requireMaterials(t, f, "GET", lossesPath+"/"+in.LossID+"/history", nil, 200)
	void.OperationID = "void-again"
	requireMaterials(t, f, "POST", lossesPath+"/void", void, 409)
	// Replay is the original operation result, while GET shows current status.
	body = requireMaterials(t, f, "POST", lossesPath, in, 200)
	var replay production.LossResult
	if err = json.Unmarshal(body, &replay); err != nil || !replay.Repeated || replay.Status != "recorded" {
		t.Fatal(string(body), err)
	}
	changed = in
	changed.OperationID, changed.LossID = "correct-op", "corrected-loss"
	requireMaterials(t, f, "POST", lossesPath, changed, 201)
	lossSummary(t, f, in.ResultID, 3000, 0)
	requireMaterials(t, f, "POST", lossesPath, production.LossInput{OperationID: "above", LossID: "above", ResultID: in.ResultID, QuantityMilli: 1000, Reason: "Teste"}, 409)
	materialCount(t, f, "production_loss_events", 4)
	materialCount(t, f, "outbox", 10)
	unchangedProductionAfterLoss(t, f)
}

func TestHTTPProductionLossesValidationScopeAndAuthorization(t *testing.T) {
	for _, kind := range []string{"zero", "negative", "fraction", "overflow", "above-shortfall", "foreign-result", "empty-reason", "role", "module"} {
		t.Run(kind, func(t *testing.T) {
			f, in := lossesFixture(t)
			want := 400
			switch kind {
			case "zero":
				in.QuantityMilli = 0
			case "negative":
				in.QuantityMilli = -1
			case "fraction":
				in.QuantityMilli = 1001
			case "overflow":
				in.QuantityMilli = production.MaxQuantity + 1
			case "above-shortfall":
				in.QuantityMilli = 4000
				want = 409
			case "foreign-result":
				in.ResultID = "outside-store"
				want = 404
			case "empty-reason":
				in.Reason = "  "
			case "role":
				if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
					t.Fatal(err)
				}
				want = 403
			case "module":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory}), 200)
				want = 403
			}
			requireMaterials(t, f, "POST", lossesPath, in, want)
			materialCount(t, f, "production_losses", 0)
			materialCount(t, f, "stock_movements", 5)
		})
	}
	f, in := lossesFixture(t)
	for _, body := range []string{`{}`, strings.Replace(orderBody(t, in), `"quantity_milli":2000`, `"quantity_milli":2000,"quantity_milli":2000`, 1), strings.Replace(orderBody(t, in), `"quantity_milli":2000`, `"quantity_milli":null`, 1), strings.Replace(orderBody(t, in), `"quantity_milli":2000`, `"quantity_milli":2000.1`, 1), strings.TrimSuffix(orderBody(t, in), "}") + `,"tenant_id":"other"}`} {
		status, _ := request(t, f.app, "POST", lossesPath, body, f.token)
		if status != 400 {
			t.Fatal("strict JSON", status, body)
		}
	}
	status, _ := request(t, f.app, "POST", lossesPath, orderBody(t, in), "")
	if status != 401 {
		t.Fatal(status)
	}
	requireMaterials(t, f, "POST", lossesPath, in, 201)
	requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID+"/losses?offset=-1", nil, 400)
	requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID+"/losses?offset=0&offset=1", nil, 400)
	requireMaterials(t, f, "GET", lossesPath+"/"+in.LossID+"?unexpected=1", nil, 400)
	requireMaterials(t, f, "POST", lossesPath+"/void", production.VoidLossInput{OperationID: "void", LossID: "foreign", ExpectedRevision: 1, Reason: "Teste"}, 404)
	actor := materialsActor(f)
	for _, company := range []bool{true, false} {
		foreign := actor
		if company {
			foreign.TenantID = "other"
		} else {
			foreign.StoreID = "other"
		}
		if _, err := production.GetProductionLoss(context.Background(), f.db, foreign, f.device, in.LossID); err == nil {
			t.Fatal("loss isolation")
		}
		if _, err := production.ListProductionLosses(context.Background(), f.db, foreign, f.device, in.ResultID, 0); err == nil {
			t.Fatal("summary isolation")
		}
		if _, err := production.ProductionLossHistory(context.Background(), f.db, foreign, f.device, in.LossID); err == nil {
			t.Fatal("audit isolation")
		}
	}
}

func TestHTTPProductionLossesNoShortfallAndHistoricUnit(t *testing.T) {
	f, result := resultsFixture(t)
	result.ProducedMilli = 30000
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	requireMaterials(t, f, "POST", lossesPath, production.LossInput{OperationID: "no-gap", LossID: "no-gap", ResultID: result.ResultID, QuantityMilli: 1000, Reason: "Teste"}, 409)
	lossSummary(t, f, result.ResultID, 0, 0)
	f, in := lossesFixture(t)
	// No stock is changed: a historic declaration retains the result unit even
	// if the catalog has subsequently changed. It never reinterprets balances.
	if _, err := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='bread'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", lossesPath, in, 201)
	summary := lossSummary(t, f, in.ResultID, 2000, 1000)
	if summary.Unit != "unit" || summary.Items[0].Unit != "unit" {
		t.Fatal(summary)
	}
	materialCount(t, f, "stock_movements", 5)
}

func TestHTTPProductionLossesWriteFailureRollsBack(t *testing.T) {
	for _, voiding := range []bool{false, true} {
		for _, table := range []string{"production_losses", "production_loss_events", "outbox"} {
			for _, failure := range []string{"IGNORE", "ABORT,'test'"} {
				t.Run(fmt.Sprintf("%t-%s-%s", voiding, table, failure), func(t *testing.T) {
					f, in := lossesFixture(t)
					if voiding {
						requireMaterials(t, f, "POST", lossesPath, in, 201)
					}
					operation := "INSERT"
					if voiding && table == "production_losses" {
						operation = "UPDATE"
					}
					if _, err := f.db.Exec("CREATE TRIGGER fail_loss BEFORE " + operation + " ON " + table + " BEGIN SELECT RAISE(" + failure + "); END"); err != nil {
						t.Fatal(err)
					}
					if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=1`); err != nil {
						t.Fatal(err)
					}
					var input any = in
					path := lossesPath
					if voiding {
						input = production.VoidLossInput{OperationID: "void", LossID: in.LossID, ExpectedRevision: 1, Reason: "Corrigir"}
						path += "/void"
					}
					status, _ := request(t, f.app, "POST", path, orderBody(t, input), f.token)
					if status < 400 {
						t.Fatal(status)
					}
					count := 0
					if voiding {
						count = 1
					}
					materialCount(t, f, "production_losses", count)
					materialCount(t, f, "production_loss_events", count)
					materialCount(t, f, "outbox", 6+count)
					var clock int64
					if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&clock); err != nil || clock != 1 {
						t.Fatal("clock rollback", clock, err)
					}
					if voiding {
						got, err := production.GetProductionLoss(context.Background(), f.db, materialsActor(f), f.device, in.LossID)
						if err != nil || got.Status != "recorded" || got.Revision != 1 {
							t.Fatal(got, err)
						}
					}
					unchangedProductionAfterLoss(t, f)
				})
			}
		}
	}
}

func TestHTTPProductionLossesConcurrentDeclarationsRespectShortfall(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			f, in := lossesFixture(t)
			other := in
			if !same {
				other.OperationID, other.LossID = "second", "second-loss"
			}
			bodies := []string{orderBody(t, in), orderBody(t, other)}
			statuses := make(chan int, 2)
			var wg sync.WaitGroup
			for _, body := range bodies {
				wg.Add(1)
				go func(b string) {
					defer wg.Done()
					status, _ := request(t, f.app, "POST", lossesPath, b, f.token)
					statuses <- status
				}(body)
			}
			wg.Wait()
			close(statuses)
			created, otherCount := 0, 0
			for status := range statuses {
				if status == 201 {
					created++
				} else if (same && status == 200) || (!same && status == 409) {
					otherCount++
				} else {
					t.Fatal(status)
				}
			}
			if created != 1 || otherCount != 1 {
				t.Fatal(created, otherCount)
			}
			materialCount(t, f, "production_losses", 1)
			lossSummary(t, f, in.ResultID, 2000, 1000)
			unchangedProductionAfterLoss(t, f)
		})
	}
}

func TestHTTPProductionLossesConcurrentVoidsAreNotDuplicated(t *testing.T) {
	f, in := lossesFixture(t)
	requireMaterials(t, f, "POST", lossesPath, in, 201)
	first := production.VoidLossInput{OperationID: "void-one", LossID: in.LossID, ExpectedRevision: 1, Reason: "Corrigir"}
	second := first
	second.OperationID = "void-two"
	bodies := []string{orderBody(t, first), orderBody(t, second)}
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, body := range bodies {
		wg.Add(1)
		go func(b string) {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", lossesPath+"/void", b, f.token)
			statuses <- status
		}(body)
	}
	wg.Wait()
	close(statuses)
	success, conflict := 0, 0
	for status := range statuses {
		if status == 200 {
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
	lossSummary(t, f, in.ResultID, 0, 3000)
	materialCount(t, f, "production_loss_events", 2)
	unchangedProductionAfterLoss(t, f)
}

func TestHTTPProductionLossesBackupRestoresRecordedAndVoidedAudit(t *testing.T) {
	f, in := lossesFixture(t)
	requireMaterials(t, f, "POST", lossesPath, in, 201)
	requireMaterials(t, f, "POST", lossesPath+"/void", production.VoidLossInput{OperationID: "void", LossID: in.LossID, ExpectedRevision: 1, Reason: "Corrigir"}, 200)
	replacement := in
	replacement.OperationID, replacement.LossID, replacement.QuantityMilli = "replace", "replacement", 1000
	requireMaterials(t, f, "POST", lossesPath, replacement, 201)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "losses.tytbak")
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
	summary, err := production.ListProductionLosses(ctx, db, actor, f.device, in.ResultID, 0)
	if err != nil || summary.RecordedLossMilli != 1000 || summary.UnclassifiedShortfallMilli != 2000 || len(summary.Items) != 2 || summary.Items[0].Status != "voided" || summary.Items[1].Status != "recorded" {
		t.Fatal(summary, err)
	}
	history, err := production.ProductionLossHistory(ctx, db, actor, f.device, in.LossID)
	if err != nil || len(history) != 2 || history[1].Reason != "Corrigir" {
		t.Fatal(history, err)
	}
	balance, err := stock.Balance(ctx, db, actor, f.device, "bread", "production-room")
	if err != nil || balance != 27000 {
		t.Fatal(balance, err)
	}
	order, err := production.GetOrder(ctx, db, actor, f.device, "order-1")
	if err != nil || order.Status != "completed" || order.Revision != 3 {
		t.Fatal(order, err)
	}
}

func TestHTTPProductionLossesExpiredContractAndRevokedRole(t *testing.T) {
	for _, expired := range []bool{true, false} {
		t.Run(fmt.Sprint(expired), func(t *testing.T) {
			f, in := lossesFixture(t)
			requireMaterials(t, f, "POST", lossesPath, in, 201)
			if expired {
				now := time.Now().Unix()
				payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
					t.Fatal(err)
				}
			}
			requireMaterials(t, f, "POST", lossesPath, in, 403)
			requireMaterials(t, f, "POST", lossesPath+"/void", production.VoidLossInput{OperationID: "void", LossID: in.LossID, ExpectedRevision: 1, Reason: "Corrigir"}, 403)
			readStatus := 403
			if expired {
				readStatus = 200
			}
			requireMaterials(t, f, "GET", lossesPath+"/"+in.LossID, nil, readStatus)
			requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID+"/losses", nil, readStatus)
			materialCount(t, f, "production_loss_events", 1)
			materialCount(t, f, "stock_movements", 5)
		})
	}
}

func TestHTTPProductionLossesAggregateOverflowFailsClosed(t *testing.T) {
	f, in := lossesFixture(t)
	// Deliberately inconsistent data only in this disposable fixture: the
	// aggregate exceeds int64 even though each row fits the public integer limit.
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1025; i++ {
		_, err = tx.Exec(`INSERT INTO production_losses VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, f.owner.TenantID, f.owner.StoreID, fmt.Sprintf("bad-%d", i), in.ResultID, "bread", "unit", production.MaxQuantity, "Corrupt fixture", "recorded", 1, f.owner.OwnerID, "now", "now")
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", lossesPath, in, 409)
	requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID+"/losses", nil, 409)
	materialCount(t, f, "production_loss_events", 0)
	materialCount(t, f, "stock_movements", 5)
}

func TestHTTPProductionLossesExactMassAndPaginationKeepsGlobalTotals(t *testing.T) {
	f, recipe := recipeFixture(t)
	if _, err := f.db.Exec(`UPDATE products SET unit='g' WHERE id='bread'`); err != nil {
		t.Fatal(err)
	}
	recipe.OutputUnit = "g"
	requireMaterials(t, f, "POST", recipePath, recipe, 201)
	if _, err := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'production-room','production','Production')`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	order := production.OrderInput{OperationID: "order-op", OrderID: "order-1", VersionID: recipe.VersionID, LocationID: "production-room", ResponsibleID: f.owner.OwnerID, PlannedBatches: 3}
	requireMaterials(t, f, "POST", orderPath, order, 201)
	requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: "approve", OrderID: order.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Plano"}, 200)
	addCapacityMovement(t, f, "flour-stock", "flour", "production-room", 2000000)
	addCapacityMovement(t, f, "oil-stock", "oil", "production-room", 400004)
	requireMaterials(t, f, "POST", materialsPath, production.ReserveInput{OperationID: "reserve", ReservationID: "materials-1", OrderID: order.OrderID, Reason: "Separar"}, 201)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: "materials-1", Action: "consume", Reason: "Executar"}, 200)
	requireMaterials(t, f, "POST", resultsPath, production.ResultInput{OperationID: "complete", ResultID: "result-1", OrderID: order.OrderID, ExpectedRevision: 2, ProducedMilli: 27000, Reason: "Resultado medido"}, 201)
	for i := 0; i < 52; i++ {
		input := production.LossInput{OperationID: fmt.Sprintf("op-%02d", i), LossID: fmt.Sprintf("loss-%02d", i), ResultID: "result-1", QuantityMilli: 1, Reason: "0,001 g declarado"}
		requireMaterials(t, f, "POST", lossesPath, input, 201)
	}
	summary := lossSummary(t, f, "result-1", 52, 2948)
	if summary.Unit != "g" || len(summary.Items) != 50 || summary.Items[0].QuantityMilli != 1 {
		t.Fatal(summary)
	}
	body := requireMaterials(t, f, "GET", resultsPath+"/result-1/losses?offset=50", nil, 200)
	if err := json.Unmarshal(body, &summary); err != nil || len(summary.Items) != 2 || summary.RecordedLossMilli != 52 || summary.UnclassifiedShortfallMilli != 2948 {
		t.Fatal(string(body), err)
	}
	body = requireMaterials(t, f, "GET", resultsPath+"/result-1/losses?offset=52", nil, 200)
	if err := json.Unmarshal(body, &summary); err != nil || len(summary.Items) != 0 || summary.RecordedLossMilli != 52 {
		t.Fatal(string(body), err)
	}
	materialCount(t, f, "stock_movements", 5)
}
