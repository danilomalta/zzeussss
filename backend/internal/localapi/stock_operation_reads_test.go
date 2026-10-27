package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/stock"
)

func stockReadRequest(t *testing.T, f *stockHTTPFixture, path string, want int) []byte {
	t.Helper()
	status, b := request(t, f.app, "GET", path, "", f.token)
	if status != want {
		t.Fatalf("%s: %d want %d %s", path, status, want, b)
	}
	return b
}
func TestHTTPStockOperationsOriginalReceiptsHistoryRestore(t *testing.T) {
	f := stockHTTPSetup(t)
	entry := stockEntry(f, "entry-original", 10000)
	stockRequest(t, f, entry, 201)
	first := stockReadRequest(t, f, "/local/v1/stock/operations/entry-original/receipt", 200)
	transfer := stock.Input{OperationID: "transfer-original", Kind: "transfer", ProductID: f.product, FromLocationID: f.back, ToLocationID: f.shelf, QuantityMilli: 3000, Reason: "Abastecer gondola"}
	stockRequest(t, f, transfer, 201)
	loss := stock.Input{OperationID: "loss-original", Kind: "loss", ProductID: f.product, FromLocationID: f.shelf, QuantityMilli: 1000, Reason: "Avaria"}
	stockRequest(t, f, loss, 201)
	assertStockBalance(t, f, f.back, 7000)
	assertStockBalance(t, f, f.shelf, 2000)
	for _, in := range []stock.Input{entry, transfer, loss} {
		stockRequest(t, f, in, 200)
	}
	assertStockRows(t, f, 3, 4, 3)
	if string(stockReadRequest(t, f, "/local/v1/stock/operations/entry-original/receipt", 200)) != string(first) {
		t.Fatal("receipt changed with balance")
	}
	for _, check := range []struct {
		location string
		want     int64
	}{{f.back, 2}, {f.shelf, 2}} {
		b := stockReadRequest(t, f, "/local/v1/stock/operations/history?product_id="+f.product+"&location_id="+check.location, 200)
		var v stock.OperationHistory
		if json.Unmarshal(b, &v) != nil || v.TotalCount != check.want || len(v.Items) != 2 || v.HasMore {
			t.Fatal(string(b))
		}
	}
	assertStockRows(t, f, 3, 4, 3)
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "stock.tytbak")
	target := filepath.Join(t.TempDir(), "restored.sqlite")
	if e := backup.Create(context.Background(), f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(context.Background(), archive, target, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), target)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	v, e := stock.Receipt(context.Background(), db, materialsActor(f.httpContractFixture), f.device, "entry-original")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(v)
	if string(b) != string(first) {
		t.Fatal("restore changed receipt", string(b), string(first))
	}
	h, e := stock.History(context.Background(), db, materialsActor(f.httpContractFixture), f.device, f.product, f.shelf, 0)
	if e != nil || h.TotalCount != 2 {
		t.Fatal(h, e)
	}
}
func TestHTTPStockOperationsPagesQueriesAndIsolation(t *testing.T) {
	f := stockHTTPSetup(t)
	for n := 0; n < 51; n++ {
		stockRequest(t, f, stockEntry(f, fmt.Sprintf("entry-%03d", n), 1000), 201)
	}
	path := "/local/v1/stock/operations/history?product_id=" + f.product + "&location_id=" + f.back
	for _, c := range []struct {
		offset int
		count  int
		more   bool
	}{{0, 50, true}, {50, 1, false}, {100, 0, false}} {
		b := stockReadRequest(t, f, fmt.Sprintf("%s&offset=%d", path, c.offset), 200)
		var h stock.OperationHistory
		if json.Unmarshal(b, &h) != nil || h.TotalCount != 51 || len(h.Items) != c.count || h.HasMore != c.more {
			t.Fatal(string(b))
		}
	}
	for _, suffix := range []string{"&offset=-1", "&offset=1e3", "&offset=1&offset=2", "&actor_id=other", "&offset=9007199254740992"} {
		stockReadRequest(t, f, path+suffix, 400)
	}
	stockReadRequest(t, f, "/local/v1/stock/operations/entry-000/receipt?actor_id=other", 400)
	stockReadRequest(t, f, "/local/v1/stock/operations/missing/receipt", 404)
	stockRequest(t, f, stockEntry(f, "history", 1000), 201)
	stockReadRequest(t, f, "/local/v1/stock/operations/history/receipt", 200)
	stockReadRequest(t, f, path+"x", 404)
	// Direct calls also require matching company/store/operator/device authorization.
	a := materialsActor(f.httpContractFixture)
	for _, field := range []string{"tenant", "store", "actor"} {
		foreign := a
		switch field {
		case "tenant":
			foreign.TenantID = "foreign"
		case "store":
			foreign.StoreID = "foreign"
		case "actor":
			foreign.IdentityID = "foreign"
		}
		if _, e := stock.Receipt(context.Background(), f.db, foreign, f.device, "entry-000"); e == nil {
			t.Fatal("foreign receipt", field)
		}
		if _, e := stock.History(context.Background(), f.db, foreign, f.device, f.product, f.back, 0); e == nil {
			t.Fatal("foreign history", field)
		}
	}
	if _, e := f.db.Exec(`PRAGMA foreign_keys=OFF`); e != nil {
		t.Fatal(e)
	}
	for _, column := range []string{"device_id", "actor_identity_id", "store_id", "tenant_id"} {
		if _, e := f.db.Exec("UPDATE stock_operations SET " + column + "='foreign' WHERE id='entry-000'"); e != nil {
			t.Fatal(e)
		}
		stockReadRequest(t, f, "/local/v1/stock/operations/entry-000/receipt", 404)
		original := map[string]string{"device_id": f.device.DeviceID, "actor_identity_id": a.IdentityID, "store_id": a.StoreID, "tenant_id": a.TenantID}[column]
		if _, e := f.db.Exec("UPDATE stock_operations SET "+column+"=? WHERE id='entry-000'", original); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, a.TenantID, a.IdentityID, a.StoreID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
		t.Fatal(e)
	}
	stockReadRequest(t, f, path, 403)
	stockReadRequest(t, f, "/local/v1/stock/operations/entry-000/receipt", 403)
	assertStockRows(t, f, 52, 52, 52)
}
func TestHTTPStockOperationsRespectReservedBalance(t *testing.T) {
	f, reserve := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reserve, 201)
	before := traceReadState(t, f)
	for _, kind := range []string{"loss", "transfer"} {
		in := stock.Input{OperationID: "blocked-" + kind, Kind: kind, ProductID: "flour", FromLocationID: "production-room", QuantityMilli: 500001, Reason: "Teste"}
		if kind == "transfer" {
			in.ToLocationID = "missing"
		}
		requireMaterials(t, f, "POST", "/local/v1/stock/operations", in, 409)
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("refusal changed reservations or stock")
	}
	in := stock.Input{OperationID: "free-loss", Kind: "loss", ProductID: "flour", FromLocationID: "production-room", QuantityMilli: 500000, Reason: "Avaria livre"}
	requireMaterials(t, f, "POST", "/local/v1/stock/operations", in, 201)
	availabilityResponse(t, f, flourAvailability, 1500000, 1500000, 0)
	requireMaterials(t, f, "GET", "/local/v1/stock/operations/free-loss/receipt", nil, 200)
	requireMaterials(t, f, "POST", "/local/v1/stock/operations", in, 200)
	availabilityResponse(t, f, flourAvailability, 1500000, 1500000, 0)
}

func TestHTTPStockOperationsRejectChangedExplicitUnitAndReplay(t *testing.T) {
	f := stockHTTPSetup(t)
	in := stockEntry(f, "unit-bound", 1000)
	in.ExpectedUnit = "g"
	stockRequest(t, f, in, 409)
	assertStockRows(t, f, 0, 0, 0)
	var current string
	if e := f.db.QueryRow(`SELECT unit FROM products WHERE id=?`, f.product).Scan(&current); e != nil {
		t.Fatal(e)
	}
	in.ExpectedUnit = current
	stockRequest(t, f, in, 201)
	stockRequest(t, f, in, 200)
	var payload string
	if e := f.db.QueryRow(`SELECT payload_json FROM outbox WHERE operation_id=?`, in.OperationID).Scan(&payload); e != nil {
		t.Fatal(e)
	}
	var event map[string]any
	if json.Unmarshal([]byte(payload), &event) != nil {
		t.Fatal(payload)
	}
	if _, present := event["expected_unit"]; present {
		t.Fatal("event contract changed")
	}
	in.ExpectedUnit = "invalid-unit"
	stockRequest(t, f, in, 400)
	assertStockRows(t, f, 1, 1, 1)
}
