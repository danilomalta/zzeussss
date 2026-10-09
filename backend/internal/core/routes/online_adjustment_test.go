package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdjustmentMountedRoutesRejectCashierStockAndAmbiguousBodyBeforeWrites(t *testing.T) {
	for _, tc := range []struct {
		role, path, body string
		status           int
	}{
		{"cashier", "/api/v1/catalog/adjustments/preview", `{}`, 403},
		{"stock", "/api/v1/catalog/adjustments/apply", `{}`, 403},
		{"owner", "/api/v1/catalog/adjustments/preview", `{"operation_id":"x","items":[]}`, 400},
		{"owner", "/api/v1/catalog/adjustments/apply", `{"operation_id":"11111111-1111-4111-8111-111111111111","items":[{"product_id":1,"action":"price","price_cents":10,"expected_version":1}]}`, 400},
		{"owner", "/api/v1/catalog/adjustments/preview?tenant_id=other", `{}`, 400},
	} {
		t.Run(tc.role+tc.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", tc.role))
			r, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			r.Body.Close()
			if r.StatusCode != tc.status {
				t.Fatal(r.StatusCode)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
