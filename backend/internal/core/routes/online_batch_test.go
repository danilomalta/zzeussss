package routes

import (
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBatchMountedRoutesRejectCashierStockAndAmbiguousBodyBeforeWrites(t *testing.T) {
	for _, tc := range []struct {
		role, path, body string
		status           int
	}{
		{"cashier", "/api/v1/catalog/batches/preview", `{}`, 403},
		{"stock", "/api/v1/catalog/batches/apply", `{}`, 403},
		{"owner", "/api/v1/catalog/batches/preview", `{"operation_id":"x","items":[]}`, 400},
		{"owner", "/api/v1/catalog/batches/apply", `{"operation_id":"11111111-1111-4111-8111-111111111111","items":[{"product_id":1,"action":"price","price_cents":10,"expected_version":1}]}`, 400},
		{"owner", "/api/v1/catalog/batches/preview?tenant_id=other", `{}`, 400},
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
func TestBatchHTTPPreviewShowsBeforeAfterAndNoStoreWrite(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	m := onlineMock(t)
	tenant := "22222222-2222-4222-8222-222222222222"
	user := "33333333-3333-4333-8333-333333333333"
	session := "44444444-4444-4444-8444-444444444444"
	m.ExpectQuery(`SELECT EXISTS`).WithArgs(user, tenant, "owner", session).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectBegin()
	m.ExpectQuery(`SELECT s.id::text`).WithArgs(session, tenant, user, "owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(session))
	m.ExpectQuery(`SELECT id,nome`).WithArgs(tenant, int64(8)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}).AddRow(8, "Item", "", "ABC", "2.50", true, 1))
	m.ExpectCommit()
	token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user, "tenant_id": tenant, "role": "owner", "sid": session, "type": "access", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("isolated-test-secret"))
	if e != nil {
		t.Fatal(e)
	}
	app := fiber.New()
	Registrar(app)
	req := httptest.NewRequest("POST", "/api/v1/catalog/batches/preview", strings.NewReader(`{"operation_id":"11111111-1111-4111-8111-111111111111","items":[{"product_id":8,"action":"price","price_cents":375,"expected_version":1}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	data, _ := io.ReadAll(r.Body)
	var p struct {
		PreviewHash string `json:"preview_hash"`
		Items       []struct {
			Before, After struct {
				PriceCents int64 `json:"price_cents"`
			}
		}
	}
	if r.StatusCode != 200 || json.Unmarshal(data, &p) != nil || len(p.PreviewHash) != 64 || len(p.Items) != 1 || p.Items[0].Before.PriceCents != 250 || p.Items[0].After.PriceCents != 375 || r.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("preview contract", r.StatusCode)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
