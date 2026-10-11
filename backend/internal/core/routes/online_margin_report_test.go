package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarginReportMountedPermissionsAndInput(t *testing.T) {
	cases := []struct {
		role, method, path, body string
		want                     int
	}{
		{"stock", "GET", "/catalog/margins/summary", "", 403},
		{"cashier", "GET", "/catalog/margins/missing-cost", "", 403},
		{"employee", "GET", "/catalog/margins/negative", "", 403},
		{"accountant", "GET", "/catalog/margins/below-target", "", 403},
		{"stock", "GET", "/catalog/margins/export.csv", "", 403},
		{"owner", "GET", "/catalog/margins/summary?limit=1", "", 400},
		{"owner", "GET", "/catalog/margins/summary?q=a&q=b", "", 400},
		{"owner", "GET", "/catalog/margins/summary?active=yes", "", 400},
		{"owner", "GET", "/catalog/margins/missing-cost?issue=negative", "", 400},
		{"owner", "GET", "/catalog/margins/negative?limit=101", "", 400},
		{"owner", "GET", "/catalog/margins/below-target?target_bps=1.5", "", 400},
		{"owner", "GET", "/catalog/margins/below-target?target_bps=10001", "", 400},
		{"owner", "GET", "/catalog/margins/below-target?target_bps=", "", 400},
		{"owner", "GET", "/catalog/margins/export.csv?issue=unknown", "", 400},
		{"owner", "GET", "/catalog/margins/export.csv?issue=", "", 400},
		{"owner", "GET", "/catalog/margins/summary?active=", "", 400},
		{"owner", "GET", "/catalog/margins/export.csv?tenant_id=x", "", 400},
		{"owner", "GET", "/catalog/margins/summary", "{}", 400},
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
