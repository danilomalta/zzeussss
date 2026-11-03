package localapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/purchases"
)

const receivingPath = "/local/v1/purchase-orders/order/receipts"

func receivingFixture(t *testing.T) *httpContractFixture {
	t.Helper()
	f, _ := purchaseTraceFixture(t)
	requireMaterials(t, f, "POST", receivingAuthPath, receivingAuthBody(), 200)
	if _, err := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'receiving-room','receiving','Conferencia')`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	return f
}
func receivingBody() map[string]any {
	return map[string]any{"operation_id": "receipt-op", "delivery_reference": "delivery-1", "location_id": "receiving-room", "unit": "unit", "delivered_milli": 2500, "accepted_milli": 2000, "reason": "500 recusados na conferencia"}
}
func receivedResponse(t *testing.T, f *httpContractFixture, in map[string]any, want int) purchases.ReceiptResult {
	t.Helper()
	b := requireMaterials(t, f, "POST", receivingPath, in, want)
	var out purchases.ReceiptResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err, string(b))
	}
	return out
}
func TestHTTPPurchaseReceivingPartialExactReplayAndRestore(t *testing.T) {
	f := receivingFixture(t)
	in := receivingBody()
	first := receivedResponse(t, f, in, 201)
	if first.Repeated || first.Receipt.AcceptedMilli != 2000 || first.Receipt.DeliveredMilli != 2500 || first.Receipt.ActorID != f.owner.OwnerID {
		t.Fatal(first)
	}
	if purchaseTraceResponse(t, f).Order.ReceivingStatus != "partially_received" {
		t.Fatal("partial status")
	}
	next := receivingBody()
	next["operation_id"] = "receipt-2"
	next["delivery_reference"] = "delivery-2"
	next["delivered_milli"] = 3000
	next["accepted_milli"] = 3000
	receivedResponse(t, f, next, 201)
	replay := receivedResponse(t, f, in, 200)
	replay.Repeated = false
	if !reflect.DeepEqual(first, replay) {
		t.Fatal(first, replay)
	}
	if purchaseTraceResponse(t, f).Order.ReceivingStatus != "received" || purchaseTraceResponse(t, f).Order.Status != "local_not_sent" {
		t.Fatal("incorrect status")
	}
	materialCount(t, f, "purchase_receipts", 2)
	materialCount(t, f, "stock_operations", 2)
	materialCount(t, f, "stock_movements", 2)
	var balance int64
	if err := f.db.QueryRow(`SELECT SUM(quantity_milli) FROM stock_movements`).Scan(&balance); err != nil || balance != 5000 {
		t.Fatal(balance, err)
	}
	next["operation_id"] = "extra"
	next["delivery_reference"] = "extra"
	next["accepted_milli"] = 1
	next["delivered_milli"] = 1
	requireMaterials(t, f, "POST", receivingPath, next, 409)
	db := restoredPurchaseDB(t, f)
	got, err := purchases.Trace(context.Background(), db, materialsActor(f), f.device, "order")
	if err != nil || got.Order.ReceivingStatus != "received" {
		t.Fatal(got, err)
	}
	if err = db.QueryRow(`SELECT SUM(quantity_milli) FROM stock_movements`).Scan(&balance); err != nil || balance != 5000 {
		t.Fatal(balance, err)
	}
}
func TestHTTPPurchaseReceivingRollbackEveryWriteAndOverflow(t *testing.T) {
	for _, table := range []string{"stock_operations", "stock_movements", "stock_operation_movements", "purchase_receipts", "purchase_receiving_events", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := receivingFixture(t)
			if _, err := f.db.Exec("CREATE TRIGGER reject_receipt BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "POST", receivingPath, receivingBody(), 409)
			for _, table := range []string{"purchase_receipts", "stock_operations", "stock_movements", "stock_operation_movements"} {
				materialCount(t, f, table, 0)
			}
			if purchaseTraceResponse(t, f).Order.ReceivingStatus != "authorized" {
				t.Fatal("partial transaction")
			}
		})
	}
	f := receivingFixture(t)
	var product string
	if err := f.db.QueryRow(`SELECT product_id FROM purchase_order_items`).Scan(&product); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO stock_movements VALUES('max',?,?,?,?,'receiving-room',?,'prior','now')`, f.owner.TenantID, f.owner.StoreID, f.device.DeviceID, product, int64(math.MaxInt64)); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", receivingPath, receivingBody(), 409)
	materialCount(t, f, "purchase_receipts", 0)
}
func TestHTTPPurchaseReceivingUnitsBoundsReferencesAndAuthorization(t *testing.T) {
	f := receivingFixture(t)
	for _, field := range []string{"tenant_id", "store_id", "product_id", "order_id", "stock_operation_id"} {
		in := receivingBody()
		in[field] = "other"
		requireMaterials(t, f, "POST", receivingPath, in, 400)
	}
	for _, c := range []struct {
		key    string
		value  any
		status int
	}{{"accepted_milli", 0, 400}, {"accepted_milli", -1, 400}, {"accepted_milli", 2501, 400}, {"delivered_milli", int64(purchases.MaxQuantity + 1), 400}, {"accepted_milli", 1.5, 400}, {"unit", "g", 409}, {"unit", "unknown", 400}, {"location_id", "other", 404}, {"reason", "bad\nreason", 400}, {"delivery_reference", " bad", 400}} {
		in := receivingBody()
		in[c.key] = c.value
		requireMaterials(t, f, "POST", receivingPath, in, c.status)
	}
	requireMaterials(t, f, "POST", receivingPath+"?tenant_id=other", receivingBody(), 400)
	receivedResponse(t, f, receivingBody(), 201)
	in := receivingBody()
	in["operation_id"] = "same-document"
	requireMaterials(t, f, "POST", receivingPath, in, 409)
	in = receivingBody()
	in["accepted_milli"] = 1999
	requireMaterials(t, f, "POST", receivingPath, in, 409)
	if _, err := f.db.Exec(`UPDATE products SET unit='kg'`); err != nil {
		t.Fatal(err)
	}
	in = receivingBody()
	in["operation_id"] = "unit-change"
	in["delivery_reference"] = "unit-change"
	requireMaterials(t, f, "POST", receivingPath, in, 409)
	// Historical replay remains stable even after catalog changes.
	receivedResponse(t, f, receivingBody(), 200)
	notAuthorized, _ := purchaseTraceFixture(t)
	requireMaterials(t, notAuthorized, "POST", receivingPath, receivingBody(), 409)
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", receivingPath, receivingBody(), 403)
	if status, _ := request(t, f.app, "POST", receivingPath, orderBody(t, receivingBody()), ""); status != 401 {
		t.Fatal(status)
	}
}
func TestHTTPPurchaseReceivingConcurrentRemainingAndReplay(t *testing.T) {
	f := receivingFixture(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	created, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := receivingBody()
			in["operation_id"] = fmt.Sprintf("concurrent-%d", i)
			in["delivery_reference"] = fmt.Sprintf("concurrent-%d", i)
			in["accepted_milli"] = 3000
			in["delivered_milli"] = 3000
			status, body := request(t, f.app, "POST", receivingPath, orderBody(t, in), f.token)
			mu.Lock()
			defer mu.Unlock()
			if status == 201 {
				created++
			} else if status == 409 {
				conflicts++
			} else {
				t.Errorf("%d %s", status, body)
			}
		}(i)
	}
	wg.Wait()
	if created != 1 || conflicts != 1 {
		t.Fatal(created, conflicts)
	}
	materialCount(t, f, "purchase_receipts", 1)
	// Distinct second delivery fills only the remaining quantity, with concurrent exact retries.
	in := receivingBody()
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, body := request(t, f.app, "POST", receivingPath, orderBody(t, in), f.token)
			if status != 201 && status != 200 {
				t.Errorf("%d %s", status, body)
			}
		}()
	}
	wg.Wait()
	materialCount(t, f, "purchase_receipts", 2)
	materialCount(t, f, "stock_movements", 2)
}
