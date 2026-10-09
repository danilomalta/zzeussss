package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManagedCreationHTTPReturnsPersistedProduct(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	m := onlineMock(t)
	tenant := "22222222-2222-4222-8222-222222222222"
	user := "33333333-3333-4333-8333-333333333333"
	session := "44444444-4444-4444-8444-444444444444"
	op := "11111111-1111-4111-8111-111111111111"
	m.ExpectQuery(`SELECT EXISTS`).WithArgs(user, tenant, "owner", session).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectBegin()
	m.ExpectQuery(`SELECT s.id::text`).WithArgs(session, tenant, user, "owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(session))
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(tenant + ":create:" + op).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT operation_id::text`).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "snapshot", "created", "hash"}))
	m.ExpectQuery(`INSERT INTO products`).WithArgs(tenant, "Item", "", "ABC", "2.50", int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version", "stock", "created", "updated"}).AddRow(8, "Item", "", "ABC", "2.50", true, 1, 1, time.Now(), time.Now()))
	m.ExpectQuery(`INSERT INTO online_catalog_creations`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
	m.ExpectExec(`INSERT INTO online_catalog_creation_outbox`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	access, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user, "tenant_id": tenant, "role": "owner", "sid": session, "type": "access", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("isolated-test-secret"))
	if e != nil {
		t.Fatal(e)
	}
	app := fiber.New()
	Registrar(app)
	req := httptest.NewRequest("POST", "/api/v1/produtos/", strings.NewReader(`{"nome":"Item","sku":"ABC","preco":2.50,"estoque":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Idempotency-Key", op)
	r, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	if r.StatusCode != 201 || !strings.Contains(string(body), `"ID":8`) || !strings.Contains(string(body), `"price_cents":250`) {
		t.Fatal(r.StatusCode, string(body))
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestSearchRejectsDuplicateForeignOrUnknownFilters(t *testing.T) {
	for _, q := range []string{"q=a&q=b", "tenant_id=other", "active=wrong", "limit=101"} {
		t.Run(q, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("GET", "/api/v1/produtos/search?"+q, nil)
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
			r, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			r.Body.Close()
			if r.StatusCode != 400 {
				t.Fatal(r.StatusCode)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
