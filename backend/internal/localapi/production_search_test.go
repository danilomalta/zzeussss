package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/production"
)

const searchPath = "/local/v1/production/order-search"

func searchResponse(t *testing.T, f *httpContractFixture, query string) production.OrderSearchPage {
	t.Helper()
	body := requireMaterials(t, f, "GET", searchPath+query, nil, 200)
	var out production.OrderSearchPage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err, string(body))
	}
	return out
}
func TestHTTPProductionSearchEffectiveStatesFiltersAndNoWrites(t *testing.T) {
	f, result := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	var completed production.Order
	body := requireMaterials(t, f, "GET", orderPath+"/"+result.OrderID, nil, 200)
	if err := json.Unmarshal(body, &completed); err != nil {
		t.Fatal(err)
	}
	// The completed record retains the approved physical status; filtering must
	// agree with the existing effective order API, not that storage detail.
	var physical string
	if err := f.db.QueryRow(`SELECT status FROM production_orders WHERE id=?`, result.OrderID).Scan(&physical); err != nil || physical != "approved" {
		t.Fatal(physical, err)
	}
	for _, pair := range []struct{ id, status string }{{"planned-order", "planned"}, {"approved-order", "approved"}, {"cancelled-order", "cancelled"}} {
		in := production.OrderInput{OperationID: pair.id, OrderID: pair.id, VersionID: completed.VersionID, LocationID: completed.LocationID, ResponsibleID: completed.ResponsibleID, PlannedBatches: 2}
		requireMaterials(t, f, "POST", orderPath, in, 201)
		if pair.status != "planned" {
			requireMaterials(t, f, "POST", orderPath+"/state", production.OrderStateInput{OperationID: pair.id + "-state", OrderID: pair.id, ExpectedRevision: 1, Status: pair.status, Reason: "Estado humano"}, 200)
		}
	}
	before := traceReadState(t, f)
	for _, status := range []string{"planned", "approved", "cancelled", "completed"} {
		out := searchResponse(t, f, "?status="+status)
		if out.TotalCount != 1 || len(out.Items) != 1 || out.Items[0].Status != status || out.HasMore || out.Limit != 50 || out.Filters.Status != status {
			t.Fatal(status, out)
		}
	}
	query := "?status=completed&location_id=" + url.QueryEscape(completed.LocationID) + "&responsible_id=" + url.QueryEscape(completed.ResponsibleID) + "&version_id=" + url.QueryEscape(completed.VersionID)
	out := searchResponse(t, f, query)
	if out.TotalCount != 1 || out.Items[0].CompletionID != result.ResultID || out.Items[0].PlannedOutputMilli != 30000 || out.Items[0].Recipe.OutputUnit != "unit" {
		t.Fatal(out)
	}
	for _, key := range []string{"location_id", "responsible_id", "version_id"} {
		out = searchResponse(t, f, "?"+key+"="+url.QueryEscape("x' OR 1=1 --"))
		if out.TotalCount != 0 || len(out.Items) != 0 || out.HasMore {
			t.Fatal(key, out)
		}
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("read writes data/clock")
	}
	// Current catalog and membership changes do not reinterpret frozen plans.
	if _, err := f.db.Exec(`UPDATE products SET name='Changed',unit='kg' WHERE id='bread'`); err != nil {
		t.Fatal(err)
	}
	out = searchResponse(t, f, "?status=completed")
	if out.Items[0].Recipe.OutputUnit != "unit" || out.Items[0].Recipe.YieldMilli != 10000 {
		t.Fatal(out)
	}
	// The original list remains compatible and accepts only its original query.
	requireMaterials(t, f, "GET", orderPath+"?offset=0", nil, 200)
	requireMaterials(t, f, "GET", orderPath+"?status=completed", nil, 400)
}
func TestHTTPProductionSearchPaginationAndCombinedFilters(t *testing.T) {
	f, in := orderFixture(t)
	if _, err := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?, 'other-room','production','Other')`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 53; i++ {
		in.OperationID = fmt.Sprintf("search-op-%02d", i)
		in.OrderID = fmt.Sprintf("search-%02d", i)
		in.LocationID = "production-room"
		if i == 52 {
			in.LocationID = "other-room"
		}
		requireMaterials(t, f, "POST", orderPath, in, 201)
	}
	if _, err := f.db.Exec(`UPDATE production_orders SET created_at='2024-02-29T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	first := searchResponse(t, f, "?location_id=production-room")
	second := searchResponse(t, f, "?location_id=production-room&offset=50")
	if first.TotalCount != 52 || len(first.Items) != 50 || !first.HasMore || first.Items[0].ID != "search-51" || second.TotalCount != 52 || len(second.Items) != 2 || second.HasMore || second.Items[0].ID != "search-01" || second.Items[1].ID != "search-00" {
		t.Fatal(first, second)
	}
	for _, n := range []string{"52", "100", "9007199254740991"} {
		out := searchResponse(t, f, "?location_id=production-room&offset="+n)
		if out.TotalCount != 52 || out.HasMore || out.Items == nil || len(out.Items) != 0 {
			t.Fatal(out)
		}
	}
	out := searchResponse(t, f, "?status=planned&location_id=other-room&responsible_id="+url.QueryEscape(in.ResponsibleID)+"&version_id="+url.QueryEscape(in.VersionID))
	if out.TotalCount != 1 || out.Items[0].ID != "search-52" {
		t.Fatal(out)
	}
}
func TestHTTPProductionSearchStrictQueryAuthorizationAndExpiredHistory(t *testing.T) {
	f, in := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, in, 201)
	for _, query := range []string{"?status=consumed", "?status=", "?status=planned&status=approved", "?location_id=", "?version_id=%20bad", "?responsible_id=%00", "?offset=-1", "?offset=%2B1", "?offset=1.5", "?offset=", "?offset=9007199254740992", "?offset=9999999999999999999999999", "?offset=0&offset=1", "?tenant_id=other", "?store_id=other", "?limit=100"} {
		requireMaterials(t, f, "GET", searchPath+query, nil, 400)
	}
	status, _ := request(t, f.app, "GET", searchPath, "", "")
	if status != 401 {
		t.Fatal(status)
	}
	for _, company := range []bool{true, false} {
		foreign := materialsActor(f)
		if company {
			foreign.TenantID = "other"
		} else {
			foreign.StoreID = "other"
		}
		if _, err := production.SearchOrders(context.Background(), f.db, foreign, f.device, production.OrderSearch{}); err == nil {
			t.Fatal("cross-scope")
		}
	}
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); err != nil {
		t.Fatal(err)
	}
	before := traceReadState(t, f)
	searchResponse(t, f, "")
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("expired read changes clock")
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", searchPath, nil, 403)
}
func TestHTTPProductionSearchBackupRestoresFiltersAndSnapshots(t *testing.T) {
	f, stages, _ := traceFixture(t)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "search.tytbak")
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
	out, err := production.SearchOrders(ctx, db, materialsActor(f), f.device, production.OrderSearch{Status: "completed"})
	if err != nil || out.TotalCount != 1 || out.Items[0].ID != stages.OrderID || len(out.Items[0].Recipe.Ingredients) != 2 {
		t.Fatal(out, err)
	}
}
func TestHTTPProductionSearchConcurrentStateCountAndPageShareSnapshot(t *testing.T) {
	f, in := orderFixture(t)
	requireMaterials(t, f, "POST", orderPath, in, 201)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		status, _ := request(t, f.app, "POST", orderPath+"/state", orderBody(t, production.OrderStateInput{OperationID: "approve", OrderID: in.OrderID, ExpectedRevision: 1, Status: "approved", Reason: "Aprovar"}), f.token)
		statuses <- status
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 15; i++ {
			status, body := request(t, f.app, "GET", searchPath+"?status=planned", "", f.token)
			var out production.OrderSearchPage
			if status != 200 || json.Unmarshal(body, &out) != nil || out.TotalCount != int64(len(out.Items)) || (len(out.Items) == 1 && out.Items[0].Status != "planned") {
				statuses <- 500
				return
			}
		}
		statuses <- 200
	}()
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatal(status)
		}
	}
	if out := searchResponse(t, f, "?status=approved"); out.TotalCount != 1 {
		t.Fatal(out)
	}
}
