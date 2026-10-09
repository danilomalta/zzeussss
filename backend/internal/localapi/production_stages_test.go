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
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/stock"
)

const stagePlansPath = "/local/v1/production/stage-plans"
const stageStatePath = "/local/v1/production/stages/state"

func stagesFixture(t *testing.T) (*httpContractFixture, production.StagePlanInput, production.ReserveInput) {
	t.Helper()
	f, reserve := materialsFixture(t)
	in := production.StagePlanInput{OperationID: "configure-stages", OrderID: reserve.OrderID, Reason: "Sequencia de preparo", Stages: []production.StageDefinition{{StageID: "mix", Name: "Misturar", ResponsibleID: f.owner.OwnerID}, {StageID: "bake", Name: "Assar", ResponsibleID: f.owner.OwnerID}}}
	return f, in, reserve
}
func consumeStages(t *testing.T, f *httpContractFixture, reserve production.ReserveInput) {
	t.Helper()
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "Executar"}, 200)
}
func stageInput(in production.StagePlanInput, id, status string) production.StageStateInput {
	revision := int64(1)
	if status == "completed" {
		revision = 2
	}
	return production.StageStateInput{OperationID: id + "-" + status, OrderID: in.OrderID, StageID: id, ExpectedRevision: revision, Status: status, Reason: "Executar etapa"}
}
func finishStages(t *testing.T, f *httpContractFixture, in production.StagePlanInput) {
	t.Helper()
	for _, stage := range in.Stages {
		for _, state := range []string{"running", "completed"} {
			requireMaterials(t, f, "POST", stageStatePath, stageInput(in, stage.StageID, state), 200)
		}
	}
}
func stageCompletion(in production.StagePlanInput) production.ResultInput {
	return production.ResultInput{OperationID: "complete", ResultID: "result-1", OrderID: in.OrderID, ExpectedRevision: 2, ProducedMilli: 27000, Reason: "Resultado medido"}
}
func TestHTTPProductionStagesSequenceReplayAndCompletionGuard(t *testing.T) {
	f, in, reserve := stagesFixture(t)
	for _, want := range []int{201, 200} {
		requireMaterials(t, f, "POST", stagePlansPath, in, want)
	}
	materialCount(t, f, "stock_movements", 2)
	materialCount(t, f, "production_material_reservations", 0)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "mix", "running"), 409)
	consumeStages(t, f, reserve)
	completion := stageCompletion(in)
	requireMaterials(t, f, "POST", resultsPath, completion, 409)
	materialCount(t, f, "stock_movements", 4)
	materialCount(t, f, "production_results", 0)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "bake", "running"), 409)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "mix", "completed"), 409)
	running := stageInput(in, "mix", "running")
	for i := 0; i < 2; i++ {
		requireMaterials(t, f, "POST", stageStatePath, running, 200)
	}
	changed := running
	changed.Reason = "Different"
	requireMaterials(t, f, "POST", stageStatePath, changed, 409)
	changed = running
	changed.OperationID = "stale"
	requireMaterials(t, f, "POST", stageStatePath, changed, 409)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "mix", "completed"), 200)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "bake", "running"), 200)
	requireMaterials(t, f, "POST", resultsPath, completion, 409)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "bake", "completed"), 200)
	materialCount(t, f, "stock_movements", 4)
	materialCount(t, f, "production_stage_events", 5)
	materialCount(t, f, "outbox", 10)
	requireMaterials(t, f, "POST", resultsPath, completion, 201)
	requireMaterials(t, f, "POST", resultsPath, completion, 200)
	// Historical replay remains available after completion; it is not a new step.
	b := requireMaterials(t, f, "POST", stageStatePath, running, 200)
	var replay production.StageResult
	if err := json.Unmarshal(b, &replay); err != nil || !replay.Repeated || replay.Revision != 2 || replay.Status != "running" {
		t.Fatal(string(b), err)
	}
	plan, err := production.GetStagePlan(context.Background(), f.db, materialsActor(f), f.device, in.OrderID)
	if err != nil || len(plan.Items) != 2 || plan.Items[0].Name != "Misturar" || plan.Items[0].Revision != 3 || plan.Items[1].Status != "completed" {
		t.Fatal(plan, err)
	}
	events, err := production.StageHistory(context.Background(), f.db, materialsActor(f), f.device, in.OrderID, 0)
	if err != nil || len(events) != 5 || events[0].Kind != "configured" || events[0].ActorID != f.owner.OwnerID || events[4].Kind != "completed" {
		t.Fatal(events, err)
	}
	for i, e := range events {
		if !json.Valid(e.Request) || !json.Valid(e.Result) || (i > 0 && e.Sequence <= events[i-1].Sequence) {
			t.Fatal(e)
		}
	}
	requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID+"/stages", nil, 200)
	requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID+"/stages/history?offset=5", nil, 200)
	changed = running
	changed.OperationID = "after-completion"
	requireMaterials(t, f, "POST", stageStatePath, changed, 409)
	unchangedProductionAfterLoss(t, f)
}
func TestHTTPProductionStagesImmutablePlanValidationAndStrictJSON(t *testing.T) {
	f, in, reserve := stagesFixture(t)
	invalid := []production.StagePlanInput{}
	for _, mode := range []string{"empty", "duplicate", "name", "responsible", "count", "foreign-order", "reason"} {
		v := in
		v.Stages = append([]production.StageDefinition(nil), in.Stages...)
		switch mode {
		case "empty":
			v.Stages = nil
		case "duplicate":
			v.Stages[1].StageID = v.Stages[0].StageID
		case "name":
			v.Stages[0].Name = " "
		case "responsible":
			v.Stages[0].ResponsibleID = "absent"
		case "count":
			for i := 0; i < 19; i++ {
				v.Stages = append(v.Stages, production.StageDefinition{StageID: fmt.Sprint(i), Name: "Test", ResponsibleID: f.owner.OwnerID})
			}
		case "foreign-order":
			v.OrderID = "foreign"
		case "reason":
			v.Reason = " "
		}
		invalid = append(invalid, v)
	}
	for i, v := range invalid {
		want := 400
		if i == 3 {
			want = 403
		}
		if i == 5 {
			want = 404
		}
		requireMaterials(t, f, "POST", stagePlansPath, v, want)
	}
	for _, b := range []string{"{}", strings.Replace(orderBody(t, in), `"name":"Misturar"`, `"name":"Misturar","name":"Assar"`, 1), strings.Replace(orderBody(t, in), `"name":"Misturar"`, `"name":null`, 1), strings.Replace(orderBody(t, in), `"name":"Misturar"`, `"name":"Misturar","extra":1`, 1), strings.Replace(orderBody(t, in), `"stages":[`, `"stages":[null,`, 1)} {
		status, _ := request(t, f.app, "POST", stagePlansPath, b, f.token)
		if status != 400 {
			t.Fatal(status, b)
		}
	}
	requireMaterials(t, f, "POST", stagePlansPath, in, 201)
	changed := in
	changed.OperationID = "new-plan"
	changed.Stages = append([]production.StageDefinition(nil), in.Stages...)
	changed.Stages[0].Name = "Changed"
	requireMaterials(t, f, "POST", stagePlansPath, changed, 409)
	changed.OperationID = in.OperationID
	requireMaterials(t, f, "POST", stagePlansPath, changed, 409)
	// No silent reordering: reversing a replay changes its request.
	changed = in
	changed.Stages = []production.StageDefinition{in.Stages[1], in.Stages[0]}
	requireMaterials(t, f, "POST", stagePlansPath, changed, 409)
	consumeStages(t, f, reserve)
	bad := stageInput(in, "mix", "running")
	for _, mode := range []string{"skip", "revision", "overflow", "foreign-stage", "reason"} {
		v := bad
		want := 400
		switch mode {
		case "skip":
			v.Status = "pending"
		case "revision":
			v.ExpectedRevision = 2
			want = 409
		case "overflow":
			v.ExpectedRevision = production.MaxRevision + 1
		case "foreign-stage":
			v.StageID = "outside"
			want = 404
		case "reason":
			v.Reason = " "
		}
		requireMaterials(t, f, "POST", stageStatePath, v, want)
	}
	for _, b := range []string{strings.Replace(orderBody(t, bad), `"expected_revision":1`, `"expected_revision":1.1`, 1), strings.Replace(orderBody(t, bad), `"expected_revision":1`, `"expected_revision":1,"expected_revision":1`, 1)} {
		status, _ := request(t, f.app, "POST", stageStatePath, b, f.token)
		if status != 400 {
			t.Fatal(status, b)
		}
	}
	materialCount(t, f, "production_stage_events", 1)
	requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID+"/stages?offset=1", nil, 400)
	requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID+"/stages/history?offset=-1", nil, 400)
	requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID+"/stages/history?offset=0&offset=1", nil, 400)
}
func TestHTTPProductionStagesCannotConfigureAfterHoldOrConsumption(t *testing.T) {
	for _, consume := range []bool{false, true} {
		t.Run(fmt.Sprint(consume), func(t *testing.T) {
			f, in, reserve := stagesFixture(t)
			requireMaterials(t, f, "POST", materialsPath, reserve, 201)
			if consume {
				requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume", ReservationID: reserve.ReservationID, Action: "consume", Reason: "Executar"}, 200)
			}
			requireMaterials(t, f, "POST", stagePlansPath, in, 409)
			materialCount(t, f, "production_stage_plans", 0)
		})
	}
}
func TestHTTPProductionStagesAuthorizationScopeAndAssignment(t *testing.T) {
	f, in, reserve := stagesFixture(t)
	if _, err := f.db.Exec(`INSERT INTO identities VALUES('other-production','Outro','now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO memberships VALUES(?,'other-production','production','active','now')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,'other-production',?)`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	in.Stages[0].ResponsibleID = "other-production"
	requireMaterials(t, f, "POST", stagePlansPath, in, 201)
	consumeStages(t, f, reserve)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "mix", "running"), 403)
	actor := materialsActor(f)
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	license, err := entitlementstore.New(f.db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []bool{false, true} {
		foreign := actor
		if tenant {
			foreign.TenantID = "foreign"
		} else {
			foreign.StoreID = "foreign"
		}
		if _, err := production.GetStagePlan(context.Background(), f.db, foreign, f.device, in.OrderID); err == nil {
			t.Fatal("isolation")
		}
		if _, err := production.StageHistory(context.Background(), f.db, foreign, f.device, in.OrderID, 0); err == nil {
			t.Fatal("audit isolation")
		}
	}
	status, _ := request(t, f.app, "POST", stageStatePath, orderBody(t, stageInput(in, "mix", "running")), "")
	if status != 401 {
		t.Fatal(status)
	}
	// Assigned staff can execute; removing its membership forbids new transitions.
	assigned := actor
	assigned.IdentityID = "other-production"
	if _, err := production.ChangeStageState(context.Background(), f.db, license, assigned, f.device, stageInput(in, "mix", "running")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id='other-production'`); err != nil {
		t.Fatal(err)
	}
	if _, err := production.ChangeStageState(context.Background(), f.db, license, assigned, f.device, stageInput(in, "mix", "completed")); err == nil {
		t.Fatal("revoked assignee")
	}
	materialCount(t, f, "production_stage_events", 2)
}
func TestHTTPProductionStagesExpiredContractAndRole(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprint(expired), func(t *testing.T) {
			f, in, reserve := stagesFixture(t)
			requireMaterials(t, f, "POST", stagePlansPath, in, 201)
			consumeStages(t, f, reserve)
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
			requireMaterials(t, f, "POST", stagePlansPath, in, 403)
			requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "mix", "running"), 403)
			want := 403
			if expired {
				want = 200
			}
			requireMaterials(t, f, "GET", orderPath+"/"+in.OrderID+"/stages", nil, want)
			materialCount(t, f, "production_stage_events", 1)
		})
	}
}
func TestHTTPProductionStagesAtomicRollback(t *testing.T) {
	for _, transition := range []bool{false, true} {
		for _, table := range []string{"production_stage_plans", "production_stages", "production_stage_events", "outbox"} {
			if transition && table == "production_stage_plans" {
				continue
			}
			for _, failure := range []string{"IGNORE", "ABORT,'test'"} {
				t.Run(fmt.Sprintf("%t-%s-%s", transition, table, failure), func(t *testing.T) {
					f, in, reserve := stagesFixture(t)
					count := 0
					outbox := 3
					operation := "INSERT"
					if transition {
						requireMaterials(t, f, "POST", stagePlansPath, in, 201)
						consumeStages(t, f, reserve)
						count = 1
						outbox = 6
						if table == "production_stages" {
							operation = "UPDATE"
						}
					}
					if _, err := f.db.Exec("CREATE TRIGGER fail_stage BEFORE " + operation + " ON " + table + " BEGIN SELECT RAISE(" + failure + "); END"); err != nil {
						t.Fatal(err)
					}
					if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=1`); err != nil {
						t.Fatal(err)
					}
					path := stagePlansPath
					var input any = in
					if transition {
						path = stageStatePath
						input = stageInput(in, "mix", "running")
					}
					status, _ := request(t, f.app, "POST", path, orderBody(t, input), f.token)
					if status < 400 {
						t.Fatal(status)
					}
					materialCount(t, f, "production_stage_plans", count)
					materialCount(t, f, "production_stage_events", count)
					materialCount(t, f, "outbox", outbox)
					var clock int64
					if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&clock); err != nil || clock != 1 {
						t.Fatal(clock, err)
					}
					if transition {
						plan, err := production.GetStagePlan(context.Background(), f.db, materialsActor(f), f.device, in.OrderID)
						if err != nil || plan.Items[0].Status != "pending" || plan.Items[0].Revision != 1 {
							t.Fatal(plan, err)
						}
					} else {
						materialCount(t, f, "production_stages", 0)
					}
				})
			}
		}
	}
}
func TestHTTPProductionStagesConcurrentTransitions(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			f, in, reserve := stagesFixture(t)
			requireMaterials(t, f, "POST", stagePlansPath, in, 201)
			consumeStages(t, f, reserve)
			first := stageInput(in, "mix", "running")
			second := first
			if !same {
				second.OperationID = "other-start"
			}
			statuses := make(chan int, 2)
			var wg sync.WaitGroup
			for _, v := range []production.StageStateInput{first, second} {
				wg.Add(1)
				go func(v production.StageStateInput) {
					defer wg.Done()
					status, _ := request(t, f.app, "POST", stageStatePath, orderBody(t, v), f.token)
					statuses <- status
				}(v)
			}
			wg.Wait()
			close(statuses)
			ok, conflict := 0, 0
			for status := range statuses {
				switch status {
				case 200:
					ok++
				case 409:
					conflict++
				default:
					t.Fatal(status)
				}
			}
			if (same && (ok != 2 || conflict != 0)) || (!same && (ok != 1 || conflict != 1)) {
				t.Fatal(ok, conflict)
			}
			materialCount(t, f, "production_stage_events", 2)
			materialCount(t, f, "stock_movements", 4)
		})
	}
}
func TestHTTPProductionStagesBackupRestoresDefinitionsAndHistory(t *testing.T) {
	f, in, reserve := stagesFixture(t)
	requireMaterials(t, f, "POST", stagePlansPath, in, 201)
	consumeStages(t, f, reserve)
	finishStages(t, f, in)
	requireMaterials(t, f, "POST", resultsPath, stageCompletion(in), 201)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "stages.tytbak")
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
	plan, err := production.GetStagePlan(ctx, db, actor, f.device, in.OrderID)
	if err != nil || len(plan.Items) != 2 || plan.Items[0].Name != in.Stages[0].Name || plan.Items[0].ResponsibleID != f.owner.OwnerID || plan.Items[1].Revision != 3 || plan.Items[1].Status != "completed" {
		t.Fatal(plan, err)
	}
	events, err := production.StageHistory(ctx, db, actor, f.device, in.OrderID, 0)
	if err != nil || len(events) != 5 || events[0].Kind != "configured" || events[4].Kind != "completed" || !json.Valid(events[0].Request) {
		t.Fatal(events, err)
	}
	balance, err := stock.Balance(ctx, db, actor, f.device, "bread", "production-room")
	if err != nil || balance != 27000 {
		t.Fatal(balance, err)
	}
	order, err := production.GetOrder(ctx, db, actor, f.device, in.OrderID)
	if err != nil || order.Status != "completed" || order.Revision != 3 {
		t.Fatal(order, err)
	}
}

