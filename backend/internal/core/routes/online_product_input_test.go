package routes

import (
	"database/sql/driver"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOnlineProductInvalidInputNeverReachesInsert(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	for _, body := range []string{
		`{"nome":"Item","sku":"sku","preco":0.001}`,
		`{"nome":"Item","sku":"sku","estoque":2147483648}`,
		`{"nome":"Item","sku":"sku","tenant_id":"company-b"}`,
		`{"nome":"Item","sku":"sku","sku":"other"}`,
		`{"nome":"Item","sku":"sku","preco":null}`,
		`{"nome":"Item","sku":"sku"} true`,
	} {
		t.Run(body, func(t *testing.T) {
			mock := onlineMock(t)
			mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", "company-a", "owner", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("POST", "/api/v1/produtos/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			result, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 400 || strings.Contains(string(result), "company-b") {
				t.Fatalf("invalid result %d", resp.StatusCode)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOnlineProductInsertFailureRollsBackWithoutPrivateDetails(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	mock := onlineMock(t)
	mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", "company-b", "stock", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectBegin()
	args := make([]driver.Value, 17)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[3] = "company-b"
	args[4] = "Item"
	args[6] = float64(0.01)
	args[7] = "sku"
	args[8] = 1
	mock.ExpectQuery(`INSERT INTO "products"`).WithArgs(args...).WillReturnError(errors.New("private-sql-secret"))
	mock.ExpectRollback()
	app := fiber.New()
	Registrar(app)
	req := httptest.NewRequest("POST", "/api/v1/produtos/", strings.NewReader(`{"nome":"Item","sku":"sku","preco":0.01,"estoque":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-b", "stock"))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 409 || strings.Contains(string(body), "private-sql-secret") {
		t.Fatalf("unsafe failure response %d", resp.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
