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

const qualityPath = "/local/v1/production/quality-reviews"

func qualityFixture(t *testing.T) (*httpContractFixture, production.QualityInput, production.LotInput) {
	t.Helper()
	f, lot := lotsFixture(t)
	requireMaterials(t, f, "POST", lotsPath, lot, 201)
	return f, production.QualityInput{OperationID: "review-1", LotID: lot.LotID, ExpectedRevision: 0, Status: "failed", Criterion: "Criterio interno informado pelo operador", Reason: "Parecer humano registrado"}, lot
}
func TestHTTPProductionQualityRevisionHistoryReplayAndNoStockChange(t *testing.T) {
	f, in, lot := qualityFixture(t)
	ctx := context.Background()
	actor := materialsActor(f)
	current, err := production.GetLotQuality(ctx, f.db, actor, f.device, in.LotID)
	if err != nil || current.Status != "not_assessed" || current.Revision != 0 || current.Latest != nil || current.LotStatus != "recorded" {
		t.Fatal(current, err)
	}
	history, err := production.LotQualityHistory(ctx, f.db, actor, f.device, in.LotID, 0)
	if err != nil || len(history) != 0 {
		t.Fatal(history, err)
	}
	for _, want := range []int{201, 200} {
		requireMaterials(t, f, "POST", qualityPath, in, want)
	}
	current, err = production.GetLotQuality(ctx, f.db, actor, f.device, in.LotID)
	if err != nil || current.Revision != 1 || current.Status != "failed" || current.Latest == nil || current.Latest.ActorID != f.owner.OwnerID || current.Latest.Criterion != in.Criterion || current.Latest.LotSnapshot.Code != lot.Code || current.Latest.LotSnapshot.QuantityMilli != lot.QuantityMilli {
		t.Fatal(current, err)
	}
	changed := in
	changed.Status = "passed"
	requireMaterials(t, f, "POST", qualityPath, changed, 409)
	changed = in
	changed.OperationID = "stale"
	requireMaterials(t, f, "POST", qualityPath, changed, 409)
	second := in
	second.OperationID = "review-2"
	second.ExpectedRevision = 1
	second.Status = "passed"
	second.Reason = "Nova avaliacao humana motivada"
	requireMaterials(t, f, "POST", qualityPath, second, 201)
	// Same verdict may be reassessed, but never overwritten without a new revision.
	third := second
	third.OperationID = "review-3"
	third.ExpectedRevision = 2
	third.Criterion = "Criterio adicional declarado"
	requireMaterials(t, f, "POST", qualityPath, third, 201)
	body := requireMaterials(t, f, "POST", qualityPath, in, 200)
	var replay production.QualityResult
	if err = json.Unmarshal(body, &replay); err != nil || !replay.Repeated || replay.Revision != 1 || replay.Status != "failed" {
		t.Fatal(string(body), err)
	}
	current, err = production.GetLotQuality(ctx, f.db, actor, f.device, in.LotID)
	if err != nil || current.Revision != 3 || current.Status != "passed" || current.Latest.Reason != second.Reason || current.Latest.Criterion != third.Criterion {
		t.Fatal(current, err)
	}
	history, err = production.LotQualityHistory(ctx, f.db, actor, f.device, in.LotID, 0)
	if err != nil || len(history) != 3 || history[0].Status != "failed" || history[1].Status != "passed" || history[0].LotSnapshot.Code != lot.Code || history[0].LotSnapshot.ManufacturedOn != lot.ManufacturedOn {
		t.Fatal(history, err)
	}
	for i, v := range history {
		if v.Revision != int64(i+1) || v.ActorID != f.owner.OwnerID || v.DeviceID != f.device.DeviceID || v.LotSnapshot.ResultID != lot.ResultID {
			t.Fatal(v)
		}
	}
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality", nil, 200)
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality/history?offset=3", nil, 200)
	materialCount(t, f, "production_quality_reviews", 3)
	materialCount(t, f, "outbox", 10)
	lotSummary(t, f, lot.ResultID, 20000, 7000)
	unchangedProductionAfterLoss(t, f)
}
func TestHTTPProductionQualityVoidedLotKeepsHistoricalVerdict(t *testing.T) {
	f, in, lot := qualityFixture(t)
	requireMaterials(t, f, "POST", qualityPath, in, 201)
	requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: "void", LotID: lot.LotID, ExpectedRevision: 1, Reason: "Corrigir lote"}, 200)
	next := in
	next.OperationID = "after-void"
	next.ExpectedRevision = 1
	next.Status = "passed"
	requireMaterials(t, f, "POST", qualityPath, next, 409)
	requireMaterials(t, f, "POST", qualityPath, in, 200)
	current, err := production.GetLotQuality(context.Background(), f.db, materialsActor(f), f.device, in.LotID)
	if err != nil || current.LotStatus != "voided" || current.Status != "failed" || current.Revision != 1 || current.Latest.LotSnapshot.Status != "recorded" || current.Latest.LotSnapshot.Revision != 1 {
		t.Fatal(current, err)
	}
	history, err := production.LotQualityHistory(context.Background(), f.db, materialsActor(f), f.device, in.LotID, 0)
	if err != nil || len(history) != 1 {
		t.Fatal(history, err)
	}
	materialCount(t, f, "production_quality_reviews", 1)
	lotSummary(t, f, lot.ResultID, 0, 27000)
	unchangedProductionAfterLoss(t, f)
}
func TestHTTPProductionQualityValidationAuthorizationAndIsolation(t *testing.T) {
	for _, kind := range []string{"negative", "overflow", "status", "criterion", "reason", "foreign-lot", "role", "module"} {
		t.Run(kind, func(t *testing.T) {
			f, in, _ := qualityFixture(t)
			want := 400
			switch kind {
			case "negative":
				in.ExpectedRevision = -1
			case "overflow":
				in.ExpectedRevision = production.MaxRevision
			case "status":
				in.Status = "released"
			case "criterion":
				in.Criterion = " "
			case "reason":
				in.Reason = " "
			case "foreign-lot":
				in.LotID = "outside-store"
				want = 404
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
			requireMaterials(t, f, "POST", qualityPath, in, want)
			materialCount(t, f, "production_quality_reviews", 0)
			materialCount(t, f, "stock_movements", 5)
		})
	}
	f, in, _ := qualityFixture(t)
	for _, body := range []string{`{}`, strings.Replace(orderBody(t, in), `"expected_revision":0`, `"expected_revision":0,"expected_revision":0`, 1), strings.Replace(orderBody(t, in), `"expected_revision":0`, `"expected_revision":null`, 1), strings.Replace(orderBody(t, in), `"expected_revision":0`, `"expected_revision":0.1`, 1), strings.Replace(orderBody(t, in), `"expected_revision":0`, `"expected_revision":9223372036854775808`, 1), strings.TrimSuffix(orderBody(t, in), "}") + `,"actor_id":"other"}`, orderBody(t, in) + `{}`} {
		status, _ := request(t, f.app, "POST", qualityPath, body, f.token)
		if status != 400 {
			t.Fatal(status, body)
		}
	}
	status, _ := request(t, f.app, "POST", qualityPath, orderBody(t, in), "")
	if status != 401 {
		t.Fatal(status)
	}
	requireMaterials(t, f, "POST", qualityPath, in, 201)
	trimmed := in
	trimmed.Criterion = " " + in.Criterion + " "
	trimmed.Reason = " " + in.Reason + " "
	requireMaterials(t, f, "POST", qualityPath, trimmed, 200)
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality?offset=1", nil, 400)
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality/history?offset=-1", nil, 400)
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality/history?offset=0&offset=1", nil, 400)
	requireMaterials(t, f, "GET", lotsPath+"/foreign/quality", nil, 404)
	for _, company := range []bool{true, false} {
		foreign := materialsActor(f)
		if company {
			foreign.TenantID = "other"
		} else {
			foreign.StoreID = "other"
		}
		if _, err := production.GetLotQuality(context.Background(), f.db, foreign, f.device, in.LotID); err == nil {
			t.Fatal("quality isolation")
		}
		if _, err := production.LotQualityHistory(context.Background(), f.db, foreign, f.device, in.LotID, 0); err == nil {
			t.Fatal("history isolation")
		}
	}
}
func TestHTTPProductionQualityAtomicRollback(t *testing.T) {
	for _, table := range []string{"production_quality_reviews", "outbox"} {
		for _, failure := range []string{"IGNORE", "ABORT,'test'"} {
			t.Run(table+failure, func(t *testing.T) {
				f, in, _ := qualityFixture(t)
				requireMaterials(t, f, "POST", qualityPath, in, 201)
				in.OperationID = "review-2"
				in.ExpectedRevision = 1
				in.Status = "passed"
				if _, err := f.db.Exec("CREATE TRIGGER fail_quality BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(" + failure + "); END"); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=1`); err != nil {
					t.Fatal(err)
				}
				status, _ := request(t, f.app, "POST", qualityPath, orderBody(t, in), f.token)
				if status < 400 {
					t.Fatal(status)
				}
				materialCount(t, f, "production_quality_reviews", 1)
				materialCount(t, f, "outbox", 8)
				var clock int64
				if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&clock); err != nil || clock != 1 {
					t.Fatal(clock, err)
				}
				current, err := production.GetLotQuality(context.Background(), f.db, materialsActor(f), f.device, in.LotID)
				if err != nil || current.Revision != 1 || current.Status != "failed" {
					t.Fatal(current, err)
				}
				unchangedProductionAfterLoss(t, f)
			})
		}
	}
}
func TestHTTPProductionQualityConcurrentRevisionAndReplay(t *testing.T) {
	for _, same := range []bool{true, false} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			f, in, _ := qualityFixture(t)
			other := in
			if !same {
				other.OperationID = "competing"
				other.Status = "passed"
			}
			statuses := make(chan int, 2)
			var wg sync.WaitGroup
			for _, v := range []production.QualityInput{in, other} {
				body := orderBody(t, v)
				wg.Add(1)
				go func(body string) {
					defer wg.Done()
					status, _ := request(t, f.app, "POST", qualityPath, body, f.token)
					statuses <- status
				}(body)
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
			materialCount(t, f, "production_quality_reviews", 1)
			materialCount(t, f, "outbox", 8)
			unchangedProductionAfterLoss(t, f)
		})
	}
}
func TestHTTPProductionQualityBackupRestoresVerdictsAndLotSnapshot(t *testing.T) {
	f, in, lot := qualityFixture(t)
	requireMaterials(t, f, "POST", qualityPath, in, 201)
	next := in
	next.OperationID = "review-2"
	next.ExpectedRevision = 1
	next.Status = "passed"
	next.Reason = "Reavaliacao motivada"
	requireMaterials(t, f, "POST", qualityPath, next, 201)
	requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: "void", LotID: lot.LotID, ExpectedRevision: 1, Reason: "Corrigir lote"}, 200)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "quality.tytbak")
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
	current, err := production.GetLotQuality(ctx, db, actor, f.device, in.LotID)
	if err != nil || current.Revision != 2 || current.Status != "passed" || current.LotStatus != "voided" || current.Latest.LotSnapshot.Code != lot.Code || current.Latest.LotSnapshot.ExpiresOn != lot.ExpiresOn || current.Latest.LotSnapshot.Status != "recorded" || current.Latest.ActorID != f.owner.OwnerID {
		t.Fatal(current, err)
	}
	history, err := production.LotQualityHistory(ctx, db, actor, f.device, in.LotID, 0)
	if err != nil || len(history) != 2 || history[0].Status != "failed" || history[1].Reason != next.Reason || history[0].LotSnapshot.QuantityMilli != lot.QuantityMilli {
		t.Fatal(history, err)
	}
	balance, err := stock.Balance(ctx, db, actor, f.device, "bread", "production-room")
	if err != nil || balance != 27000 {
		t.Fatal(balance, err)
	}
	result, err := production.GetProductionResult(ctx, db, actor, f.device, lot.ResultID)
	if err != nil || result.OrderID != "order-1" {
		t.Fatal(result, err)
	}
	order, err := production.GetOrder(ctx, db, actor, f.device, result.OrderID)
	if err != nil || order.Status != "completed" || order.Recipe.VersionID != order.VersionID || len(order.Recipe.Ingredients) != 2 {
		t.Fatal(order, err)
	}
}
func TestHTTPProductionQualityContractExpiryAndRevokedRole(t *testing.T) {
	for _, expired := range []bool{true, false} {
		t.Run(fmt.Sprint(expired), func(t *testing.T) {
			f, in, _ := qualityFixture(t)
			requireMaterials(t, f, "POST", qualityPath, in, 201)
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
			requireMaterials(t, f, "POST", qualityPath, in, 403)
			in.OperationID = "after-revoke"
			in.ExpectedRevision = 1
			requireMaterials(t, f, "POST", qualityPath, in, 403)
			want := 403
			if expired {
				want = 200
			}
			requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality", nil, want)
			requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality/history", nil, want)
			materialCount(t, f, "production_quality_reviews", 1)
			materialCount(t, f, "stock_movements", 5)
		})
	}
}
func TestHTTPProductionQualityPaginationAndIncompleteHistoryFailClosed(t *testing.T) {
	f, in, _ := qualityFixture(t)
	for i := 0; i < 52; i++ {
		in.OperationID = fmt.Sprintf("review-%02d", i)
		in.ExpectedRevision = int64(i)
		if i%2 == 0 {
			in.Status = "failed"
		} else {
			in.Status = "passed"
		}
		requireMaterials(t, f, "POST", qualityPath, in, 201)
	}
	current, err := production.GetLotQuality(context.Background(), f.db, materialsActor(f), f.device, in.LotID)
	if err != nil || current.Revision != 52 || current.Status != "passed" {
		t.Fatal(current, err)
	}
	history, err := production.LotQualityHistory(context.Background(), f.db, materialsActor(f), f.device, in.LotID, 0)
	if err != nil || len(history) != 50 || history[0].Revision != 1 || history[49].Revision != 50 {
		t.Fatal(history, err)
	}
	history, err = production.LotQualityHistory(context.Background(), f.db, materialsActor(f), f.device, in.LotID, 50)
	if err != nil || len(history) != 2 || history[1].Revision != 52 {
		t.Fatal(history, err)
	}
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality/history?offset=52", nil, 200)
	// A gap in a disposable fixture is not accepted as an ordinary prior revision.
	if _, err := f.db.Exec(`DELETE FROM production_quality_reviews WHERE revision=2`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality", nil, 409)
	requireMaterials(t, f, "GET", lotsPath+"/"+in.LotID+"/quality/history", nil, 409)
	in.OperationID = "after-gap"
	in.ExpectedRevision = 52
	requireMaterials(t, f, "POST", qualityPath, in, 409)
	unchangedProductionAfterLoss(t, f)
}
