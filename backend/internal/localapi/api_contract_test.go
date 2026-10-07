package localapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"titansystem-backend/internal/apicontract"
)

func TestLocalRouteInventoryAndNegotiatedAuthenticationError(t *testing.T) {
	_, app, _ := fixture(t)
	if e := apicontract.CheckInventory(app, "../../../docs/api/route-inventory.json", "local", "/local/v1/"); e != nil {
		t.Fatal(e)
	}
	req := httptest.NewRequest("GET", "/local/v1/products", nil)
	req.Header.Set(apicontract.ErrorFormatHeader, "v1")
	resp, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	var body apicontract.ErrorBody
	if json.NewDecoder(resp.Body).Decode(&body) != nil || resp.StatusCode != 401 || body.Error.Code != "unauthorized" {
		t.Fatal("local negotiated auth contract")
	}
}

func TestDocumentedPaginationBoundsAndAuthenticationOrder(t *testing.T) {
	_, app, owner := fixture(t)
	token := loginToken(t, app, owner.OwnerID)
	for _, path := range []string{
		"/local/v1/products?limit=0", "/local/v1/products?limit=101",
		"/local/v1/products?offset=-1", "/local/v1/products?limit=bad",
		"/local/v1/sales?limit=0", "/local/v1/sales?limit=51",
		"/local/v1/sales?offset=-1", "/local/v1/sales?offset=bad",
		"/local/v1/catalog/search?offset=-1", "/local/v1/catalog/search?offset=1000000001",
		"/local/v1/access/audit?limit=101", "/local/v1/access/audit?limit=0",
		"/local/v1/access/audit?cursor=invalid",
	} {
		for _, auth := range []bool{false, true} {
			useToken := ""
			want := 401
			if auth {
				useToken = token
				want = 400
			}
			status, _ := request(t, app, "GET", path, "", useToken)
			if status != want {
				t.Fatalf("pagination status %s: %d expected %d", path, status, want)
			}
		}
	}
}
