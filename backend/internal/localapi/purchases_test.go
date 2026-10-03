package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/replenishment"
)

func purchaseFixture(t *testing.T) (*httpContractFixture, string) {
	t.Helper()
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory, modules.Orders}), 200)
	status, body := request(t, f.app, "POST", "/local/v1/products", testProductBody, f.token)
	if status != 201 {
		t.Fatalf("product %d %s", status, body)
	}
	var product struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal(body, &product); e != nil {
		t.Fatal(e)
	}
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	ctx := context.Background()
	if _, e := replenishment.SetPolicy(ctx, f.db, a, f.device, replenishment.PolicyInput{OperationID: "policy", ProductID: product.ID, MinimumMilli: 1000, TargetMilli: 5000}); e != nil {
		t.Fatal(e)
	}
	suggestion, e := replenishment.Suggest(ctx, f.db, a, f.device, replenishment.SuggestInput{OperationID: "suggest", ProductID: product.ID})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = replenishment.Review(ctx, f.db, a, f.device, replenishment.ReviewInput{OperationID: "review", SuggestionID: suggestion.SuggestionID, Decision: "approved", Reason: "reposicao"}); e != nil {
		t.Fatal(e)
	}
	status, body = request(t, f.app, "POST", "/local/v1/purchase-suppliers", `{"operation_id":"supplier-op","id":"supplier","name":"Padaria","status":"active"}`, f.token)
	if status != 201 {
		t.Fatalf("supplier %d %s", status, body)
	}
	return f, suggestion.SuggestionID
}

func TestHTTPPurchaseExpiredContractPreservesReadAndBlocksReplay(t *testing.T) {
	f, s := purchaseFixture(t)
	status, _ := request(t, f.app, "POST", "/local/v1/purchase-orders", purchaseBody(s), f.token)
	if status != 201 {
		t.Fatal(status)
	}
	now := time.Now().Unix()
	payload, e := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Orders}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); e != nil {
		t.Fatal(e)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/purchase-orders", purchaseBody(s), f.token)
	if status != 403 {
		t.Fatalf("expired replay %d", status)
	}
	status, body := request(t, f.app, "GET", "/local/v1/purchase-orders/order", "", f.token)
	if status != 200 {
		t.Fatalf("expired read %d %s", status, body)
	}
	purchaseCounts(t, f, 1, 1, 2)
}

