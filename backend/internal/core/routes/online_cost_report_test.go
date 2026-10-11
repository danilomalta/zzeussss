package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCostReportMountedPermissionsAndInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		role, path, body string
		want             int
	}{
		{"stock", "/summary", "", 403}, {"cashier", "/entries", "", 403}, {"accountant", "/entries/00000000-0000-4000-8000-000000000001", "", 403},
		{"employee", "/products/1", "", 403}, {"stock", "/export.csv", "", 403},
		{"owner", "/summary?limit=1", "", 400}, {"owner", "/entries?change=bad", "", 400},
		{"owner", "/entries/bad", "", 400}, {"owner", "/entries/00000000-0000-4000-8000-000000000001?q=x", "", 400},
		{"owner", "/products/01", "", 400}, {"owner", "/export.csv?tenant_id=x", "", 400}, {"owner", "/summary", "{}", 400},
	} {
		t.Run(tc.role+tc.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("GET", "/api/v1/catalog/cost-history"+tc.path, strings.NewReader(tc.body))
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
