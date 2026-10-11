package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSuppliersMountedPermissionsAndStrictInputs(t *testing.T) {
	for _, tc := range []struct {
		role, method, path, body string
		want                     int
	}{
		{"stock", "POST", "", "{}", 403}, {"cashier", "GET", "/11111111-1111-4111-8111-111111111111", "", 403}, {"stock", "PUT", "/11111111-1111-4111-8111-111111111111", "{}", 403}, {"employee", "PUT", "/11111111-1111-4111-8111-111111111111", "{}", 403}, {"accountant", "GET", "/operations/11111111-1111-4111-8111-111111111111", "", 403},
		{"owner", "POST", "", "{}", 400}, {"owner", "GET", "/bad", "", 400}, {"owner", "PUT", "/11111111-1111-4111-8111-111111111111", "{}", 400}, {"manager", "PUT", "/11111111-1111-4111-8111-111111111111?q=x", "{}", 400}, {"owner", "GET", "/operations/11111111-1111-4111-8111-111111111111", "{}", 400},
	} {
		t.Run(tc.role+tc.method+tc.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest(tc.method, "/api/v1/suppliers"+tc.path, strings.NewReader(tc.body))
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