func TestHTTPProductionStagesPlanBeforeApprovalAndCancellation(t *testing.T) {
	f, order := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, order, 201)
	in := production.StagePlanInput{OperationID: "configure", OrderID: order.OrderID, Reason: "Plano inicial", Stages: []production.StageDefinition{{StageID: "prepare", Name: "Preparo", ResponsibleID: f.owner.OwnerID}}}
	requireMaterials(t, f, "POST", stagePlansPath, in, 201)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "prepare", "running"), 409)
	requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: "cancel", OrderID: order.OrderID, ExpectedRevision: 1, Status: "cancelled", Reason: "Cancelar plano"}, 200)
	requireMaterials(t, f, "POST", stageStatePath, stageInput(in, "prepare", "running"), 409)
	requireMaterials(t, f, "GET", orderPath+"/"+order.OrderID+"/stages", nil, 200)
	requireMaterials(t, f, "POST", stagePlansPath, in, 200)
	materialCount(t, f, "stock_movements", 0)
}
func TestHTTPProductionStagesReleasedHoldAllowsPlanButIncompleteDefinitionBlocksOutput(t *testing.T) {
	f, in, reserve := stagesFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "release", ReservationID: reserve.ReservationID, Action: "release", Reason: "Replanejar"}, 200)
	requireMaterials(t, f, "POST", stagePlansPath, in, 201)
	reserve.OperationID, reserve.ReservationID = "reserve-again", "materials-2"
	consumeStages(t, f, reserve)
	finishStages(t, f, in)
	// Corrupt only this disposable fixture: a missing step must not be treated as
	// all remaining steps complete. The declared count preserves the requirement.
	if _, err := f.db.Exec(`DELETE FROM production_stages WHERE id='bake'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", resultsPath, stageCompletion(in), 409)
	materialCount(t, f, "production_results", 0)
	materialCount(t, f, "stock_movements", 4)
}
