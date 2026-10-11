package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCostMountedPermissionsAndInput(t *testing.T) {
	cases := []struct {
		role, method, path, body string
		want                     int
	}{
		{"stock", "GET", "/produtos/1/cost", "", 403},
		{"cashier", "GET", "/produtos/1/margin", "", 403},
		{"employee", "GET", "/produtos/1/cost/history", "", 403},
		{"accountant", "GET", "/catalog/costs/operations/11111111-1111-4111-8111-111111111111", "", 403},
		{"stock", "POST", "/produtos/1/cost", "{}", 403},
		{"owner", "GET", "/produtos/01/cost", "", 400},
		{"owner", "GET", "/produtos/1/cost?tenant_id=x", "", 400},
		{"owner", "GET", "/produtos/1/margin", "{}", 400},
		{"owner", "GET", "/produtos/1/cost/history?limit=101", "", 400},
		{"owner", "GET", "/produtos/1/cost/history?offset=1&offset=2", "", 400},
		{"owner", "GET", "/catalog/costs/operations/bad", "", 400},
		{"owner", "POST", "/produtos/1/cost", "{}", 400},
	}
	for _, tc := range cases {
		t.Run(tc.role+tc.method+tc.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest(tc.method, "/api/v1"+tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", tc.role))
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
