package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPurchaseQueriesMountedRolesAndInputs(t *testing.T) {
	for _, tc := range []struct {
		role, method, path, body string
		want                     int
	}{
		{"stock", "GET", "/purchases/summary", "", 403}, {"stock", "GET", "/purchases", "", 403}, {"cashier", "GET", "/purchases/export.csv", "", 403}, {"employee", "GET", "/purchases/11111111-1111-4111-8111-111111111111/history", "", 403}, {"accountant", "GET", "/purchases/11111111-1111-4111-8111-111111111111/check", "", 403},
		{"owner", "GET", "/purchases/summary?state=all", "", 400}, {"owner", "GET", "/purchases?limit=101", "", 400}, {"owner", "GET", "/purchases/export.csv?tenant_id=x", "", 400}, {"manager", "GET", "/purchases/bad/history", "", 400}, {"owner", "GET", "/purchases/11111111-1111-4111-8111-111111111111/check", "{}", 400},
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
