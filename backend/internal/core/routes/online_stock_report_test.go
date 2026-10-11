package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOnlineStockReportsMountedRolesAndInputs(t *testing.T) {
	for _, tc := range []struct {
		role, path, body string
		want             int
	}{
		{"cashier", "/summary", "", 403}, {"employee", "/zero", "", 403}, {"accountant", "/below-minimum", "", 403},
		{"cashier", "/coverage", "", 403}, {"employee", "/export.csv", "", 403},
		{"stock", "/summary?limit=1", "", 400}, {"stock", "/zero?issue=all", "", 400}, {"owner", "/below-minimum?active=", "", 400},
		{"manager", "/coverage?horizon_days=0", "", 400}, {"admin", "/export.csv?tenant_id=x", "", 400}, {"owner", "/summary", "{}", 400},
	} {
		t.Run(tc.role+tc.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("GET", "/api/v1/catalog/stock"+tc.path, strings.NewReader(tc.body))
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
