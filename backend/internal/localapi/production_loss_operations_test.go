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

func TestHTTPProductionLossReceiptsRemainOriginalAndRestore(t *testing.T) {
	f, in := lossesFixture(t)
	requireMaterials(t, f, "POST", lossesPath, in, 201)
	void := production.VoidLossInput{OperationID: "void-loss", LossID: in.LossID, ExpectedRevision: 1, Reason: "Classificacao incorreta"}
	requireMaterials(t, f, "POST", lossesPath+"/void", void, 200)
	replacement := in
	replacement.OperationID = "replace-loss"
	replacement.LossID = "loss-2"
	replacement.QuantityMilli = 3000
	requireMaterials(t, f, "POST", lossesPath, replacement, 201)
	unchangedProductionAfterLoss(t, f)
	cases := []struct{ kind, op, status string }{{"loss", in.OperationID, "recorded"}, {"loss_void", void.OperationID, "voided"}, {"loss", replacement.OperationID, "recorded"}}
	before := map[string]string{}
	for _, v := range cases {
		b := requireMaterials(t, f, "GET", "/local/v1/production/operations/"+v.kind+"/"+v.op, nil, 200)
		var out production.OperationReceipt
		if json.Unmarshal(b, &out) != nil || !strings.Contains(string(out.Result), `"status":"`+v.status+`"`) {
			t.Fatal(string(b))
		}
		before[v.op] = string(out.Result)
	}
	requireMaterials(t, f, "GET", "/local/v1/production/operations/loss_void/"+in.OperationID, nil, 404)
	materialCount(t, f, "production_loss_events", 3)
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
func TestHTTPProductionLossReceiptsRejectForeignOperatorAndCorruption(t *testing.T) {
	for _, field := range []string{"actor_id", "request_json", "result_json"} {
		t.Run(field, func(t *testing.T) {
			f, in := lossesFixture(t)
			requireMaterials(t, f, "POST", lossesPath, in, 201)
			// Deliberate corruption of a disposable fixture, never a real database.
			if _, err := f.db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
				t.Fatal(err)
			}
			value := "foreign"
			want := 404
			if field != "actor_id" {
				value = `{"operation_id":"wrong"}`
				want = 409
			}
			if _, err := f.db.Exec("UPDATE production_loss_events SET "+field+"=? WHERE operation_id=?", value, in.OperationID); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "GET", "/local/v1/production/operations/loss/"+in.OperationID, nil, want)
		})
	}
}
