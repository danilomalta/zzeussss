package localapi

import (
	"encoding/json"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/purchases"
)

const rejectionPath = "/local/v1/purchase-orders/order/rejections"

func rejectionBody() map[string]any {
	return map[string]any{"operation_id": "reject-op", "delivery_reference": "rejected-delivery", "unit": "unit", "delivered_milli": 5000, "reason": "embalagens danificadas: recusa integral"}
}
func TestHTTPPurchaseRejectionReplayNoStockAndBackup(t *testing.T) {
	f := receivingFixture(t)
	first := requireMaterials(t, f, "POST", rejectionPath, rejectionBody(), 201)
	again := requireMaterials(t, f, "POST", rejectionPath, rejectionBody(), 200)
	var a, b purchases.RejectionResult
	if json.Unmarshal(first, &a) != nil || json.Unmarshal(again, &b) != nil || a.Repeated || !b.Repeated || a.Rejection.ID != b.Rejection.ID {
		t.Fatal(string(first), string(again))
	}
	for _, table := range []string{"stock_movements", "stock_operations", "purchase_receipts"} {
		materialCount(t, f, table, 0)
	}
	if purchaseTraceResponse(t, f).Order.ReceivingStatus != "authorized" {
		t.Fatal("refusal reduced remainder")
	}
	receipt := receivingBody()
	receipt["delivery_reference"] = "rejected-delivery"
	requireMaterials(t, f, "POST", receivingPath, receipt, 409)
	in := rejectionBody()
	in["reason"] = "changed"
	requireMaterials(t, f, "POST", rejectionPath, in, 409)
	db := restoredPurchaseDB(t, f)
	var body string
	var n int
	if err := db.QueryRow(`SELECT request_json FROM purchase_delivery_rejections WHERE id=?`, a.Rejection.ID).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM purchase_receiving_exception_events WHERE kind='rejected'`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if body == "" {
		t.Fatal("lost evidence")
	}
}
func TestHTTPPurchaseRejectionRollbackValidationAndCrossReferenceRace(t *testing.T) {
	for _, table := range []string{"purchase_delivery_rejections", "purchase_receiving_exception_events", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := receivingFixture(t)
			if _, err := f.db.Exec("CREATE TRIGGER reject_test BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); err != nil {
				t.Fatal(err)
			}
			requireMaterials(t, f, "POST", rejectionPath, rejectionBody(), 409)
			materialCount(t, f, "purchase_delivery_rejections", 0)
		})
	}
	f := receivingFixture(t)
	for _, c := range []struct {
		k    string
		v    any
		want int
	}{{"delivered_milli", 0, 400}, {"delivered_milli", purchases.MaxQuantity + 1, 400}, {"unit", "kg", 409}, {"unit", "unknown", 400}, {"accepted_milli", 0, 400}, {"tenant_id", "other", 400}, {"reason", "bad\nreason", 400}} {
		in := rejectionBody()
		in[c.k] = c.v
		requireMaterials(t, f, "POST", rejectionPath, in, c.want)
	}
	requireMaterials(t, f, "POST", rejectionPath+"?offset=0", rejectionBody(), 400)
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "POST", rejectionPath, rejectionBody(), 403)
	raced := receivingFixture(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	created, conflicts := 0, 0
	for _, call := range []struct {
		path string
		body map[string]any
	}{{rejectionPath, rejectionBody()}, {receivingPath, receivingBody()}} {
		call.body["delivery_reference"] = "same-delivery"
		wg.Add(1)
		go func(path string, in map[string]any) {
			defer wg.Done()
			status, body := request(t, raced.app, "POST", path, orderBody(t, in), raced.token)
			mu.Lock()
			defer mu.Unlock()
			if status == 201 {
				created++
			} else if status == 409 {
				conflicts++
			} else {
				t.Errorf("%d %s", status, body)
			}
		}(call.path, call.body)
	}
	wg.Wait()
	if created != 1 || conflicts != 1 {
		t.Fatal(created, conflicts)
	}
}
