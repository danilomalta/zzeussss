package routes

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"testing"
	"titansystem-backend/internal/apicontract"
)

func TestOnlineRouteInventoryAndNegotiatedAuthenticationError(t *testing.T) {
	app := fiber.New()
	Registrar(app)
	if e := apicontract.CheckInventory(app, "../../../../docs/api/route-inventory.json", "online", "/api/v1/"); e != nil {
		t.Fatal(e)
	}
	req := httptest.NewRequest("GET", "/api/v1/auth/sessions", nil)
	req.Header.Set(apicontract.ErrorFormatHeader, "v1")
	resp, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	var body apicontract.ErrorBody
	if json.NewDecoder(resp.Body).Decode(&body) != nil || resp.StatusCode != 401 || body.Error.Code != "unauthorized" {
		t.Fatal("online negotiated auth contract")
	}
}
