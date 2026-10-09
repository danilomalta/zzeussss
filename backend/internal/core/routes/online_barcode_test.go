package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBarcodeMountedRoutesRejectUnauthorizedAndAmbiguousInputsBeforeWrites(t *testing.T) {
	cases := []struct {
		role, method, path, body string
		status                   int
	}{
		{"cashier", "POST", "/api/v1/produtos/1/barcodes", `{}`, 403},
		{"stock", "GET", "/api/v1/produtos/1/barcodes/history", "", 403},
		{"employee", "GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931", "", 403},
		{"owner", "GET", "/api/v1/catalog/barcodes/lookup", "", 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/lookup?code=4006381333932", "", 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931&code=96385074", "", 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931&tenant_id=x", "", 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/lookup?code=4006381333931", `{}`, 400},
		{"owner", "GET", "/api/v1/produtos/01/barcodes", "", 400},
		{"owner", "GET", "/api/v1/produtos/1/barcodes?limit=51", "", 400},
		{"owner", "GET", "/api/v1/produtos/1/barcodes?state=", "", 400},
		{"owner", "GET", "/api/v1/produtos/1/barcodes?state=active&state=all", "", 400},
		{"owner", "GET", "/api/v1/produtos/1/barcodes/history?state=all", "", 400},
		{"owner", "POST", "/api/v1/produtos/1/barcodes?x=1", `{}`, 400},
		{"owner", "POST", "/api/v1/produtos/1/barcodes", `{"operation_id":"11111111-1111-4111-8111-111111111111","code":4006381333931,"expected_version":1,"reason":"x"}`, 400},
		{"owner", "POST", "/api/v1/produtos/1/barcodes/invalid/active", `{}`, 400},
		{"owner", "GET", "/api/v1/catalog/barcodes/operations/invalid", "", 400},
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
