package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQualityRoutesRejectRolesAndAmbiguousQueries(t *testing.T) {
	paths := []string{"summary", "no-active-gtin", "no-description", "zero-stock", "export.csv"}
	for _, path := range paths {
		for _, role := range []string{"cashier", "employee", "accountant"} {
			t.Run(path+role, func(t *testing.T) {
				t.Setenv("JWT_SECRET", "isolated-test-secret")
				m := onlineMock(t)
				m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				app := fiber.New()
				Registrar(app)
				req := httptest.NewRequest("GET", "/api/v1/catalog/quality/"+path, nil)
				req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", role))
				resp, e := app.Test(req)
				if e != nil {
					t.Fatal(e)
				}
				resp.Body.Close()
				if resp.StatusCode != 403 {
					t.Fatal(resp.StatusCode)
				}
				if e = m.ExpectationsWereMet(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
	for _, path := range []string{
		"summary?limit=1", "summary?q=a&q=b", "summary?tenant_id=x", "summary?active=bad",
		"no-active-gtin?issue=zero_stock", "no-description?limit=101", "zero-stock?offset=-1",
		"export.csv?issue=fiscal", "export.csv?active=all&active=inactive", "export.csv?store_id=x",
		"zero-stock?limit=",
	} {
		t.Run(path, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("GET", "/api/v1/catalog/quality/"+path, nil)
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
			resp, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			resp.Body.Close()
			if resp.StatusCode != 400 {
				t.Fatal(resp.StatusCode)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
	t.Run("get_body", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "isolated-test-secret")
		m := onlineMock(t)
		m.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		app := fiber.New()
		Registrar(app)
		req := httptest.NewRequest("GET", "/api/v1/catalog/quality/summary", strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "stock"))
		resp, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Fatal(resp.StatusCode)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	})
}
func TestQualityRoutesRequireAuthentication(t *testing.T) {
	app := fiber.New()
	Registrar(app)
	for _, path := range []string{"summary", "no-active-gtin", "no-description", "zero-stock", "export.csv"} {
		resp, e := app.Test(httptest.NewRequest("GET", "/api/v1/catalog/quality/"+path, nil))
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Fatal(path, resp.StatusCode)
		}
	}
}
