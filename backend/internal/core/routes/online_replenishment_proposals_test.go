package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReplenishmentProposalsMountedPermissionsAndStrictInputs(t *testing.T) {
	for _, tc := range []struct {
		role, method, path, body string
		want                     int
	}{
		{"stock", "POST", "/proposals", "{}", 403}, {"cashier", "GET", "/proposals/11111111-1111-4111-8111-111111111111", "", 403}, {"stock", "POST", "/proposals/11111111-1111-4111-8111-111111111111/approve", "{}", 403}, {"employee", "POST", "/proposals/11111111-1111-4111-8111-111111111111/cancel", "{}", 403}, {"accountant", "GET", "/operations/11111111-1111-4111-8111-111111111111", "", 403},
		{"owner", "POST", "/proposals", "{}", 400}, {"owner", "GET", "/proposals/bad", "", 400}, {"owner", "POST", "/proposals/11111111-1111-4111-8111-111111111111/approve", "{}", 400}, {"manager", "POST", "/proposals/11111111-1111-4111-8111-111111111111/cancel?q=x", "{}", 400}, {"owner", "GET", "/operations/11111111-1111-4111-8111-111111111111", "{}", 400},
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
