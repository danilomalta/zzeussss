package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStockSettingsMountedPermissionsAndStrictInputs(t *testing.T) {
	for _, tc := range []struct {
		role, method, path, body string
		want                     int
	}{
		{"stock", "GET", "/produtos/1/stock-settings", "", 403}, {"stock", "POST", "/produtos/1/stock-settings/preview", "{}", 403}, {"cashier", "POST", "/produtos/1/stock-settings", "{}", 403}, {"accountant", "GET", "/produtos/1/stock-settings/history", "", 403}, {"employee", "GET", "/catalog/stock-settings/operations/00000000-0000-4000-8000-000000000001", "", 403},
		{"owner", "GET", "/produtos/01/stock-settings", "", 400}, {"owner", "GET", "/produtos/1/stock-settings?tenant_id=x", "", 400}, {"owner", "POST", "/produtos/1/stock-settings/preview", "{}", 400}, {"owner", "POST", "/produtos/1/stock-settings", "{}", 400}, {"owner", "GET", "/produtos/1/stock-settings/history?limit=", "", 400}, {"owner", "GET", "/produtos/1/stock-settings/history?limit=1&limit=2", "", 400}, {"owner", "GET", "/catalog/stock-settings/operations/bad", "", 400}, {"owner", "GET", "/produtos/1/stock-settings", "{}", 400},
	} {
		t.Run(tc.role+tc.method+tc.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest(tc.method, "/api/v1"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", tc.role))
			req.Header.Set("Content-Type", "application/json")
			resp, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatal(resp.StatusCode, tc.want)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
