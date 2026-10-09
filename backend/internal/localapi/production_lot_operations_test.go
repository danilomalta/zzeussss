package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/production"
)

func TestHTTPProductionLotReceiptsRemainOriginalAndRestore(t *testing.T) {
	f, in := lotsFixture(t)
	requireMaterials(t, f, "POST", lotsPath, in, 201)
	review := production.QualityInput{OperationID: "review", LotID: in.LotID, ExpectedRevision: 0, Status: "failed", Criterion: "Visual", Reason: "Parecer humano"}
	requireMaterials(t, f, "POST", qualityPath, review, 201)
	second := review
	second.OperationID = "review-second"
	second.ExpectedRevision = 1
	second.Status = "passed"
	requireMaterials(t, f, "POST", qualityPath, second, 201)
	void := production.VoidLotInput{OperationID: "void-loss", LotID: in.LotID, ExpectedRevision: 1, Reason: "Classificacao incorreta"}
	requireMaterials(t, f, "POST", lotsPath+"/void", void, 200)
	replacement := in
	replacement.OperationID = "replace-loss"
	replacement.LotID = "lot-2"
	replacement.QuantityMilli = 27000
	replacement.Code = "REPLACEMENT"
	requireMaterials(t, f, "POST", lotsPath, replacement, 201)
	unchangedProductionAfterLoss(t, f)
	cases := []struct{ kind, op, status string }{{"lot", in.OperationID, "recorded"}, {"lot_void", void.OperationID, "voided"}, {"lot", replacement.OperationID, "recorded"}, {"quality", review.OperationID, "failed"}, {"quality", second.OperationID, "passed"}}
	before := map[string]string{}
	for _, v := range cases {
		b := requireMaterials(t, f, "GET", "/local/v1/production/operations/"+v.kind+"/"+v.op, nil, 200)
		var out production.OperationReceipt
		if json.Unmarshal(b, &out) != nil || !strings.Contains(string(out.Result), `"status":"`+v.status+`"`) {
			t.Fatal(string(b))
		}
		before[v.op] = string(out.Result)
	}
	requireMaterials(t, f, "GET", "/local/v1/production/operations/lot_void/"+in.OperationID, nil, 404)
	materialCount(t, f, "production_lot_events", 3)
	unchangedProductionAfterLoss(t, f)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "loss.tytbak")
	path := filepath.Join(t.TempDir(), "restored.sqlite")
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Verify(ctx, archive, f.device, key); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(ctx, archive, path, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, v := range cases {
		out, err := production.GetOperation(ctx, db, materialsActor(f), f.device, v.kind, v.op)
		if err != nil || string(out.Result) != before[v.op] {
			t.Fatal(v, err, string(out.Result))
		}
	}
}
func TestHTTPProductionLotReceiptsRejectForeignOperatorAndCorruption(t *testing.T) {
	for _, kind := range []string{"lot", "lot_void", "quality"} {
		for _, field := range []string{"actor_id", "request_json", "result_json"} {
			t.Run(kind+field, func(t *testing.T) {
				f, review, lot := qualityFixture(t)
				table := "production_lot_events"
				op := lot.OperationID
				if kind == "lot_void" {
					in := production.VoidLotInput{OperationID: "void", LotID: lot.LotID, ExpectedRevision: 1, Reason: "Corrigir"}
					requireMaterials(t, f, "POST", lotsPath+"/void", in, 200)
					op = in.OperationID
				}
				if kind == "quality" {
					requireMaterials(t, f, "POST", qualityPath, review, 201)
					table = "production_quality_reviews"
					op = review.OperationID
				}
				if _, err := f.db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
					t.Fatal(err)
				}
				value := "foreign"
				want := 404
				if field != "actor_id" {
					value = `{"operation_id":"wrong"}`
					want = 409
				}
				if _, err := f.db.Exec("UPDATE "+table+" SET "+field+"=? WHERE operation_id=?", value, op); err != nil {
					t.Fatal(err)
				}
				requireMaterials(t, f, "GET", "/local/v1/production/operations/"+kind+"/"+op, nil, want)
			})
		}
	}
}
