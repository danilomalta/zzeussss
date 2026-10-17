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

func TestHTTPProductionExecutionReceiptsKeepOriginalAndRestore(t *testing.T) {
	f, res := materialsFixture(t)
	ctx := context.Background()
	actor := materialsActor(f)
	order, err := production.GetOrder(ctx, f.db, actor, f.device, res.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	state := production.RecipeStateInput{OperationID: "inactive", RecipeID: order.Recipe.RecipeID, ExpectedRevision: 0, Status: "inactive", Reason: "Pausar"}
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", state, 200)
	state.OperationID = "active"
	state.ExpectedRevision = 1
	state.Status = "active"
	requireMaterials(t, f, "POST", "/local/v1/production/recipe-state", state, 200)
	requireMaterials(t, f, "POST", materialsPath, res, 201)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "release", ReservationID: res.ReservationID, Action: "release", Reason: "Liberar"}, 200)
	res.OperationID = "reserve2"
	res.ReservationID = "materials-2"
	requireMaterials(t, f, "POST", materialsPath, res, 201)
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "consume2", ReservationID: res.ReservationID, Action: "consume", Reason: "Executar"}, 200)
	in := production.ResultInput{OperationID: "complete2", ResultID: "result-2", OrderID: res.OrderID, ExpectedRevision: 2, ProducedMilli: 27000, Reason: "Medido"}
	requireMaterials(t, f, "POST", resultsPath, in, 201)
	cases := []struct{ kind, op, status string }{{"recipe_state", "inactive", "inactive"}, {"recipe_state", "active", "active"}, {"reserve", "reserve", "active"}, {"materials", "release", "released"}, {"reserve", "reserve2", "active"}, {"materials", "consume2", "consumed"}, {"result", "complete2", "completed"}}
	before := map[string]string{}
	for _, v := range cases {
		b := requireMaterials(t, f, "GET", "/local/v1/production/operations/"+v.kind+"/"+v.op, nil, 200)
		var out production.OperationReceipt
		if json.Unmarshal(b, &out) != nil || !strings.Contains(string(out.Result), `"status":"`+v.status+`"`) {
			t.Fatal(string(b))
		}
		before[v.op] = string(out.Result)
	}
	materialCount(t, f, "production_material_events", 4)
	materialCount(t, f, "production_results", 1)
	materialCount(t, f, "stock_movements", 5)
	requireMaterials(t, f, "GET", "/local/v1/production/operations/materials/reserve", nil, 404)
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "receipts.tytbak")
	path := filepath.Join(dir, "restored.sqlite")
	if err = backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err = backup.Verify(ctx, archive, f.device, key); err != nil {
		t.Fatal(err)
	}
	if err = backup.Restore(ctx, archive, path, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, v := range cases {
		out, e := production.GetOperation(ctx, db, actor, f.device, v.kind, v.op)
		if e != nil || string(out.Result) != before[v.op] {
			t.Fatal(v, e, string(out.Result))
		}
	}
}
func TestHTTPProductionExecutionReceiptsRejectCorruptionAndForeignOperator(t *testing.T) {
	for _, field := range []string{"actor_id", "request_json", "result_json"} {
		t.Run(field, func(t *testing.T) {
			f, in := resultsFixture(t)
			requireMaterials(t, f, "POST", resultsPath, in, 201)
			if _, err := f.db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
				t.Fatal(err)
			}
			value := "foreign"
			want := 404
			if field != "actor_id" {
				value = `{"operation_id":"wrong"}`
				want = 409
			}
			if _, err := f.db.Exec("UPDATE production_results SET "+field+"=? WHERE operation_id=?", value, in.OperationID); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "GET", "/local/v1/production/operations/result/"+in.OperationID, nil, want)
		})
	}
}
