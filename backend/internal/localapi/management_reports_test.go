package localapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/stock"
	"titansystem-backend/internal/localdb/stocktaking"
)

const planReportPath = "/local/v1/management/production/simultaneous-plan"

func TestManagementSimultaneousSharedIngredientsAndNoWrites(t *testing.T) {
	f, in := capacityFixture(t)
	v2 := in
	v2.OperationID = "plan-v2"
	v2.VersionID = "bread-v2"
	v2.ExpectedRevision = 1
	status, b := request(t, f.app, "POST", recipePath, recipeBody(t, v2), f.token)
	if status != 201 {
		t.Fatalf("%d %s", status, b)
	}
	var before int
	if err := f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, batches := range []int{1, 2, 3} {
		t.Run(fmt.Sprint(batches), func(t *testing.T) {
			body := fmt.Sprintf(`{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":%d},{"version_id":"bread-v2","batches":%d}]}`, batches, batches)
			status, b := request(t, f.app, "POST", planReportPath, body, f.token)
			if status != 200 {
				t.Fatalf("%d %s", status, b)
			}
			var out production.SimultaneousPlan
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatal(err)
			}
			if len(out.Materials) != 2 || len(out.Outputs) != 2 || out.Reserved || out.Materials[0].RequiredMilli != int64(batches)*1000000 || out.Materials[1].RequiredMilli != int64(batches)*200002 {
				t.Fatal(out)
			}
			if out.Feasible != (batches <= 2) {
				t.Fatal(out)
			}
			if batches == 3 && out.Materials[1].ShortfallMilli != 100001 {
				t.Fatal(out)
			}
		})
	}
	var after int
	if err := f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&after); err != nil || after != before {
		t.Fatal("preview wrote outbox", after, before, err)
	}
}
func TestManagementSimultaneousStrictInputs(t *testing.T) {
	f, _ := capacityFixture(t)
	for _, body := range []string{`{}`, `null`, `{"location_id":"production-room","recipes":[]}`, `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":0}]}`, `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":1,"batches":2}]}`, `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":1},{"version_id":"bread-v1","batches":1}]}`, `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":1.1}]}`, `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":9007199254740991}]}`, `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":1}],"tenant_id":"foreign"}`} {
		status, b := request(t, f.app, "POST", planReportPath, body, f.token)
		if status != 400 {
			t.Fatalf("%s: %d %s", body, status, b)
		}
	}
	for _, body := range []string{`{"location_id":"missing","recipes":[{"version_id":"bread-v1","batches":1}]}`, `{"location_id":"production-room","recipes":[{"version_id":"missing","batches":1}]}`} {
		status, b := request(t, f.app, "POST", planReportPath, body, f.token)
		if status != 404 {
			t.Fatalf("%d %s", status, b)
		}
	}
	if _, err := f.db.Exec(`UPDATE products SET unit='kg' WHERE id='flour'`); err != nil {
		t.Fatal(err)
	}
	status, b := request(t, f.app, "POST", planReportPath, `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":1}]}`, f.token)
	if status != 409 {
		t.Fatalf("%d %s", status, b)
	}
}
func TestManagementSimultaneousHeldStockAndRevocation(t *testing.T) {
	f, reservation := materialsFixture(t)
	requireMaterials(t, f, "POST", materialsPath, reservation, 201)
	body := `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":2}]}`
	status, b := request(t, f.app, "POST", planReportPath, body, f.token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	var out production.SimultaneousPlan
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Feasible || out.Materials[0].ReservedMilli != 1500000 || out.Materials[0].ShortfallMilli != 500000 {
		t.Fatal(out)
	}
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, f.app, "POST", planReportPath, body, f.token)
	if status != 401 && status != 403 {
		t.Fatal(status)
	}
}
func TestManagementCostScenarioExactAndUnknownComponents(t *testing.T) {
	f, in := recipeFixture(t)
	status, b := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 201 {
		t.Fatal(status, string(b))
	}
	if _, err := f.db.Exec(`UPDATE products SET cost_cents=1`); err != nil {
		t.Fatal(err)
	}
	path := "/local/v1/management/production/recipe-versions/bread-v1/cost-scenario"
	status, b = request(t, f.app, "POST", path, `{"batches":1,"sale_price_cents":1000}`, f.token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	var out production.CostScenario
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Labor != nil || out.Contribution != nil || out.MarginBasisPoints != nil || out.AllRequestedComponentsProvided || out.KnownCost.Numerator != "600001" || out.Revenue.Numerator != "10000" || out.Profit || out.HistoricalCost {
		t.Fatal(out)
	}
	status, b = request(t, f.app, "POST", path, `{"batches":1,"labor":{"minutes":1,"workers":2,"hourly_cost_cents":100},"overhead_cents":0,"sale_price_cents":1000}`, f.token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Labor.Numerator != "10" || out.Labor.Denominator != "3" || out.KnownCost.Numerator != "1810003" || out.KnownCost.Denominator != "3000" || !out.AllRequestedComponentsProvided || out.Contribution == nil || out.MarginBasisPoints == nil {
		t.Fatal(out)
	}
	recipeCounts(t, f, 1)
}
func TestManagementCostScenarioRejectsAmbiguityAndUnsafeValues(t *testing.T) {
	f, in := recipeFixture(t)
	status, b := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 201 {
		t.Fatal(status, string(b))
	}
	path := "/local/v1/management/production/recipe-versions/bread-v1/cost-scenario"
	for _, body := range []string{`{}`, `{"batches":1,"batches":2}`, `{"batches":1,"labor":null}`, `{"batches":1,"labor":{}}`, `{"batches":1,"labor":{"minutes":1,"minutes":2,"workers":1,"hourly_cost_cents":5}}`, `{"batches":1,"labor":{"minutes":1,"workers":0,"hourly_cost_cents":5}}`, `{"batches":1,"labor":{"minutes":525601,"workers":1,"hourly_cost_cents":5}}`, `{"batches":1,"overhead_cents":-1}`, `{"batches":1,"sale_price_cents":1.1}`, `{"batches":1,"tenant_id":"foreign"}`, `{"batches":1}{}`} {
		status, b = request(t, f.app, "POST", path, body, f.token)
		if status != 400 {
			t.Fatalf("%s %d %s", body, status, b)
		}
	}
}
func TestManagementPositionsValuationCSVAndPermissions(t *testing.T) {
	f := stockHTTPSetup(t)
	worker := inventoryHTTPWorker(t, f)
	stockRequest(t, f, stock.Input{OperationID: "report-entry", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "Opening"}, 201)
	if _, err := f.db.Exec(`UPDATE products SET cost_cents=101,name='=HYPERLINK("bad")' WHERE id=?`, f.product); err != nil {
		t.Fatal(err)
	}
	base := "/local/v1/management/stock/"
	status, b := request(t, f.app, "GET", base+"positions?location_id="+f.shelf, "", worker)
	if status != 200 || strings.Contains(string(b), "catalog_cost_cents") {
		t.Fatal(status, string(b))
	}
	status, b = request(t, f.app, "GET", base+"valuation?location_id="+f.shelf, "", worker)
	if status != 403 {
		t.Fatal("cost leaked", status, string(b))
	}
	status, b = request(t, f.app, "GET", base+"valuation?location_id="+f.shelf+"&product_id="+f.product, "", f.token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	var out stock.PositionReport
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || *out.PageValueNumerator != "505" || *out.PageValueDenominator != "1" || out.Items[0].PhysicalMilli != 5000 {
		t.Fatal(out)
	}
	status, b = request(t, f.app, "GET", base+"valuation.csv?location_id="+f.shelf, "", f.token)
	if status != 200 || !strings.Contains(string(b), "'=HYPERLINK") {
		t.Fatal(status, string(b))
	}
	for _, q := range []string{"location_id=missing", "location_id=" + f.shelf + "&product_id=" + f.product + "&limit=1&offset=999"} {
		status, b = request(t, f.app, "GET", base+"positions?"+q, "", f.token)
		if strings.Contains(q, "missing") {
			if status != 404 {
				t.Fatal(status)
			}
		} else if status != 200 {
			t.Fatal(status, string(b))
		}
	}
	for _, q := range []string{"location_id=" + f.shelf + "&location_id=x", "location_id=" + f.shelf + "&limit=0", "location_id=" + f.shelf + "&offset=-1", "location_id=" + f.shelf + "&product_id=", "location_id=" + f.shelf + "&tenant_id=foreign"} {
		status, b = request(t, f.app, "GET", base+"positions?"+q, "", f.token)
		if status != 400 {
			t.Fatal(q, status, string(b))
		}
	}
}
func TestManagementVarianceUncountedAndEstimatedShortage(t *testing.T) {
	f := stockHTTPSetup(t)
	worker := inventoryHTTPWorker(t, f)
	stockRequest(t, f, stock.Input{OperationID: "report-entry", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "Opening"}, 201)
	if _, err := f.db.Exec(`UPDATE products SET cost_cents=101 WHERE id=?`, f.product); err != nil {
		t.Fatal(err)
	}
	inventoryHTTPOpen(t, f, worker)
	path := "/local/v1/management/inventories/inv/variance"
	status, b := request(t, f.app, "GET", path, "", f.token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	var out stocktaking.VarianceReport
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.UncountedLines != 1 || out.CountedLines != 0 || out.Lines[0].DifferenceNumerator != nil {
		t.Fatal(out)
	}
	status, b = request(t, f.app, "POST", "/local/v1/stock/inventories/inv/counts", `{"operation_id":"report-count","expected_revision":1,"items":[{"product_id":"`+f.product+`","counted_milli":3000}]}`, worker)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	status, b = request(t, f.app, "GET", path, "", f.token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.ShortageNumerator != "202" || out.NetNumerator != "-202" || out.ShortageLines != 1 || out.PostedToFinance {
		t.Fatal(out)
	}
	status, _ = request(t, f.app, "GET", path, "", worker)
	if status != 403 {
		t.Fatal(status)
	}
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	balance, err := stock.AvailableBalance(context.Background(), f.db, a, f.device, f.product, f.shelf)
	if err != nil || balance.PhysicalMilli != 5000 {
		t.Fatal(balance, err)
	}
}
func TestManagementParallelPreviewsNeverConsumeSharedStock(t *testing.T) {
	f, _ := capacityFixture(t)
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := production.Simultaneous(context.Background(), f.db, a, f.device, production.SimultaneousInput{LocationID: "production-room", Recipes: []production.PlanRecipe{{VersionID: "bread-v1", Batches: 1}}})
			if err == nil && (!out.Feasible || out.Materials[0].PhysicalMilli != 4000000) {
				err = fmt.Errorf("unexpected plan %+v", out)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM production_material_reservations`).Scan(&n); err != nil || n != 0 {
		t.Fatal("preview reserved", n, err)
	}
}

func TestManagementReportQueriesBodiesAndSessionRequired(t *testing.T) {
	f, _ := capacityFixture(t)
	for _, v := range []struct{ method, path, body string }{
		{"GET", "/local/v1/management/sales/products?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z", ""},
		{"GET", "/local/v1/management/stock/positions?location_id=production-room", ""},
		{"POST", planReportPath, `{"location_id":"production-room","recipes":[{"version_id":"bread-v1","batches":1}]}`},
	} {
		status, b := request(t, f.app, v.method, v.path, v.body, "")
		if status != 401 {
			t.Fatal(status, string(b))
		}
	}
	for _, q := range []string{"", "?from=bad&to=bad", "?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z&compare_from=2026-09-01T00:00:00Z", "?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z&from=bad", "?from=2026-10-01T00:00:00Z&to=2028-10-02T00:00:00Z"} {
		status, b := request(t, f.app, "GET", "/local/v1/management/sales/products"+q, "", f.token)
		if status != 400 {
			t.Fatal(q, status, string(b))
		}
	}
	status, b := request(t, f.app, "GET", "/local/v1/management/sales/products?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z", "{}", f.token)
	if status != 400 {
		t.Fatal(status, string(b))
	}
	for _, path := range []string{"/local/v1/management/sales/products?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z", "/local/v1/management/sales/products.csv?from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z"} {
		status, b := request(t, f.app, "GET", path, "", f.token)
		if status != 200 {
			t.Fatal(status, string(b))
		}
	}
}

func TestManagementRestockHumanPolicyAndNoAutomaticOrder(t *testing.T) {
	f := stockHTTPSetup(t)
	worker := inventoryHTTPWorker(t, f)
	if _, err := f.db.Exec(`INSERT INTO restock_policies VALUES(?,?,?,2000,10000,1,?,'now')`, f.owner.TenantID, f.owner.StoreID, f.product, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	stockRequest(t, f, stock.Input{OperationID: "restock-entry", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 1000, Reason: "Opening"}, 201)
	body := `{"product_id":"` + f.product + `","from":"2026-10-01T00:00:00Z","to":"2026-10-02T00:00:00Z","lead_days":2,"coverage_days":3,"safety_milli":0}`
	path := "/local/v1/management/replenishment/preview"
	status, b := request(t, f.app, "POST", path, body, f.token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	var out stock.RestockPreview
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.SuggestedOrderMilli != 9000 || out.SuggestedMinimumMilli != 2000 || out.SuggestedTargetMilli != 10000 || out.SoldMilli != 0 || out.OrderCreated || !out.ReviewRequired || out.DemandBasis != "no_sales_in_selected_period_policy_only" {
		t.Fatal(out)
	}
	status, _ = request(t, f.app, "POST", path, body, worker)
	if status != 403 {
		t.Fatal(status)
	}
	for _, bad := range []string{strings.Replace(body, `"lead_days":2`, `"lead_days":-1`, 1), strings.Replace(body, `"coverage_days":3`, `"coverage_days":0`, 1), strings.Replace(body, `"safety_milli":0`, `"safety_milli":1`, 1), strings.Replace(body, `"from":"2026-10-01T00:00:00Z"`, `"from":"bad"`, 1), strings.Replace(body, `"lead_days":2`, `"lead_days":2,"lead_days":3`, 1)} {
		status, b = request(t, f.app, "POST", path, bad, f.token)
		if status != 400 && status != 409 {
			t.Fatal(bad, status, string(b))
		}
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM purchase_orders`).Scan(&n); err != nil || n != 0 {
		t.Fatal("preview created order", n, err)
	}
}
