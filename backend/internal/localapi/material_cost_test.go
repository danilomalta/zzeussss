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
	"titansystem-backend/internal/localdb/production"
)

func TestMaterialCostHTTPExactEstimateAndCurrentValuation(t *testing.T) {
	f, in := recipeFixture(t)
	if _, err := f.db.Exec(`UPDATE products SET cost_cents=1 WHERE tenant_id=? AND id IN('flour','oil')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	status, body := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 201 {
		t.Fatalf("publish %d %s", status, body)
	}
	path := recipePath + "/" + in.VersionID + "/material-cost"
	status, body = request(t, f.app, "GET", path, "", f.token)
	if status != 200 {
		t.Fatalf("cost %d %s", status, body)
	}
	var result production.MaterialCostEstimate
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.MaterialCostCents != 600 || result.MaterialCost.Numerator != "600001" || result.MaterialCost.Denominator != "1000" || result.LaborCostCents != nil || result.OverheadCostCents != nil || result.IsComplete || result.Source != "current_catalog_material_estimate" {
		t.Fatal(result)
	}
	if _, err := f.db.Exec(`UPDATE products SET cost_cents=2 WHERE tenant_id=? AND id='oil'`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	status, body = request(t, f.app, "GET", path+"?batches=3", "", f.token)
	if status != 200 {
		t.Fatalf("cost %d %s", status, body)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.MaterialCostCents != 2100 || result.OutputQuantityMilli != 30000 || result.Batches != 3 {
		t.Fatal(result)
	}
	recipeCounts(t, f, 1)
}
func TestMaterialCostHTTPOrderFrozenRecipeAndNoFinanceEffects(t *testing.T) {
	f, in := orderFixture(t)
	status, body := request(t, f.app, "POST", orderPath, orderBody(t, in), f.token)
	if status != 201 {
		t.Fatalf("order %d %s", status, body)
	}
	f.db.Exec(`UPDATE products SET cost_cents=10 WHERE tenant_id=? AND id='flour'`, f.owner.TenantID)
	status, body = request(t, f.app, "GET", orderPath+"/"+in.OrderID+"/material-cost", "", f.token)
	if status != 200 {
		t.Fatalf("order cost %d %s", status, body)
	}
	var result production.MaterialCostEstimate
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.OrderID != in.OrderID || result.MaterialCostCents != 15000 || result.OutputQuantityMilli != 30000 || result.Lines[1].RequiresCostReview != true {
		t.Fatal(result)
	}
	orderCounts(t, f, 1, 1)
}
func TestMaterialCostHTTPRequiresAccountingInAdditionToProduction(t *testing.T) {
	f, in := recipeFixture(t)
	status, body := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 201 {
		t.Fatalf("publish %d %s", status, body)
	}
	for _, q := range []string{`INSERT INTO identities VALUES('producer','Production','now')`, `INSERT INTO memberships VALUES(?,'producer','production','active','now')`, `INSERT INTO membership_stores VALUES(?,'producer',?)`} {
		var err error
		if strings.Contains(q, "membership_stores") {
			_, err = f.db.Exec(q, f.owner.TenantID, f.owner.StoreID)
		} else if strings.Contains(q, "memberships") {
			_, err = f.db.Exec(q, f.owner.TenantID)
		} else {
			_, err = f.db.Exec(q)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	owner := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	if err := localauth.SetPassword(context.Background(), f.db, owner, f.device, "producer", "senha-forte-de-teste-1"); err != nil {
		t.Fatal(err)
	}
	token := loginToken(t, f.app, "producer")
	status, _ = request(t, f.app, "GET", recipePath+"/"+in.VersionID, "", token)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = request(t, f.app, "GET", recipePath+"/"+in.VersionID+"/material-cost", "", token)
	if status != 403 {
		t.Fatal("cost leaked", status)
	}
}
func TestMaterialCostHTTPBoundsUnitChangeAndHistoricalRead(t *testing.T) {
	f, in := recipeFixture(t)
	status, body := request(t, f.app, "POST", recipePath, recipeBody(t, in), f.token)
	if status != 201 {
		t.Fatalf("publish %d %s", status, body)
	}
	path := recipePath + "/" + in.VersionID + "/material-cost"
	for _, q := range []string{"?batches=0", "?batches=-1", "?batches=1.0", "?batches=1&batches=2", "?batches=", "?batches=9007199254740991", "?tenant_id=other"} {
		status, body = request(t, f.app, "GET", path+q, "", f.token)
		if status != 400 {
			t.Fatalf("query %s %d %s", q, status, body)
		}
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core}), 200)
	status, body = request(t, f.app, "GET", path, "", f.token)
	if status != 200 {
		t.Fatalf("historical %d %s", status, body)
	}
	f.db.Exec(`UPDATE products SET unit='kg' WHERE tenant_id=? AND id='flour'`, f.owner.TenantID)
	status, _ = request(t, f.app, "GET", path, "", f.token)
	if status != 409 {
		t.Fatal("changed unit accepted", status)
	}
	status, _ = request(t, f.app, "GET", fmt.Sprintf("%s/%s/material-cost", recipePath, "unknown"), "", f.token)
	if status != 404 {
		t.Fatal(status)
	}
}
