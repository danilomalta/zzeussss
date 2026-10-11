package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscountReportMountedPermissionsAndInput(t *testing.T) {
	cases := []struct {
		role, method, path, body string
		want                     int
	}{
		{"cashier", "GET", "/discounts/report/summary", "", 403},
		{"stock", "GET", "/discounts/report/pending", "", 403},
		{"employee", "GET", "/discounts/report/approved", "", 403},
		{"accountant", "GET", "/discounts/report/rejected", "", 403},
		{"stock", "GET", "/discounts/report/export.csv", "", 403},
		{"owner", "GET", "/discounts/report/summary?limit=1", "", 400},
		{"owner", "GET", "/discounts/report/summary?q=a&q=b", "", 400},
		{"owner", "GET", "/discounts/report/pending?status=APPROVED", "", 400},
		{"owner", "GET", "/discounts/report/approved?limit=101", "", 400},
		{"owner", "GET", "/discounts/report/rejected", "{}", 400},
		{"owner", "GET", "/discounts/report/export.csv?status=INVALID", "", 400},
		{"owner", "GET", "/discounts/report/export.csv?tenant_id=x", "", 400},
		{"owner", "GET", "/discounts/report/pending?offset=-1", "", 400},
		{"owner", "GET", "/discounts/report/pending?active=unknown", "", 400},
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
