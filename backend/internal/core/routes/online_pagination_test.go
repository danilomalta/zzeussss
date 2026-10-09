package routes

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
)

func TestOnlinePaginationBoundsAndTenant(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	for _, tenant := range []string{"company-a", "company-b"} {
		for _, kind := range []string{"products", "suggestions"} {
			t.Run(tenant+kind, func(t *testing.T) {
				mock := onlineMock(t)
				mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", tenant, "owner", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				path := "/api/v1/produtos/?limit=2&offset=3"
				if kind == "products" {
					mock.ExpectQuery(`SELECT .* FROM "products" WHERE tenant_id = \$1.*ORDER BY id desc LIMIT \$2 OFFSET \$3`).WithArgs(tenant, 2, 3).WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "nome"}).AddRow(4, tenant, "Item"))
				} else {
					path = "/api/v1/discounts/suggestions?limit=2&offset=3&status=APPROVED"
					mock.ExpectQuery(`SELECT .* FROM "discount_suggestions" WHERE \(tenant_id = \$1 AND status = \$2\).*ORDER BY id DESC LIMIT \$3 OFFSET \$4`).WithArgs(tenant, "APPROVED", 2, 3).WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "status"}).AddRow(4, tenant, "APPROVED"))
				}
				app := fiber.New()
				Registrar(app)
				req := httptest.NewRequest("GET", path, nil)
				req.Header.Set("Authorization", "Bearer "+onlineToken(t, tenant, "owner"))
				response, e := app.Test(req)
				if e != nil {
					t.Fatal(e)
				}
				defer response.Body.Close()
				if response.StatusCode != 200 {
					t.Fatal(response.StatusCode)
				}
				b, e := io.ReadAll(response.Body)
				if e != nil {
					t.Fatal(e)
				}
				if kind == "products" {
					var rows []map[string]any
					if json.Unmarshal(b, &rows) != nil || len(rows) != 1 {
						t.Fatal("product page shape")
					}
					if _, ok := rows[0]["tenant_id"]; ok {
						t.Fatal("tenant exposed")
					}
				} else {
					var page struct {
						Count int              `json:"count"`
						Rows  []map[string]any `json:"sugestoes"`
					}
					if json.Unmarshal(b, &page) != nil || page.Count != 1 || len(page.Rows) != 1 {
						t.Fatal("suggestion count must describe page")
					}
				}
				if e := mock.ExpectationsWereMet(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

func TestOnlinePaginationRejectsBeforeBusinessQuery(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	for _, path := range []string{"/api/v1/produtos/", "/api/v1/discounts/suggestions"} {
		for _, query := range []string{"limit=0", "limit=101", "limit=-1", "limit=1.5", "limit=1e2", "limit=", "offset=-1", "offset=1000000001", "offset=999999999999999999999", "limit=1&limit=2", "offset=0&offset=1", "tenant_id=foreign", "unknown=1"} {
			t.Run(path+query, func(t *testing.T) {
				mock := onlineMock(t)
				mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", "company-a", "owner", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				app := fiber.New()
				Registrar(app)
				req := httptest.NewRequest("GET", path+"?"+query, nil)
				req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
				response, e := app.Test(req)
				if e != nil {
					t.Fatal(e)
				}
				response.Body.Close()
				if response.StatusCode != 400 {
					t.Fatal("invalid query accepted", response.StatusCode)
				}
				if e := mock.ExpectationsWereMet(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

func TestOnlinePaginationEmptyProductPageAtMaximum(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	mock := onlineMock(t)
	mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", "company-a", "owner", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT .* FROM "products" WHERE tenant_id = \$1.*LIMIT \$2 OFFSET \$3`).WithArgs("company-a", 100, 1000000000).WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "nome"}))
	app := fiber.New()
	Registrar(app)
	req := httptest.NewRequest("GET", "/api/v1/produtos/?limit=100&offset=1000000000", nil)
	req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
	response, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	b, e := io.ReadAll(response.Body)
	if e != nil {
		t.Fatal(e)
	}
	if response.StatusCode != 200 || string(b) != "[]" {
		t.Fatal("empty page must be []")
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
