package routes

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/onlinecatalog"
)

func TestExportHTTPRejectsRolesDuplicateUnknownAndEmptyQueries(t *testing.T) {
	for _, tc := range []struct {
		role, query, body string
		status            int
	}{{"stock", "", "", 403}, {"cashier", "", "", 403}, {"owner", "?tenant_id=other", "", 400}, {"owner", "?action=price&action=active", "", 400}, {"owner", "?action=", "", 400}, {"owner", "?ativo=", "", 400}, {"owner", "?ativo=true", "", 400}, {"owner", "?limit=101", "", 400}, {"owner", "?limit=0", "", 400}, {"owner", "?offset=-1", "", 400}, {"owner", "?format=xlsx", "", 400}, {"owner", "", `{}`, 400}} {
		t.Run(tc.role+tc.query+tc.body, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("GET", "/api/v1/catalog/exports/preview"+tc.query, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", tc.role))
			resp, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Fatal(resp.StatusCode)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestExportHTTPJSONAndCSVHaveExactBodyAndSafeDownloadHeaders(t *testing.T) {
	for _, raw := range []bool{false, true} {
		t.Run(fmt.Sprint(raw), func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			tenant := "22222222-2222-4222-8222-222222222222"
			user := "33333333-3333-4333-8333-333333333333"
			session := "44444444-4444-4444-8444-444444444444"
			m.ExpectQuery(`SELECT EXISTS`).WithArgs(user, tenant, "owner", session).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			m.ExpectBegin()
			m.ExpectQuery(`SELECT s.id::text`).WithArgs(session, tenant, user, "owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(session))
			m.ExpectQuery(`SELECT count`).WithArgs(tenant, "all", "").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
			m.ExpectQuery(`SELECT id,nome.*ORDER BY id ASC`).WithArgs(tenant, "all", "", 1, 0).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}).AddRow(8, "Item", "", "000123", "9999999999.99", true, 2))
			m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(time.Now()))
			m.ExpectCommit()
			access, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user, "tenant_id": tenant, "role": "owner", "sid": session, "type": "access", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("isolated-test-secret"))
			if e != nil {
				t.Fatal(e)
			}
			app := fiber.New()
			Registrar(app)
			path := "/api/v1/catalog/exports/preview?limit=1"
			if raw {
				path = "/api/v1/catalog/exports/csv?limit=1"
			}
			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("Authorization", "Bearer "+access)
			resp, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			defer resp.Body.Close()
			body, e := io.ReadAll(resp.Body)
			if e != nil {
				t.Fatal(e)
			}
			csv := "sku;expected_version;preco;ativo\n000123;2;9999999999.99;\n"
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(csv)))
			if resp.StatusCode != 200 || resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal(resp.StatusCode)
			}
			if raw {
				if string(body) != csv || resp.Header.Get("Content-Type") != "text/csv; charset=utf-8" || resp.Header.Get("Content-Disposition") != `attachment; filename="titan-catalog-price.csv"` || resp.Header.Get("X-Catalog-Source-Hash") != hash || resp.Header.Get("X-Catalog-Has-More") != "true" || resp.Header.Get("X-Catalog-Next-Offset") != "1" {
					t.Fatal("CSV HTTP contract")
				}
			} else {
				var page onlinecatalog.ExportPage
				if json.Unmarshal(body, &page) != nil || page.CSV != csv || page.SourceHash != hash || page.Total != 2 || page.NextOffset == nil || *page.NextOffset != 1 || len(page.Items) != 1 || page.Items[0].SKU != "000123" {
					t.Fatal("JSON HTTP contract")
				}
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