func TestHTTPPurchaseSupplierReplayConflictAndAuditRollback(t *testing.T) {
	f, _ := purchaseFixture(t)
	input := `{"operation_id":"supplier-op","id":"supplier","name":"Padaria","status":"active"}`
	status, _ := request(t, f.app, "POST", "/local/v1/purchase-suppliers", input, f.token)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/purchase-suppliers", `{"operation_id":"supplier-op","id":"supplier","name":"Other","status":"active"}`, f.token)
	if status != 409 {
		t.Fatal(status)
	}
	if _, e := f.db.Exec(`CREATE TRIGGER fail_supplier BEFORE INSERT ON purchase_audit BEGIN SELECT RAISE(IGNORE); END`); e != nil {
		t.Fatal(e)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/purchase-suppliers", `{"operation_id":"new-op","id":"new","name":"Other","status":"active"}`, f.token)
	if status == 201 || status == 200 {
		t.Fatal(status)
	}
	var n int
	if e := f.db.QueryRow(`SELECT COUNT(*) FROM purchase_suppliers`).Scan(&n); e != nil || n != 1 {
		t.Fatalf("partial supplier %d %v", n, e)
	}
	noVerifier, e := New(f.db, f.device)
	if e != nil {
		t.Fatal(e)
	}
	status, _ = request(t, noVerifier, "POST", "/local/v1/purchase-suppliers", input, f.token)
	if status != 503 {
		t.Fatal(status)
	}
}
func purchaseBody(suggestion string) string {
	b, _ := json.Marshal(purchases.Input{OperationID: "order-op", OrderID: "order", SupplierID: "supplier", SuggestionID: suggestion})
	return string(b)
}
func purchaseCounts(t *testing.T, f *httpContractFixture, orders, items, audit int) {
	t.Helper()
	for _, v := range []struct {
		table string
		want  int
	}{{"purchase_orders", orders}, {"purchase_order_items", items}, {"purchase_audit", audit}, {"stock_movements", 0}, {"cash_movements", 0}, {"sale_payments", 0}, {"outbox", 3}} {
		var n int
		if e := f.db.QueryRow("SELECT COUNT(*) FROM " + v.table).Scan(&n); e != nil || n != v.want {
			t.Fatalf("%s %d want %d err %v", v.table, n, v.want, e)
		}
	}
}
func TestHTTPPurchaseApprovalSnapshotAndReplay(t *testing.T) {
	f, suggestion := purchaseFixture(t)
	input := purchaseBody(suggestion)
	for _, want := range []int{201, 200} {
		status, body := request(t, f.app, "POST", "/local/v1/purchase-orders", input, f.token)
		if status != want {
			t.Fatalf("create %d %s", status, body)
		}
	}
	purchaseCounts(t, f, 1, 1, 2)
	if _, e := f.db.Exec(`UPDATE products SET name='Changed',sku='Changed',unit='kg'`); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE restock_suggestions SET recommended_milli=9999`); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE purchase_suppliers SET name='Changed'`); e != nil {
		t.Fatal(e)
	}
	status, body := request(t, f.app, "GET", "/local/v1/purchase-orders/order", "", f.token)
	var order purchases.Order
	if e := json.Unmarshal(body, &order); e != nil || status != 200 || len(order.Items) != 1 || order.Items[0].Name != "Arroz" || order.Items[0].Quantity != 5000 || order.SupplierName != "Padaria" || order.Status != "local_not_sent" {
		t.Fatalf("snapshot %d %s %v", status, body, e)
	}
	status, body = request(t, f.app, "GET", "/local/v1/purchase-orders?offset=1", "", f.token)
	if status != 200 || string(body) != "{\"items\":[]}" {
		t.Fatalf("pagination %d %s", status, body)
	}
	status, body = request(t, f.app, "GET", "/local/v1/purchase-approvals", "", f.token)
	if status != 200 || string(body) != "{\"items\":[]}" {
		t.Fatalf("consumed %d %s", status, body)
	}
}
func TestHTTPPurchaseRejectsMissingRejectedInactiveAndAmbiguous(t *testing.T) {
	for _, kind := range []string{"missing", "rejected", "inactive", "duplicate-json", "foreign-field"} {
		t.Run(kind, func(t *testing.T) {
			f, s := purchaseFixture(t)
			in := purchaseBody(s)
			want := 409
			switch kind {
			case "missing":
				in = purchaseBody("missing")
			case "rejected":
				if _, e := f.db.Exec(`UPDATE restock_reviews SET decision='rejected'`); e != nil {
					t.Fatal(e)
				}
			case "inactive":
				if _, e := f.db.Exec(`UPDATE purchase_suppliers SET status='inactive'`); e != nil {
					t.Fatal(e)
				}
			case "duplicate-json":
				in = `{"operation_id":"x","operation_id":"y","order_id":"order","supplier_id":"supplier","suggestion_id":"s"}`
				want = 400
			case "foreign-field":
				in = `{"operation_id":"x","order_id":"order","supplier_id":"supplier","suggestion_id":"s","tenant_id":"other"}`
				want = 400
			}
			status, body := request(t, f.app, "POST", "/local/v1/purchase-orders", in, f.token)
			if status != want {
				t.Fatalf("%d %s", status, body)
			}
			purchaseCounts(t, f, 0, 0, 1)
		})
	}
}
func TestHTTPPurchaseConflictsAndConcurrentRequests(t *testing.T) {
	f, s := purchaseFixture(t)
	in := purchaseBody(s)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, body := request(t, f.app, "POST", "/local/v1/purchase-orders", in, f.token)
			if status != 201 && status != 200 {
				t.Errorf("concurrent %d %s", status, body)
			}
		}()
	}
	wg.Wait()
	purchaseCounts(t, f, 1, 1, 2)
	for _, in := range []purchases.Input{{OperationID: "order-op", OrderID: "other", SupplierID: "supplier", SuggestionID: s}, {OperationID: "other", OrderID: "other", SupplierID: "supplier", SuggestionID: s}} {
		body, _ := json.Marshal(in)
		status, b := request(t, f.app, "POST", "/local/v1/purchase-orders", string(body), f.token)
		if status != 409 {
			t.Fatalf("conflict %d %s", status, b)
		}
	}
}
func TestHTTPPurchaseFailureRollsBackAllWritesAndClock(t *testing.T) {
	for _, table := range []string{"purchase_orders", "purchase_order_items", "purchase_audit"} {
		for _, action := range []string{"ABORT,'test'", "IGNORE"} {
			t.Run(table+action, func(t *testing.T) {
				f, s := purchaseFixture(t)
				if _, e := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=1`); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec("CREATE TRIGGER fail_purchase BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(" + action + "); END"); e != nil {
					t.Fatal(e)
				}
				status, _ := request(t, f.app, "POST", "/local/v1/purchase-orders", purchaseBody(s), f.token)
				if status == 201 || status == 200 {
					t.Fatalf("successful failure %d", status)
				}
				purchaseCounts(t, f, 0, 0, 1)
				var observed int64
				if e := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&observed); e != nil || observed != 1 {
					t.Fatalf("clock %d %v", observed, e)
				}
			})
		}
	}
}
func TestHTTPPurchaseAuthorizationAndCrossStore(t *testing.T) {
	for _, kind := range []string{"anonymous", "module", "cashier", "revoked", "device", "other-store"} {
		t.Run(kind, func(t *testing.T) {
			f, s := purchaseFixture(t)
			token := f.token
			want := 403
			switch kind {
			case "anonymous":
				token = ""
				want = 401
			case "module":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory}), 200)
			case "cashier":
				if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
					t.Fatal(e)
				}
			case "revoked":
				if _, e := f.db.Exec(`UPDATE memberships SET status='revoked'`); e != nil {
					t.Fatal(e)
				}
				want = 401
			case "device":
				if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); e != nil {
					t.Fatal(e)
				}
				want = 401
			case "other-store":
				if _, e := f.db.Exec(`INSERT INTO stores VALUES(?,'other','Other')`, f.owner.TenantID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`INSERT INTO devices VALUES(?,'other','other-device','Other')`, f.owner.TenantID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`UPDATE purchase_suppliers SET store_id='other',device_id='other-device'`); e != nil {
					t.Fatal(e)
				}
				want = 409
			}
			status, body := request(t, f.app, "POST", "/local/v1/purchase-orders", purchaseBody(s), token)
			if status != want {
				t.Fatalf("access %d want %d %s", status, want, body)
			}
			purchaseCounts(t, f, 0, 0, 1)
		})
	}
}
func TestHTTPPurchasePersistsAfterReopen(t *testing.T) {
	f, s := purchaseFixture(t)
	status, _ := request(t, f.app, "POST", "/local/v1/purchase-orders", purchaseBody(s), f.token)
	if status != 201 {
		t.Fatal(status)
	}
	var path string
	var seq int
	var name string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	reopened, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	v, e := purchases.Get(context.Background(), reopened, a, f.device, "order")
	if e != nil || v.ID != "order" || len(v.Items) != 1 {
		t.Fatalf("reopen %+v %v", v, e)
	}
	foreign := a
	foreign.TenantID = "foreign"
	if _, e = purchases.Get(context.Background(), reopened, foreign, f.device, "order"); e == nil {
		t.Fatal("foreign tenant read")
	}
}
