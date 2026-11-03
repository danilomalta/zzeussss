package localapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stock"
	"titansystem-backend/internal/localdb/stocktaking"
)

func inventoryHTTPJSON(t *testing.T, in any) string {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func inventoryHTTPWorker(t *testing.T, f *stockHTTPFixture) string {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{`INSERT INTO identities VALUES('inventory-worker','Counter','now')`, `INSERT INTO memberships(tenant_id,identity_id,role,status,created_at) VALUES(?, 'inventory-worker','stock','active','now')`, `INSERT INTO membership_stores VALUES(?,'inventory-worker',?)`} {
		var err error
		switch {
		case strings.Contains(q, "membership_stores"):
			_, err = f.db.Exec(q, f.owner.TenantID, f.owner.StoreID)
		case strings.Contains(q, "memberships("):
			_, err = f.db.Exec(q, f.owner.TenantID)
		default:
			_, err = f.db.Exec(q)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	owner := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	if err := localauth.SetPassword(ctx, f.db, owner, f.device, "inventory-worker", "senha-forte-de-teste-1"); err != nil {
		t.Fatal(err)
	}
	return loginToken(t, f.app, "inventory-worker")
}
func inventoryHTTPOpen(t *testing.T, f *stockHTTPFixture, token string) {
	t.Helper()
	in := stocktaking.OpenInput{OperationID: "inv-open", ID: "inv", LocationID: f.shelf, Title: "Physical count", Items: []stocktaking.Item{{ProductID: f.product, Unit: "unit"}}}
	status, body := request(t, f.app, "POST", "/local/v1/stock/inventories", inventoryHTTPJSON(t, in), token)
	if status != 201 {
		t.Fatalf("open %d %s", status, body)
	}
}
func TestReviewedInventoryHTTPEndToEndWithLegitimateSessions(t *testing.T) {
	f := stockHTTPSetup(t)
	worker := inventoryHTTPWorker(t, f)
	stockRequest(t, f, stock.Input{OperationID: "opening", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "Opening"}, 201)
	inventoryHTTPOpen(t, f, worker)
	steps := []struct {
		path  string
		in    any
		token string
		want  int
	}{
		{"/local/v1/stock/inventories/inv/counts", stocktaking.CountInput{OperationID: "inv-count", ExpectedRevision: 1, Items: []stocktaking.CountItem{{ProductID: f.product, CountedMilli: 3000}}}, worker, 200},
		{"/local/v1/stock/inventories/inv/state", stocktaking.StateInput{OperationID: "inv-submit", ExpectedRevision: 2, Action: "submit", Reason: "Count complete"}, worker, 200},
		{"/local/v1/stock/inventories/inv/state", stocktaking.StateInput{OperationID: "inv-approve", ExpectedRevision: 3, Action: "approve", Reason: "Independently checked"}, worker, 403},
		{"/local/v1/stock/inventories/inv/state", stocktaking.StateInput{OperationID: "inv-approve", ExpectedRevision: 3, Action: "approve", Reason: "Independently checked"}, f.token, 200},
	}
	for _, s := range steps {
		status, body := request(t, f.app, "POST", s.path, inventoryHTTPJSON(t, s.in), s.token)
		if status != s.want {
			t.Fatalf("step %s %d %s", s.path, status, body)
		}
	}
	assertStockBalance(t, f, f.shelf, 3000)
	for _, path := range []string{"/stock/inventories", "/stock/inventories?status=approved&limit=1&offset=0", "/stock/inventories/inv", "/stock/inventories/inv/history", "/stock/inventory-operations/inv-approve", "/system/info"} {
		status, body := request(t, f.app, "GET", "/local/v1"+path, "", f.token)
		if status != 200 {
			t.Fatalf("GET %s %d %s", path, status, body)
		}
	}
	status, body := request(t, f.app, "GET", "/local/v1/stock/inventories/inv/report.csv", "", f.token)
	if status != 200 || !strings.Contains(string(body), "counted_milli") || !strings.Contains(string(body), "-2000") {
		t.Fatalf("csv %d %s", status, body)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/stock/inventory-operations/inv-count", "", f.token)
	if status != 404 {
		t.Fatal("exposed another actor receipt", status)
	}
}
func TestReviewedInventoryHTTPRejectsDuplicateUnknownAndFractionalJSON(t *testing.T) {
	f := stockHTTPSetup(t)
	base := fmt.Sprintf(`{"operation_id":"open","inventory_id":"i","location_id":%q,"title":"Count","items":[{"product_id":%q,"unit":"unit"}]}`, f.shelf, f.product)
	for _, body := range []string{
		strings.Replace(base, `"operation_id":"open"`, `"operation_id":"open","operation_id":"other"`, 1),
		strings.Replace(base, `"unit":"unit"`, `"unit":"unit","unit":"kg"`, 1),
		strings.Replace(base, `"title":"Count"`, `"title":null`, 1),
		strings.Replace(base, `"title":"Count"`, `"title":"Count","tenant_id":"other"`, 1),
		base + ` {}`,
		strings.Replace(base, `"unit":"unit"`, `"unit":"unit","extra":true`, 1),
	} {
		status, response := request(t, f.app, "POST", "/local/v1/stock/inventories", body, f.token)
		if status != 400 {
			t.Fatalf("malformed %d %s", status, response)
		}
	}
	inventoryHTTPOpen(t, f, f.token)
	for _, number := range []string{"1.5", "1e3", "null", "-1", "9007199254740992"} {
		body := fmt.Sprintf(`{"operation_id":"count","expected_revision":1,"items":[{"product_id":%q,"counted_milli":%s}]}`, f.product, number)
		status, response := request(t, f.app, "POST", "/local/v1/stock/inventories/inv/counts", body, f.token)
		if status != 400 {
			t.Fatalf("number %s %d %s", number, status, response)
		}
	}
}
func TestReviewedInventoryHTTPQueriesAndAuthentication(t *testing.T) {
	f := stockHTTPSetup(t)
	for _, path := range []string{"/stock/inventories", "/stock/inventories/unknown", "/system/info"} {
		status, _ := request(t, f.app, "GET", "/local/v1"+path, "", "")
		if status != 401 {
			t.Fatal(status)
		}
	}
	for _, query := range []string{"?limit=0", "?offset=-1", "?limit=101", "?limit=1&limit=2", "?tenant_id=other", "?limit=", "?limit=1.0", "?status=missing", "?offset=1000001"} {
		status, body := request(t, f.app, "GET", "/local/v1/stock/inventories"+query, "", f.token)
		if status != 400 {
			t.Fatalf("query %s %d %s", query, status, body)
		}
	}
}
func TestReviewedInventoryHTTPHistoricalReadsAfterContractRemoval(t *testing.T) {
	f := stockHTTPSetup(t)
	inventoryHTTPOpen(t, f, f.token)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core}), 200)
	for _, path := range []string{"/stock/inventories/inv", "/stock/inventory-operations/inv-open"} {
		status, body := request(t, f.app, "GET", "/local/v1"+path, "", f.token)
		if status != 200 {
			t.Fatalf("historical %d %s", status, body)
		}
	}
	in := stocktaking.CountInput{OperationID: "new", ExpectedRevision: 1, Items: []stocktaking.CountItem{{ProductID: f.product, CountedMilli: 0}}}
	status, _ := request(t, f.app, "POST", "/local/v1/stock/inventories/inv/counts", inventoryHTTPJSON(t, in), f.token)
	if status != 403 {
		t.Fatal(status)
	}
}
func TestReviewedInventoryHTTPStaleCodeAndNoStockAdjustment(t *testing.T) {
	f := stockHTTPSetup(t)
	worker := inventoryHTTPWorker(t, f)
	inventoryHTTPOpen(t, f, worker)
	in := stocktaking.CountInput{OperationID: "count", ExpectedRevision: 1, Items: []stocktaking.CountItem{{ProductID: f.product, CountedMilli: 0}}}
	status, _ := request(t, f.app, "POST", "/local/v1/stock/inventories/inv/counts", inventoryHTTPJSON(t, in), worker)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/stock/inventories/inv/state", inventoryHTTPJSON(t, stocktaking.StateInput{OperationID: "submit", ExpectedRevision: 2, Action: "submit", Reason: "Checked"}), worker)
	if status != 200 {
		t.Fatal(status)
	}
	stockRequest(t, f, stock.Input{OperationID: "entry-after", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 1000, Reason: "Delivery"}, 201)
	status, body := request(t, f.app, "POST", "/local/v1/stock/inventories/inv/state", inventoryHTTPJSON(t, stocktaking.StateInput{OperationID: "approve", ExpectedRevision: 3, Action: "approve", Reason: "Checked"}), f.token)
	if status != 409 || !strings.Contains(string(body), "inventory_recount_required") {
		t.Fatalf("stale %d %s", status, body)
	}
	assertStockBalance(t, f, f.shelf, 1000)
}
