package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBarcodeBatchMountedRoutesRejectUnauthorizedAndAmbiguousInputsBeforeWrites(t *testing.T) {
	cases := []struct {
		role, method, path, body string
		status                   int
	}{
		{"cashier", "POST", "/api/v1/catalog/barcodes/batches/preview", `{}`, 403},
		{"employee", "GET", "/api/v1/catalog/barcodes/batches/11111111-1111-4111-8111-111111111111", "", 403},
		{"owner", "POST", "/api/v1/catalog/barcodes/batches/preview", `{}`, 400},
		{"owner", "POST", "/api/v1/catalog/barcodes/batches/apply", `{}`, 400},
		{"stock", "POST", "/api/v1/catalog/barcodes/batches/apply?tenant_id=x", `{}`, 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/batches/invalid", "", 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/batches/11111111-1111-4111-8111-111111111111?limit=1", "", 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/batches/11111111-1111-4111-8111-111111111111", `{}`, 400},
	}
	for _, tc := range cases {
		t.Run(tc.role+tc.method+tc.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", tc.role))
			req.Header.Set("Content-Type", "application/json")
			resp, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Fatal(resp.StatusCode, tc.status)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
