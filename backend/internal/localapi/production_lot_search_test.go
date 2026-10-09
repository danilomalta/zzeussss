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

const lotSearchPath = "/local/v1/production/lot-search"

func lotSearchResponse(t *testing.T, f *httpContractFixture, query string) production.LotSearchPage {
	t.Helper()
	body := requireMaterials(t, f, "GET", lotSearchPath+query, nil, 200)
	var out production.LotSearchPage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err, string(body))
	}
	return out
}
func addSearchLot(t *testing.T, f *httpContractFixture, resultID, id, expiry string) production.LotInput {
	t.Helper()
	in := production.LotInput{OperationID: id, LotID: id, ResultID: resultID, QuantityMilli: 1000, Code: "CODE-" + id, ManufacturedOn: "2024-02-29", ExpiresOn: expiry, Reason: "Identificar"}
	requireMaterials(t, f, "POST", lotsPath, in, 201)
	return in
}
func TestHTTPProductionLotSearchDatesUnknownQualityAndHistoricalStates(t *testing.T) {
	f, result := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	for _, v := range []struct{ id, expiry string }{{"a", "2024-02-29"}, {"b", "2024-03-01"}, {"c", "2024-03-02"}, {"d", ""}, {"e", "2024-03-01"}} {
		addSearchLot(t, f, result.ResultID, v.id, v.expiry)
	}
	q := production.QualityInput{OperationID: "q-b-fail", LotID: "b", Status: "failed", Criterion: "Criterio", Reason: "Parecer"}
	requireMaterials(t, f, "POST", qualityPath, q, 201)
	q.OperationID, q.Status, q.ExpectedRevision = "q-b-pass", "passed", 1
	requireMaterials(t, f, "POST", qualityPath, q, 201)
	q.OperationID, q.LotID, q.Status, q.ExpectedRevision = "q-e-fail", "e", "failed", 0
	requireMaterials(t, f, "POST", qualityPath, q, 201)
	requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: "void-e", LotID: "e", ExpectedRevision: 1, Reason: "Anular"}, 200)
	before := traceReadState(t, f)
	for _, pair := range []struct {
		query string
		count int64
	}{{"", 5}, {"?expiry=dated", 4}, {"?expiry=undated", 1}, {"?expires_from=2024-02-29&expires_to=2024-03-01", 3}, {"?expires_to=2024-02-29", 1}, {"?expires_from=2024-03-02", 1}, {"?status=recorded", 4}, {"?status=voided", 1}, {"?quality=not_assessed", 3}, {"?quality=passed", 1}, {"?quality=failed", 1}, {"?quality=failed&status=recorded", 0}, {"?quality=passed&status=recorded&expiry=dated&expires_from=2024-03-01&expires_to=2024-03-01", 1}} {
		out := lotSearchResponse(t, f, pair.query)
		if out.TotalCount != pair.count || int64(len(out.Items)) != pair.count || out.HasMore || out.Limit != 50 {
			t.Fatal(pair, out)
		}
		for _, item := range out.Items {
			if item.OrderID != result.OrderID || item.LocationID != "production-room" || item.Lot.Unit != "unit" || item.Lot.QuantityMilli != 1000 || item.Quality.LotStatus != item.Lot.Status {
				t.Fatal(item)
			}
		}
	}
	out := lotSearchResponse(t, f, "")
	if out.Items[0].Lot.ID != "a" || out.Items[4].Lot.ID != "d" || out.Items[4].Quality.Status != "not_assessed" || out.Items[4].Quality.Latest != nil {
		t.Fatal(out)
	}
	out = lotSearchResponse(t, f, "?quality=passed")
	if out.Items[0].Lot.ID != "b" || out.Items[0].Quality.Revision != 2 || out.Items[0].Quality.Latest.Status != "passed" {
		t.Fatal(out)
	}
	out = lotSearchResponse(t, f, "?quality=failed&status=voided")
	if out.Items[0].Lot.ID != "e" || out.Items[0].Quality.Latest.LotSnapshot.Status != "recorded" {
		t.Fatal(out)
	}
	for _, key := range []string{"product_id", "location_id"} {
		out = lotSearchResponse(t, f, "?"+key+"="+url.QueryEscape("x' OR 1=1 --"))
		if out.TotalCount != 0 || len(out.Items) != 0 {
			t.Fatal(out)
		}
	}
	out = lotSearchResponse(t, f, "?product_id=bread&location_id=production-room&quality=passed")
	if out.TotalCount != 1 {
		t.Fatal(out)
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("read changed data/clock")
	}
	if _, err := f.db.Exec(`UPDATE products SET name='Changed',unit='kg' WHERE id='bread'`); err != nil {
		t.Fatal(err)
	}
	out = lotSearchResponse(t, f, "?product_id=bread")
	if out.Items[0].Lot.Unit != "unit" {
		t.Fatal(out)
	}
	// Existing P09 routes retain their contract and result-level totals.
	requireMaterials(t, f, "GET", lotsPath+"/a", nil, 200)
	requireMaterials(t, f, "GET", resultsPath+"/"+result.ResultID+"/lots", nil, 200)
}
func TestHTTPProductionLotSearchPaginationAcrossVoidedHistory(t *testing.T) {
	f, result := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	for i := 0; i < 27; i++ {
		addSearchLot(t, f, result.ResultID, fmt.Sprintf("lot-%02d", i), "2030-01-01")
	}
	for i := 0; i < 26; i++ {
		id := fmt.Sprintf("lot-%02d", i)
		requireMaterials(t, f, "POST", lotsPath+"/void", production.VoidLotInput{OperationID: "void-" + id, LotID: id, ExpectedRevision: 1, Reason: "Anular"}, 200)
		addSearchLot(t, f, result.ResultID, fmt.Sprintf("replacement-%02d", i), "2030-01-01")
	}
	if _, err := f.db.Exec(`UPDATE production_lots SET created_at='2024-02-29T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	first := lotSearchResponse(t, f, "?expiry=dated")
	second := lotSearchResponse(t, f, "?expiry=dated&offset=50")
	if first.TotalCount != 53 || len(first.Items) != 50 || !first.HasMore || first.Items[0].Lot.ID != "lot-00" || second.TotalCount != 53 || len(second.Items) != 3 || second.HasMore || second.Items[0].Lot.ID != "replacement-23" {
		t.Fatal(first, second)
	}
	for _, offset := range []string{"53", "100", "9007199254740991"} {
		out := lotSearchResponse(t, f, "?offset="+offset)
		if out.TotalCount != 53 || out.Items == nil || len(out.Items) != 0 || out.HasMore {
			t.Fatal(out)
		}
	}
	out := lotSearchResponse(t, f, "?status=recorded")
	if out.TotalCount != 27 {
		t.Fatal(out)
	}
}
func TestHTTPProductionLotSearchStrictQueryAuthorizationAndHistoricalRead(t *testing.T) {
	f, _, _ := traceFixture(t)
	for _, query := range []string{"?status=approved", "?quality=recorded", "?expiry=expired", "?expiry=undated&expires_from=2024-02-29", "?expires_from=2025-02-29", "?expires_to=0000-01-01", "?expires_from=2024-3-01", "?expires_from=2024-03-02&expires_to=2024-03-01", "?quality=", "?quality=passed&quality=failed", "?expires_to=2024-03-01&expires_to=2024-03-02", "?product_id=%00", "?location_id=%20room", "?offset=-1", "?offset=%2B1", "?offset=1.5", "?offset=", "?offset=9007199254740992", "?offset=9999999999999999999999", "?offset=0&offset=1", "?tenant_id=other", "?store_id=other", "?limit=100"} {
		requireMaterials(t, f, "GET", lotSearchPath+query, nil, 400)
	}
	status, _ := request(t, f.app, "GET", lotSearchPath, "", "")
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
		if _, err := production.SearchProductionLots(context.Background(), f.db, foreign, f.device, production.LotSearch{}); err == nil {
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
	lotSearchResponse(t, f, "")
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("expired read changed clock")
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='cashier'`); err != nil {
		t.Fatal(err)
	}
	requireMaterials(t, f, "GET", lotSearchPath, nil, 403)
}
func TestHTTPProductionLotSearchBackupRestoresDatesAndQuality(t *testing.T) {
	f, result := resultsFixture(t)
	requireMaterials(t, f, "POST", resultsPath, result, 201)
	lot := addSearchLot(t, f, result.ResultID, "dated-lot", "2024-03-01")
	requireMaterials(t, f, "POST", qualityPath, production.QualityInput{OperationID: "review", LotID: lot.LotID, Status: "passed", Criterion: "Criterio", Reason: "Parecer"}, 201)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "lots.tytbak")
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
	out, err := production.SearchProductionLots(ctx, db, materialsActor(f), f.device, production.LotSearch{Quality: "passed", ExpiresFrom: "2024-03-01", ExpiresTo: "2024-03-01"})
	if err != nil || out.TotalCount != 1 || out.Items[0].OrderID != result.OrderID || out.Items[0].Lot.ExpiresOn != "2024-03-01" || out.Items[0].Quality.Latest.Revision != 1 {
		t.Fatal(out, err)
	}
}
func TestHTTPProductionLotSearchConcurrentQualityFilterSharesSnapshot(t *testing.T) {
	f, _, lot := traceFixture(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 1; i <= 6; i++ {
			status := "passed"
			if i%2 == 0 {
				status = "failed"
			}
			code, _ := request(t, f.app, "POST", qualityPath, orderBody(t, production.QualityInput{OperationID: fmt.Sprint("search-review", i), LotID: lot.LotID, ExpectedRevision: int64(i), Status: status, Criterion: "Criterio", Reason: "Reavaliar"}), f.token)
			if code != 201 {
				statuses <- code
				return
			}
		}
		statuses <- 200
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 16; i++ {
			status, body := request(t, f.app, "GET", lotSearchPath+"?quality=passed", "", f.token)
			var out production.LotSearchPage
			if status != 200 || json.Unmarshal(body, &out) != nil || out.TotalCount != int64(len(out.Items)) {
				statuses <- 500
				return
			}
			for _, item := range out.Items {
				if item.Quality.Status != "passed" || item.Quality.Latest.Status != "passed" || item.Quality.Revision != item.Quality.Latest.Revision {
					statuses <- 500
					return
				}
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
}
