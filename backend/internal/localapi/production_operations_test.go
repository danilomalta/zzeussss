package localapi

import (
	"encoding/json"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/production"
)

func TestHTTPProductionOperationReceiptsRemainOriginalAndNeverWrite(t *testing.T) {
	f, in := orderFixture(t)
	status, b := request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
	if status != 201 {
		t.Fatalf("%d %s", status, b)
	}
	state := production.OrderStateInput{OperationID: "approve-op", OrderID: in.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Plano conferido"}
	status, b = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
	if status != 200 {
		t.Fatalf("%d %s", status, b)
	}
	state.OperationID, state.ExpectedRevision, state.Status = "cancel-op", 2, "cancelled"
	status, b = request(t, f.app, "POST", orderPath+"/state", orderBody(t, state), f.token)
	if status != 200 {
		t.Fatalf("%d %s", status, b)
	}
	for _, v := range []struct{ kind, op, contains string }{{"recipe", "recipe-op", `"version_id":"bread-v1"`}, {"order", in.OperationID, `"status":"planned"`}, {"state", "approve-op", `"status":"approved"`}, {"state", "cancel-op", `"status":"cancelled"`}} {
		status, b = request(t, f.app, "GET", "/local/v1/production/operations/"+v.kind+"/"+v.op, "", f.token)
		var out production.OperationReceipt
		if status != 200 || json.Unmarshal(b, &out) != nil || out.Kind != v.kind || !strings.Contains(string(out.Result), v.contains) {
			t.Fatalf("%s %d %s", v.op, status, b)
		}
	}
	orderCounts(t, f, 1, 3)
	for _, path := range []string{"order/missing", "state/order-op"} {
		status, _ = request(t, f.app, "GET", "/local/v1/production/operations/"+path, "", f.token)
		if status != 404 {
			t.Fatal(path, status)
		}
	}
	for _, path := range []string{"future/order-op", "order/order-op?tenant_id=foreign"} {
		status, _ = request(t, f.app, "GET", "/local/v1/production/operations/"+path, "", f.token)
		if status != 400 {
			t.Fatal(path, status)
		}
	}
	status, _ = request(t, f.app, "GET", "/local/v1/production/operations/order/order-op", "", "")
	if status != 401 {
		t.Fatal(status)
	}
}

func TestHTTPProductionOperationReceiptsAreScopedAndDetectCorruption(t *testing.T) {
	for _, field := range []string{"actor_id", "device_id", "store_id", "request_json", "result_json"} {
		t.Run(field, func(t *testing.T) {
			f, in := orderFixture(t)
			status, b := request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
			if status != 201 {
				t.Fatalf("%d %s", status, b)
			}
			// Corrupt this disposable fixture to verify the scoped lookup never leaks.
			if _, err := f.db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
				t.Fatal(err)
			}
			value := "foreign"
			if field == "request_json" {
				value = `{"operation_id":"wrong"}`
			} else if field == "result_json" {
				value = `{"order_id":"wrong"}`
			}
			if _, err := f.db.Exec("UPDATE production_order_events SET "+field+"=? WHERE operation_id=?", value, in.OperationID); err != nil {
				t.Fatal(err)
			}
			status, _ = request(t, f.app, "GET", "/local/v1/production/operations/order/"+in.OperationID, "", f.token)
			want := 404
			if field == "request_json" || field == "result_json" {
				want = 409
			}
			if status != want {
				t.Fatal(field, status, want)
			}
		})
	}
	f, recipe := recipeFixture(t)
	status, b := request(t, f.app, "POST", recipePath, recipeBody(t, recipe), f.token)
	if status != 201 {
		t.Fatalf("%d %s", status, b)
	}
	if _, err := f.db.Exec(`DELETE FROM production_recipe_audit`); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/production/operations/recipe/recipe-op", "", f.token)
	if status != 409 {
		t.Fatal(status)
	}
}
