package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/purchases"
)

const purchaseTracePath = "/local/v1/purchase-orders/order/trace"

func purchaseTraceFixture(t *testing.T) (*httpContractFixture, string) {
	t.Helper()
	f, s := purchaseFixture(t)
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 201)
	return f, s
}
func purchaseTraceResponse(t *testing.T, f *httpContractFixture) purchases.OrderTrace {
	t.Helper()
	b := requireMaterials(t, f, "GET", purchaseTracePath, nil, 200)
	var out purchases.OrderTrace
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err, string(b))
	}
	return out
}
func TestHTTPPurchaseTraceSnapshotsApprovalAuditAndReplay(t *testing.T) {
	f, s := purchaseTraceFixture(t)
	out := purchaseTraceResponse(t, f)
	v := out.Order
	if v.ID != "order" || v.SupplierName != "Padaria" || v.Status != "local_not_sent" || len(v.Items) != 1 || v.Items[0].Name != "Arroz" || v.Items[0].Unit != "unit" || v.Items[0].Quantity != 5000 {
		t.Fatal(out)
	}
	if out.Creation.Kind != "purchase.created" || out.Creation.OperationID != "order-op" || out.Creation.ActorID != f.owner.OwnerID || out.Creation.DeviceID != f.device.DeviceID || out.Creation.CreatedAt != v.CreatedAt {
		t.Fatal(out)
	}
	if out.Approval.SuggestionID != s || out.Approval.OperationID != "review" || out.Approval.DeviceID != f.device.DeviceID || out.Approval.ReviewerID != v.ApprovedBy || out.Approval.DecidedAt != v.ApprovedAt || out.Approval.Decision != "approved" || out.Approval.Reason != "reposicao" {
		t.Fatal(out)
	}
	requireMaterials(t, f, "POST", "/local/v1/purchase-orders", json.RawMessage(purchaseBody(s)), 200)
	for _, q := range []string{`UPDATE products SET name='Changed',sku='Changed',unit='kg'`, `UPDATE purchase_suppliers SET name='Changed',status='inactive'`, `UPDATE restock_suggestions SET recommended_milli=9999`} {
		if _, err := f.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	before := purchaseSearchState(t, f)
	for i := 0; i < 3; i++ {
		if got := purchaseTraceResponse(t, f); !reflect.DeepEqual(out, got) {
			t.Fatal(got)
		}
	}
	if !reflect.DeepEqual(before, purchaseSearchState(t, f)) {
		t.Fatal("read changes records or clock")
	}
	purchaseCounts(t, f, 1, 1, 2)
	// Numeric scale stays attached to each existing unit; never convert in a trace.
	for _, unit := range []string{"unit", "kg", "g", "liter", "ml", "meter"} {
		if _, err := f.db.Exec(`UPDATE purchase_order_items SET unit=?,quantity_milli=?`, unit, purchases.MaxQuantity); err != nil {
			t.Fatal(err)
		}
		got := purchaseTraceResponse(t, f)
		if got.Order.Items[0].Unit != unit || got.Order.Items[0].Quantity != purchases.MaxQuantity {
			t.Fatal(got)
		}
	}
}
func TestHTTPPurchaseTraceStrictPathQueryAuthorizationAndExpiry(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	for _, p := range []string{purchaseTracePath + "?offset=0", purchaseTracePath + "?tenant_id=other", purchaseTracePath + "?store_id=other", "/local/v1/purchase-orders/%00/trace", "/local/v1/purchase-orders/order%0A/trace", "/local/v1/purchase-orders/%20order/trace", "/local/v1/purchase-orders/" + strings.Repeat("x", 129) + "/trace"} {
		requireMaterials(t, f, "GET", p, nil, 400)
	}
	requireMaterials(t, f, "GET", "/local/v1/purchase-orders/missing/trace", nil, 404)
	status, _ := request(t, f.app, "GET", purchaseTracePath, "", "")
	if status != 401 {
		t.Fatal(status)
	}
	for _, kind := range []string{"tenant", "store", "device", "actor"} {
		a := materialsActor(f)
		d := f.device
		switch kind {
		case "tenant":
			a.TenantID = "other"
		case "store":
			a.StoreID = "other"
		case "device":
			d.DeviceID = "other"
		case "actor":
			a.IdentityID = "other"
		}
		if _, err := purchases.Trace(context.Background(), f.db, a, d, "order"); err == nil {
			t.Fatal("cross scope", kind)
		}
	}
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Orders}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	before := purchaseSearchState(t, f)
	purchaseTraceResponse(t, f)
	if !reflect.DeepEqual(before, purchaseSearchState(t, f)) {
		t.Fatal("expired read changes clock")
	}
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	status, body := request(t, app, "GET", purchaseTracePath, "", f.token)
	if status != 200 {
		t.Fatal(status, string(body))
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"manager", "stock", "production", "supplier", "cashier", "accountant", "employee"} {
		if _, err = f.db.Exec(`UPDATE memberships SET role=?`, role); err != nil {
			t.Fatal(err)
		}
		want := 200
		if role == "cashier" || role == "accountant" || role == "employee" {
			want = 403
		}
		requireMaterials(t, f, "GET", purchaseTracePath, nil, want)
	}
}
func TestHTTPPurchaseTraceRejectsBrokenLinksWithoutPartialResponse(t *testing.T) {
	for _, q := range []string{
		`DELETE FROM purchase_audit WHERE kind='purchase.created'`,
		`UPDATE purchase_audit SET operation_id='wrong' WHERE kind='purchase.created'`,
		`UPDATE purchase_audit SET created_at='wrong' WHERE kind='purchase.created'`,
		`UPDATE restock_reviews SET decision='rejected'`,
		`UPDATE restock_reviews SET decided_at='wrong'`,
		`DELETE FROM purchase_order_items`,
		`UPDATE purchase_order_items SET unit='unknown'`,
		`INSERT INTO purchase_audit SELECT tenant_id,store_id,device_id,'extra',kind,aggregate_id,actor_id,created_at FROM purchase_audit WHERE kind='purchase.created'`,
	} {
		t.Run(q, func(t *testing.T) {
			f, _ := purchaseTraceFixture(t)
			if _, err := f.db.Exec(q); err != nil {
				t.Fatal(err)
			}
			b := requireMaterials(t, f, "GET", purchaseTracePath, nil, 409)
			if strings.Contains(string(b), "supplier_name") {
				t.Fatal("partial trace", string(b))
			}
		})
	}
}
func TestHTTPPurchaseTraceBackupRestoresApprovalAndAudit(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	before := purchaseTraceResponse(t, f)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "trace.tytbak")
	restored := filepath.Join(dir, "restored.sqlite")
	ctx := context.Background()
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err := backup.Verify(ctx, archive, f.device, key); err != nil {
		t.Fatal(err)
	}
	if err := backup.Restore(ctx, archive, restored, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	out, err := purchases.Trace(ctx, db, materialsActor(f), f.device, "order")
	if err != nil || !reflect.DeepEqual(before, out) {
		t.Fatal(out, err)
	}
}
func TestHTTPPurchaseTraceConcurrentReplayKeepsSingleEvidence(t *testing.T) {
	f, s := purchaseTraceFixture(t)
	before := purchaseTraceResponse(t, f)
	var wg sync.WaitGroup
	wg.Add(1)
	result := make(chan error, 1)
	go func() {
		defer wg.Done()
		status, body := request(t, f.app, "POST", "/local/v1/purchase-orders", purchaseBody(s), f.token)
		if status != 200 {
			result <- fmt.Errorf("replay %d %s", status, body)
		} else {
			result <- nil
		}
	}()
	for i := 0; i < 12; i++ {
		out := purchaseTraceResponse(t, f)
		if !reflect.DeepEqual(before, out) {
			t.Fatal(out)
		}
	}
	wg.Wait()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	purchaseCounts(t, f, 1, 1, 2)
}

