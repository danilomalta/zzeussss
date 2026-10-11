package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReplenishmentMountedPermissionsAndStrictInputs(t *testing.T) {
	for _, tc := range []struct {
		role, method, path, body string
		want                     int
	}{
		{"cashier", "GET", "/summary", "", 403}, {"accountant", "GET", "/items", "", 403}, {"employee", "GET", "/products/1", "", 403}, {"cashier", "POST", "/products/1/simulate", "{}", 403}, {"employee", "GET", "/export.csv", "", 403},
		{"stock", "GET", "/summary?limit=1", "", 400}, {"owner", "GET", "/items?tenant_id=x", "", 400}, {"owner", "GET", "/products/01", "", 400}, {"owner", "GET", "/products/1?state=all", "", 400}, {"stock", "POST", "/products/1/simulate", "{}", 400}, {"owner", "POST", "/products/1/simulate?q=x", "{}", 400}, {"owner", "GET", "/export.csv?review_days=", "", 400}, {"owner", "GET", "/summary", "{}", 400},
	} {
		t.Run(tc.role+tc.method+tc.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest(tc.method, "/api/v1/catalog/replenishment"+tc.path, strings.NewReader(tc.body))
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
