package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogManagementRejectsInputBeforeAnyMutation(t *testing.T) {
	for _, test := range []struct {
		path, body string
		status     int
	}{
		{"/api/v1/produtos/1/price", `{"operation_id":"11111111-1111-4111-8111-111111111111","expected_version":1,"price_cents":2.5}`, 400},
		{"/api/v1/produtos/1/price", `{"operation_id":"11111111-1111-4111-8111-111111111111","expected_version":1,"price_cents":250,"tenant_id":"foreign"}`, 400},
		{"/api/v1/produtos/1/active", `{"operation_id":"11111111-1111-4111-8111-111111111111","expected_version":1,"ativo":null}`, 400},
		{"/api/v1/produtos/01/details", `{}`, 400},
		{"/api/v1/produtos/1/price?tenant_id=foreign", `{}`, 400},
	} {
		t.Run(test.path+test.body, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("POST", test.path, strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
			res, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			defer res.Body.Close()
			if res.StatusCode != test.status {
				t.Fatal(res.StatusCode)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestCatalogPriceAndHistoryRequireManagementRole(t *testing.T) {
	for _, test := range []struct{ method, path string }{{"POST", "/api/v1/produtos/1/price"}, {"POST", "/api/v1/produtos/1/active"}, {"GET", "/api/v1/produtos/1/history"}} {
		t.Run(test.path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "stock"))
			res, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			res.Body.Close()
			if res.StatusCode != 403 {
				t.Fatal(res.StatusCode)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestCatalogIndividualLookupScopesTenantAndReturnsExactCents(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	m := onlineMock(t)
	m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectQuery(`SELECT id,nome`).WithArgs("company-a", int64(42)).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "descricao", "sku", "preco", "ativo", "version"}).AddRow(42, "Item", "", "ABC", "9999999999.99", false, 8))
	app := fiber.New()
	Registrar(app)
	req := httptest.NewRequest("GET", "/api/v1/produtos/42", nil)
	req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "cashier"))
	res, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), `"price_cents":999999999999`) || !strings.Contains(string(body), `"ativo":false`) {
		t.Fatal(string(body))
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
