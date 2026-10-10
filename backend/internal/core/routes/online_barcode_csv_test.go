package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBarcodeCSVMountedRoutesRejectUnauthorizedAndAmbiguousInputsBeforeWrites(t *testing.T) {
	cases := []struct {
		role, method, path, body string
		status                   int
	}{
		{"cashier", "POST", "/api/v1/catalog/barcodes/import/preview", `{}`, 403},
		{"employee", "GET", "/api/v1/catalog/barcodes/import/template", "", 403},
		{"cashier", "GET", "/api/v1/produtos/1/barcodes/export.csv", "", 403},
		{"owner", "POST", "/api/v1/catalog/barcodes/import/preview", `{}`, 400},
		{"owner", "POST", "/api/v1/catalog/barcodes/import/apply", `{}`, 400},
		{"stock", "POST", "/api/v1/catalog/barcodes/import/apply?tenant_id=x", `{}`, 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/import/template?x=1", "", 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/import/template", `{}`, 400},
		{"owner", "GET", "/api/v1/produtos/01/barcodes/export.csv", "", 400},
		{"owner", "GET", "/api/v1/produtos/1/barcodes/export.csv?state=active", "", 400},
		{"owner", "GET", "/api/v1/produtos/1/barcodes/export.csv", `{}`, 400},
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
