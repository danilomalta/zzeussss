package routes

import (
	"database/sql/driver"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"titansystem-backend/internal/core/database"
)

func TestOnlineProductCreationCannotSelectAnotherCompany(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	mock := onlineMock(t)
	mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", "company-a", "owner", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectBegin()
	args := make([]driver.Value, 17)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[3] = "company-a"
	args[4] = "Item"
	args[6] = float64(2.5)
	args[7] = "sku"
	mock.ExpectQuery(`INSERT INTO "products"`).WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()
	app := fiber.New()
	Registrar(app)
	req := httptest.NewRequest("POST", "/api/v1/produtos/", strings.NewReader(`{"nome":"Item","sku":"sku","preco":2.50,"estoque":1,"tenant_id":"company-b"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("recebido %d", resp.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func onlineMock(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; conn.Close() })
	return mock
}

func onlineToken(t *testing.T, tenant, role string) string {
	t.Helper()
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sid": "11111111-1111-4111-8111-111111111112",
		"sub": "operator", "tenant_id": tenant, "role": role, "type": "access", "exp": time.Now().Add(time.Minute).Unix(),
	}).SignedString([]byte("isolated-test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestOnlineImplementedRoutesRecheckMembershipBeforeBusinessQueries(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/produtos/"}, {"POST", "/api/v1/produtos/"},
		{"POST", "/api/v1/discounts/suggest"}, {"GET", "/api/v1/discounts/suggestions"},
	} {
		for _, scenario := range []string{"anonymous", "foreign-or-revoked", "role-denied", "database-failure"} {
			t.Run(route.method+route.path+scenario, func(t *testing.T) {
				mock := onlineMock(t)
				app := fiber.New()
				Registrar(app)
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{"tenant_id":"foreign"}`))
				req.Header.Set("Content-Type", "application/json")
				want := 401
				if scenario != "anonymous" {
					role := "owner"
					if scenario == "role-denied" {
						role = "employee"
						want = 403
					}
					req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", role))
					expect := mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", "company-a", role, "11111111-1111-4111-8111-111111111112")
					if scenario == "database-failure" {
						expect.WillReturnError(errors.New("private database detail"))
						want = 503
					} else {
						expect.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(scenario == "role-denied"))
					}
				}
				resp, err := app.Test(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != want {
					t.Fatalf("recebido %d esperado %d", resp.StatusCode, want)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestOnlineReadsAndDiscountGenerationUseAuthenticatedCompany(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	for _, tenant := range []string{"company-a", "company-b"} {
		for _, route := range []string{"/api/v1/produtos/", "/api/v1/discounts/suggestions", "/api/v1/discounts/suggest"} {
			t.Run(tenant+route, func(t *testing.T) {
				mock := onlineMock(t)
				mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", tenant, "owner", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				method := "GET"
				switch route {
				case "/api/v1/produtos/":
					mock.ExpectQuery(`SELECT .* FROM "products" WHERE tenant_id = \$1`).WithArgs(tenant).WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "nome", "preco"}).AddRow(1, tenant, "Item da empresa", 2.5))
				case "/api/v1/discounts/suggestions":
					mock.ExpectQuery(`SELECT .* FROM "discount_suggestions" WHERE \(tenant_id = \$1 AND status = \$2\)`).WithArgs(tenant, "PENDING").WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "status"}))
				case "/api/v1/discounts/suggest":
					method = "POST"
					mock.ExpectBegin()
					mock.ExpectQuery(`SELECT .* FROM "products" WHERE \(tenant_id = \$1 AND ativo = \$2\)`).WithArgs(tenant, true).WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id"}))
					mock.ExpectCommit()
				}
				app := fiber.New()
				Registrar(app)
				req := httptest.NewRequest(method, route+"?tenant_id=foreign", nil)
				req.Header.Set("Authorization", "Bearer "+onlineToken(t, tenant, "owner"))
				resp, err := app.Test(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != 200 {
					t.Fatalf("recebido %d", resp.StatusCode)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
