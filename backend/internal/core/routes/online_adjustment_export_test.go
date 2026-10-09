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
	"titansystem-backend/internal/onlinecatalog"
)

func TestAdjustmentExportHTTPRejectsUnauthorizedAndAmbiguousFilters(t *testing.T) {
	for _, tc := range []struct {
		role, query, body string
		status            int
	}{{"stock", "", "", 403}, {"cashier", "", "", 403}, {"owner", "?tenant_id=other", "", 400}, {"owner", "?actor_id=x&actor_id=y", "", 400}, {"owner", "?actor_id=", "", 400}, {"owner", "?product_id=01", "", 400}, {"owner", "?product_id=", "", 400}, {"owner", "?from=2026-10-01T00:00:00Z", "", 400}, {"owner", "?until=", "", 400}, {"owner", "?limit=11", "", 400}, {"owner", "?limit=0", "", 400}, {"owner", "?offset=-1", "", 400}, {"owner", "?format=xlsx", "", 400}, {"owner", "", `{}`, 400}} {
		t.Run(tc.role+tc.query+tc.body, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("GET", "/api/v1/catalog/adjustments/exports/preview"+tc.query, strings.NewReader(tc.body))
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

func TestAdjustmentExportHTTPEmptyJSONAndCSVHaveSafeMetadata(t *testing.T) {
	for _, kind := range []string{"preview", "csv"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "isolated-test-secret")
			m := onlineMock(t)
			tenant := "22222222-2222-4222-8222-222222222222"
			user := "33333333-3333-4333-8333-333333333333"
			session := "44444444-4444-4444-8444-444444444444"
			m.ExpectQuery(`SELECT EXISTS`).WithArgs(user, tenant, "owner", session).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			m.ExpectBegin()
			m.ExpectQuery(`SELECT s.id::text`).WithArgs(session, tenant, user, "owner").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(session))
			m.ExpectQuery(`SELECT count`).WithArgs(tenant, "", nil, nil, "").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			m.ExpectQuery(`SELECT operation_id::text.*ORDER BY created_at DESC`).WithArgs(tenant, "", nil, nil, "", 10, 0).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "reason", "rate", "raw", "at", "hash"}))
			m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(time.Now()))
			m.ExpectCommit()
			access, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user, "tenant_id": tenant, "role": "owner", "sid": session, "type": "access", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("isolated-test-secret"))
			if e != nil {
				t.Fatal(e)
			}
			app := fiber.New()
			Registrar(app)
			req := httptest.NewRequest("GET", "/api/v1/catalog/adjustments/exports/"+kind, nil)
			req.Header.Set("Authorization", "Bearer "+access)
			resp, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			defer resp.Body.Close()
			body, e := io.ReadAll(resp.Body)
			if e != nil || resp.StatusCode != 200 || resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("unsafe HTTP response")
			}
			if kind == "csv" {
				if !strings.HasSuffix(string(body), "reason;rounding\n") || resp.Header.Get("Content-Disposition") != `attachment; filename="titan-catalog-adjustment-history.csv"` || resp.Header.Get("X-Catalog-Text-Prefix") != "'" || resp.Header.Get("X-Catalog-Row-Count") != "0" || resp.Header.Get("X-Catalog-Next-Offset") != "" || resp.Header.Get("Content-Type") != "text/csv; charset=utf-8" {
					t.Fatal("CSV headers wrong")
				}
			} else {
				var page onlinecatalog.AdjustmentExportPage
				if json.Unmarshal(body, &page) != nil || page.Items == nil || page.Total != 0 || page.RowCount != 0 || page.HasMore || page.NextOffset != nil || page.Limit != 10 || page.SourceHash == "" {
					t.Fatal("empty JSON page wrong")
				}
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