func TestHTTPPurchaseTraceIsolatesSameIDsAcrossTenants(t *testing.T) {
	f, _ := purchaseTraceFixture(t)
	before := purchaseTraceResponse(t, f)
	// Clone only disposable commercial fixtures, keeping local IDs equal.
	if _, err := f.db.Exec(`INSERT INTO tenants SELECT 'foreign-tenant','Foreign',created_at FROM tenants WHERE id=?`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"stores", "devices", "memberships", "membership_stores", "products", "restock_policies", "restock_suggestions", "restock_reviews", "purchase_suppliers", "purchase_orders", "purchase_order_items", "purchase_audit"} {
		rows, err := f.db.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			t.Fatal(err)
		}
		columns := []string{}
		for rows.Next() {
			var cid, notnull, pk int
			var name, typ string
			var defaultValue any
			if err = rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if name == "tenant_id" {
				columns = append(columns, "'foreign-tenant'")
			} else {
				columns = append(columns, name)
			}
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		if _, err = f.db.Exec("INSERT INTO "+table+" SELECT "+strings.Join(columns, ",")+" FROM "+table+" WHERE tenant_id=?", f.owner.TenantID); err != nil {
			t.Fatal(table, err)
		}
	}
	if _, err := f.db.Exec(`UPDATE purchase_order_items SET name='Foreign' WHERE tenant_id='foreign-tenant'`); err != nil {
		t.Fatal(err)
	}
	if got := purchaseTraceResponse(t, f); !reflect.DeepEqual(before, got) {
		t.Fatal("tenant leak", got)
	}
	a := materialsActor(f)
	a.TenantID = "foreign-tenant"
	if _, err := purchases.Trace(context.Background(), f.db, a, f.device, "order"); err == nil {
		t.Fatal("foreign tenant with current device")
	}
}
