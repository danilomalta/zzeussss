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

const lotsPath = "/local/v1/production/lots"

func lotsFixture(t *testing.T) (*httpContractFixture, production.LotInput) {
	t.Helper()
	f, result := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	return f, production.LotInput{OperationID: "record-lot", LotID: "lot-1", ResultID: result.ResultID, QuantityMilli: 20000, Code: "BREAD-A", ManufacturedOn: "2024-02-29", ExpiresOn: "2024-03-01", Reason: "Identificar vinte unidades produzidas"}
}
func lotSummary(t *testing.T, f *httpContractFixture, id string, assigned, unassigned int64) production.ResultLots {
	t.Helper()
	body := requireMaterials(t, f, "GET", resultsPath+"/"+id+"/lots", nil, 200)
	var out production.ResultLots
	if err := json.Unmarshal(body, &out); err != nil || out.AssignedMilli != assigned || out.UnassignedMilli != unassigned {
		t.Fatalf("%s %v", body, err)
	}
	return out
}
func TestHTTPProductionLotsRecordVoidAuditReplayAndNoStockChange(t *testing.T) {
	f, in := lotsFixture(t)
	summary := lotSummary(t, f, in.ResultID, 0, 27000)
	if len(summary.Items) != 0 || summary.ProducedMilli != 27000 || summary.Unit != "unit" {
		t.Fatal(summary)
	}
	for _, want := range []int{201, 200} {
		requireMaterials(t, f, "POST", lotsPath, in, want)
	}
	summary = lotSummary(t, f, in.ResultID, 20000, 7000)
	if len(summary.Items) != 1 || summary.Items[0].Code != in.Code || summary.Items[0].ManufacturedOn != in.ManufacturedOn || summary.Items[0].ExpiresOn != in.ExpiresOn || summary.Items[0].CreatedBy != f.owner.OwnerID {
		t.Fatal(summary)
	}
	// Dates are declarations. An elapsed expiry never withdraws physical stock.
	unchangedProductionAfterLoss(t, f)
	changed := in
	changed.QuantityMilli = 10000
	requireMaterials(t, f, "POST", lotsPath, changed, 409)
	changed = in
	changed.OperationID = "other"
	requireMaterials(t, f, "POST", lotsPath, changed, 409)
	changed.LotID = "second-id"
	requireMaterials(t, f, "POST", lotsPath, changed, 409)
	changed.Code = "BREAD-B"
	requireMaterials(t, f, "POST", lotsPath, changed, 409)
	changed.QuantityMilli = 7000
	changed.ExpiresOn = ""
	requireMaterials(t, f, "POST", lotsPath, changed, 201)
	lotSummary(t, f, in.ResultID, 27000, 0)
	materialCount(t, f, "production_lot_events", 2)
	materialCount(t, f, "outbox", 8)
	void := production.VoidLotInput{OperationID: "void-lot", LotID: in.LotID, ExpectedRevision: 2, Reason: "Corrigir identificacao"}
	requireMaterials(t, f, "POST", lotsPath+"/void", void, 409)
	void.ExpectedRevision = 1
	for i := 0; i < 2; i++ {
		requireMaterials(t, f, "POST", lotsPath+"/void", void, 200)
	}
	summary = lotSummary(t, f, in.ResultID, 7000, 20000)
	if summary.Items[0].Status != "voided" || summary.Items[0].Revision != 2 || summary.Items[0].Code != in.Code || summary.Items[1].ExpiresOn != "" {
		t.Fatal(summary)
	}
	var got production.ProductionLot
	body := requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID, nil, 200)
	if err := json.Unmarshal(body, &got); err != nil || got.Status != "voided" || got.Reason != in.Reason || got.QuantityMilli != 20000 {
		t.Fatal(string(body), err)
	}
	history, err := production.ProductionLotHistory(context.Background(), f.db, materialsActor(f), f.device, in.LotID)
	if err != nil || len(history) != 2 || history[0].Kind != "recorded" || history[1].Kind != "voided" || history[1].Reason != void.Reason || history[1].ActorID != f.owner.OwnerID {
		t.Fatal(history, err)
	}
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/history", nil, 200)
	body = requireMaterials(t, f, "POST", lotsPath, in, 200)
	var replay production.LotResult
	if err = json.Unmarshal(body, &replay); err != nil || !replay.Repeated || replay.Status != "recorded" {
		t.Fatal(string(body), err)
	}
	replacement := in
	replacement.OperationID, replacement.LotID = "corrected", "corrected-lot"
	requireMaterials(t, f, "POST", lotsPath, replacement, 409) // Voided code cannot be reused.
	replacement.Code = "BREAD-C"
	requireMaterials(t, f, "POST", lotsPath, replacement, 201)
	lotSummary(t, f, in.ResultID, 27000, 0)
	void.OperationID = "void-again"
	requireMaterials(t, f, "POST", lotsPath+"/void", void, 409)
	var audit string
	if err = f.db.QueryRow(`SELECT request_json FROM production_lot_events WHERE lot_id=? AND kind='recorded'`, in.LotID).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	var saved production.LotInput
	if err = json.Unmarshal([]byte(audit), &saved); err != nil || saved != in {
		t.Fatal(audit, err)
	}
	// Finished goods and shortfall use distinct allowances, with no double stock.
	requireMaterials(t, f, "POST", lossesPath, production.LossInput{OperationID: "classify-loss", LossID: "loss-1", ResultID: in.ResultID, QuantityMilli: 2000, Reason: "Perda declarada"}, 201)
	lossSummary(t, f, in.ResultID, 2000, 1000)
	lotSummary(t, f, in.ResultID, 27000, 0)
	unchangedProductionAfterLoss(t, f)
}
func TestHTTPProductionLotsValidationAuthorizationAndScope(t *testing.T) {
	for _, kind := range []string{"zero", "negative", "fraction", "overflow", "above-output", "foreign-result", "empty-code", "long-code", "invalid-date", "expiry-before", "empty-reason", "role", "module"} {
		t.Run(kind, func(t *testing.T) {
			f, in := lotsFixture(t)
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
			case "above-output":
				in.QuantityMilli = 28000
				want = 409
			case "foreign-result":
				in.ResultID = "outside"
				want = 404
			case "empty-code":
				in.Code = " "
			case "long-code":
				in.Code = strings.Repeat("á", 33)
			case "invalid-date":
				in.ManufacturedOn = "2023-02-29"
			case "expiry-before":
				in.ExpiresOn = "2024-02-28"
			case "empty-reason":
				in.Reason = " "
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
			requireMaterials(t, f, "POST", lotsPath, in, want)
			materialCount(t, f, "production_lots", 0)
			materialCount(t, f, "stock_movements", 5)
		})
	}
	f, in := lotsFixture(t)
	for _, body := range []string{`{}`, strings.Replace(orderBody(t, in), `"quantity_milli":20000`, `"quantity_milli":20000,"quantity_milli":20000`, 1), strings.Replace(orderBody(t, in), `"quantity_milli":20000`, `"quantity_milli":null`, 1), strings.Replace(orderBody(t, in), `"quantity_milli":20000`, `"quantity_milli":20000.1`, 1), strings.Replace(orderBody(t, in), `"expires_on":"2024-03-01"`, `"expires_on":null`, 1), strings.TrimSuffix(orderBody(t, in), "}") + `,"tenant_id":"outside"}`, orderBody(t, in) + `{}`} {
		status, _ := request(t, f.app, "POST", lotsPath, body, f.token)
		if status != 400 {
			t.Fatal(status, body)
		}
	}
	status, _ := request(t, f.app, "POST", lotsPath, orderBody(t, in), "")
	if status != 401 {
		t.Fatal(status)
	}
	requireMaterials(t, f, "POST", lotsPath, in, 201)
	trimmed := in
	trimmed.Code = " " + in.Code + " "
	trimmed.Reason = " " + in.Reason + " "
	requireMaterials(t, f, "POST", lotsPath, trimmed, 200)
	requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID+"/lots?offset=-1", nil, 400)
	requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID+"/lots?offset=0&offset=1", nil, 400)
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"?unexpected=1", nil, 400)
	requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: "void", LotID: "foreign", ExpectedRevision: 1, Reason: "Teste"}, 404)
	for _, company := range []bool{true, false} {
		foreign := materialsActor(f)
		if company {
			foreign.TenantID = "other"
		} else {
			foreign.StoreID = "other"
		}
		if _, err := production.GetProductionLot(context.Background(), f.db, foreign, f.device, in.LotID); err == nil {
			t.Fatal("lot isolation")
		}
		if _, err := production.ListProductionLots(context.Background(), f.db, foreign, f.device, in.ResultID, 0); err == nil {
			t.Fatal("summary isolation")
		}
		if _, err := production.ProductionLotHistory(context.Background(), f.db, foreign, f.device, in.LotID); err == nil {
			t.Fatal("audit isolation")
		}
	}
}
func TestHTTPProductionLotsWriteFailureRollsBack(t *testing.T) {
	for _, voiding := range []bool{false, true} {
		for _, table := range []string{"production_lots", "production_lot_events", "outbox"} {
			for _, failure := range []string{"IGNORE", "ABORT,'test'"} {
				t.Run(fmt.Sprintf("%t-%s-%s", voiding, table, failure), func(t *testing.T) {
					f, in := lotsFixture(t)
					count := 0
					operation := "INSERT"
					if voiding {
						requireMaterials(t, f, "POST", lotsPath, in, 201)
						count = 1
						if table == "production_lots" {
							operation = "UPDATE"
						}
					}
					if _, err := f.db.Exec("CREATE TRIGGER fail_lot BEFORE " + operation + " ON " + table + " BEGIN SELECT RAISE(" + failure + "); END"); err != nil {
						t.Fatal(err)
					}
					if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=1`); err != nil {
						t.Fatal(err)
					}
					path := lotsPath
					var input any = in
					if voiding {
						path += "/void"
						input = production.VoidLotInput{OperationID: "void", LotID: in.LotID, ExpectedRevision: 1, Reason: "Corrigir"}
					}
					status, _ := request(t, f.app, "POST", path, orderBody(t, input), f.token)
					if status < 400 {
						t.Fatal(status)
					}
					materialCount(t, f, "production_lots", count)
					materialCount(t, f, "production_lot_events", count)
					materialCount(t, f, "outbox", 6+count)
					var clock int64
					if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&clock); err != nil || clock != 1 {
						t.Fatal(clock, err)
					}
					if voiding {
						got, err := production.GetProductionLot(context.Background(), f.db, materialsActor(f), f.device, in.LotID)
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
func TestHTTPProductionLotsConcurrentAllocationsRespectOutput(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			f, in := lotsFixture(t)
			other := in
			if !same {
				other.OperationID, other.LotID, other.Code = "second", "second-lot", "BREAD-B"
			}
			bodies := []string{orderBody(t, in), orderBody(t, other)}
			statuses := make(chan int, 2)
			var wg sync.WaitGroup
			for _, b := range bodies {
				wg.Add(1)
				go func(body string) {
					defer wg.Done()
					status, _ := request(t, f.app, "POST", lotsPath, body, f.token)
					statuses <- status
				}(b)
			}
			wg.Wait()
			close(statuses)
			created, rest := 0, 0
			for status := range statuses {
				if status == 201 {
					created++
				} else if (same && status == 200) || (!same && status == 409) {
					rest++
				} else {
					t.Fatal(status)
				}
			}
			if created != 1 || rest != 1 {
				t.Fatal(created, rest)
			}
			materialCount(t, f, "production_lots", 1)
			lotSummary(t, f, in.ResultID, 20000, 7000)
			unchangedProductionAfterLoss(t, f)
		})
	}
}
func TestHTTPProductionLotsConcurrentVoidsAreNotDuplicated(t *testing.T) {
	f, in := lotsFixture(t)
	requireMaterials(t, f, "POST", lotsPath, in, 201)
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, op := range []string{"void-one", "void-two"} {
		body := orderBody(t, production.VoidLotInput{OperationID: op, LotID: in.LotID, ExpectedRevision: 1, Reason: "Corrigir"})
		wg.Add(1)
		go func(body string) {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", lotsPath+"/void", body, f.token)
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
	materialCount(t, f, "production_lot_events", 2)
	lotSummary(t, f, in.ResultID, 0, 27000)
	unchangedProductionAfterLoss(t, f)
}
func TestHTTPProductionLotsBackupRestoresDefinitionsAndVoidedAudit(t *testing.T) {
	f, in := lotsFixture(t)
	requireMaterials(t, f, "POST", lotsPath, in, 201)
	requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: "void", LotID: in.LotID, ExpectedRevision: 1, Reason: "Corrigir"}, 200)
	replacement := in
	replacement.OperationID, replacement.LotID, replacement.Code = "replace", "replacement", "BREAD-B"
	replacement.QuantityMilli = 27000
	replacement.ExpiresOn = ""
	requireMaterials(t, f, "POST", lotsPath, replacement, 201)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "lots.tytbak")
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
	summary, err := production.ListProductionLots(ctx, db, actor, f.device, in.ResultID, 0)
	if err != nil || summary.AssignedMilli != 27000 || summary.UnassignedMilli != 0 || len(summary.Items) != 2 || summary.Items[0].Status != "voided" || summary.Items[0].Code != in.Code || summary.Items[0].ExpiresOn != in.ExpiresOn || summary.Items[1].Code != replacement.Code || summary.Items[1].ExpiresOn != "" {
		t.Fatal(summary, err)
	}
	history, err := production.ProductionLotHistory(ctx, db, actor, f.device, in.LotID)
	if err != nil || len(history) != 2 || history[1].Reason != "Corrigir" {
		t.Fatal(history, err)
	}
	balance, err := stock.Balance(ctx, db, actor, f.device, "bread", "production-room")
	if err != nil || balance != 27000 {
		t.Fatal(balance, err)
	}
	result, err := production.GetProductionResult(ctx, db, actor, f.device, in.ResultID)
	if err != nil || result.OrderID != "order-1" || result.ProducedMilli != 27000 {
		t.Fatal(result, err)
	}
	order, err := production.GetOrder(ctx, db, actor, f.device, result.OrderID)
	if err != nil || order.Status != "completed" || order.Recipe.VersionID != order.VersionID || len(order.Recipe.Ingredients) != 2 {
		t.Fatal(order, err)
	}
}
func TestHTTPProductionLotsExpiredContractAndRevokedRole(t *testing.T) {
	for _, expired := range []bool{true, false} {
		t.Run(fmt.Sprint(expired), func(t *testing.T) {
			f, in := lotsFixture(t)
			requireMaterials(t, f, "POST", lotsPath, in, 201)
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
			requireMaterials(t, f, "POST", lotsPath, in, 403)
			requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: "void", LotID: in.LotID, ExpectedRevision: 1, Reason: "Corrigir"}, 403)
			want := 403
			if expired {
				want = 200
			}
			requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID, nil, want)
			requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID+"/lots", nil, want)
			materialCount(t, f, "production_lot_events", 1)
			materialCount(t, f, "stock_movements", 5)
		})
	}
}
func TestHTTPProductionLotsAggregateOverflowFailsClosed(t *testing.T) {
	f, in := lotsFixture(t)
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1025; i++ {
		id := fmt.Sprintf("bad-%d", i)
		if _, err = tx.Exec(`INSERT INTO production_lots VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, f.owner.TenantID, f.owner.StoreID, id, in.ResultID, "bread", "unit", production.MaxQuantity, id, in.ManufacturedOn, in.ExpiresOn, "Corrupt fixture", "recorded", 1, f.owner.OwnerID, "now", "now"); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", lotsPath, in, 409)
	requireMaterials(t, f, "GET", resultsPath+"/"+in.ResultID+"/lots", nil, 409)
	materialCount(t, f, "production_lot_events", 0)
	unchangedProductionAfterLoss(t, f)
}
func TestHTTPProductionLotsHistoricUnitNoOutputAndPagination(t *testing.T) {
	f, in := lotsFixture(t)
	if _, err := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='bread'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", lotsPath, in, 201)
	summary := lotSummary(t, f, in.ResultID, 20000, 7000)
	if summary.Unit != "unit" || summary.Items[0].Unit != "unit" {
		t.Fatal(summary)
	}
	f, result := resultsFixture(t)
	result.ProducedMilli = 0
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	in.ResultID = result.ResultID
	requireMaterials(t, f, "POST", lotsPath, in, 409)
	lotSummary(t, f, result.ResultID, 0, 0)
	// Exact 0.001 g and global totals independent of pagination.
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
		v := production.LotInput{OperationID: fmt.Sprintf("op-%02d", i), LotID: fmt.Sprintf("lot-%02d", i), ResultID: "result-1", Code: fmt.Sprintf("CODE-%02d", i), ManufacturedOn: "2024-02-29", ExpiresOn: "", QuantityMilli: 1, Reason: "0,001 g identificado"}
		requireMaterials(t, f, "POST", lotsPath, v, 201)
	}
	summary = lotSummary(t, f, "result-1", 52, 26948)
	if len(summary.Items) != 50 || summary.Unit != "g" || summary.Items[0].QuantityMilli != 1 {
		t.Fatal(summary)
	}
	body := requireMaterials(t, f, "GET", resultsPath+"/result-1/lots?offset=50", nil, 200)
	if err := json.Unmarshal(body, &summary); err != nil || len(summary.Items) != 2 || summary.AssignedMilli != 52 || summary.UnassignedMilli != 26948 {
		t.Fatal(string(body), err)
	}
	body = requireMaterials(t, f, "GET", resultsPath+"/result-1/lots?offset=52", nil, 200)
	if err := json.Unmarshal(body, &summary); err != nil || len(summary.Items) != 0 || summary.AssignedMilli != 52 {
		t.Fatal(string(body), err)
	}
	materialCount(t, f, "stock_movements", 5)
}
