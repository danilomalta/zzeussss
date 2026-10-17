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

func TestHTTPProductionStageReceiptsRemainOriginalAndRestore(t *testing.T) {
	f, in, reserve := stagesFixture(t)
	requireMaterials(t, f, "POST", stagePlansPath, in, 201)
	consumeStages(t, f, reserve)
	finishStages(t, f, in)
	requireMaterials(t, f, "POST", resultsPath, stageCompletion(in), 201)
	cases := []struct{ kind, op, status string }{{"stage_plan", in.OperationID, "configured"}, {"stage_state", "mix-running", "running"}, {"stage_state", "mix-completed", "completed"}, {"stage_state", "bake-running", "running"}, {"stage_state", "bake-completed", "completed"}}
	before := map[string]string{}
	for _, v := range cases {
		b := requireMaterials(t, f, "GET", "/local/v1/production/operations/"+v.kind+"/"+v.op, nil, 200)
		var out production.OperationReceipt
		if json.Unmarshal(b, &out) != nil || !strings.Contains(string(out.Result), `"status":"`+v.status+`"`) {
			t.Fatal(string(b))
		}
		before[v.op] = string(out.Result)
	}
	materialCount(t, f, "production_stage_events", 5)
	materialCount(t, f, "stock_movements", 5)
	requireMaterials(t, f, "GET", "/local/v1/production/operations/stage_state/"+in.OperationID, nil, 404)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "stages.tytbak")
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
func TestHTTPProductionStageReceiptsRejectCorruptionAndForeignOperator(t *testing.T) {
	for _, field := range []string{"actor_id", "request_json", "result_json"} {
		t.Run(field, func(t *testing.T) {
			f, in, _ := stagesFixture(t)
			requireMaterials(t, f, "POST", stagePlansPath, in, 201)
			// Only this disposable fixture is intentionally corrupted.
			if _, err := f.db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
				t.Fatal(err)
			}
			value := "foreign"
			want := 404
			if field != "actor_id" {
				value = `{"operation_id":"wrong"}`
				want = 409
			}
			if _, err := f.db.Exec("UPDATE production_stage_events SET "+field+"=? WHERE operation_id=?", value, in.OperationID); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "GET", "/local/v1/production/operations/stage_plan/"+in.OperationID, nil, want)
		})
	}
}
