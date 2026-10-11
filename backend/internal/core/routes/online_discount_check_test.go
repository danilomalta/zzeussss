package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscountCheckMountedPermissionsAndInput(t *testing.T) {
	cases := []struct {
		role, method, path, body string
		want                     int
	}{
		{"cashier", "GET", "/discounts/checks/summary", "", 403},
		{"stock", "GET", "/discounts/checks/price-changed", "", 403},
		{"employee", "GET", "/discounts/checks/inactive", "", 403},
		{"accountant", "GET", "/discounts/checks/suggestions/1", "", 403},
		{"stock", "GET", "/discounts/checks/export.csv", "", 403},
		{"owner", "GET", "/discounts/checks/summary?limit=1", "", 400},
		{"owner", "GET", "/discounts/checks/summary?q=a&q=b", "", 400},
		{"owner", "GET", "/discounts/checks/price-changed?issue=inactive", "", 400},
		{"owner", "GET", "/discounts/checks/inactive?limit=101", "", 400},
		{"owner", "GET", "/discounts/checks/suggestions/01", "", 400},
		{"owner", "GET", "/discounts/checks/suggestions/1?tenant_id=x", "", 400},
		{"owner", "GET", "/discounts/checks/suggestions/1", "{}", 400},
		{"owner", "GET", "/discounts/checks/export.csv?issue=INVALID", "", 400},
		{"owner", "GET", "/discounts/checks/export.csv?active=all", "", 400},
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
