package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscountReviewMountedPermissionsAndInput(t *testing.T) {
	cases := []struct {
		role, method, path, body string
		want                     int
	}{
		{"cashier", "GET", "/discounts/suggestions/1", "", 403},
		{"stock", "POST", "/discounts/suggestions/1/approve", "{}", 403},
		{"employee", "GET", "/discounts/suggestions/1/reviews", "", 403},
		{"accountant", "GET", "/discounts/reviews/11111111-1111-4111-8111-111111111112", "", 403},
		{"owner", "POST", "/discounts/suggestions/1/approve", "{}", 400},
		{"owner", "POST", "/discounts/suggestions/1/reject", "{}", 400},
		{"owner", "GET", "/discounts/suggestions/01", "", 400},
		{"owner", "GET", "/discounts/suggestions/1?tenant_id=x", "", 400},
		{"owner", "GET", "/discounts/suggestions/1", "{}", 400},
		{"owner", "GET", "/discounts/suggestions/1/reviews?limit=101", "", 400},
		{"owner", "GET", "/discounts/reviews/not-uuid", "", 400},
		{"owner", "GET", "/discounts/suggestions/1/reviews?limit=1&limit=2", "", 400},
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
