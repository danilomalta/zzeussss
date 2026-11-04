package delivery

import (
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"titansystem-backend/internal/core/database"
)

func TestLogoutCookieMatchesIssuanceAndRequiresConfirmedCommit(t *testing.T) {
	const user = "22222222-2222-4222-8222-222222222222"
	const tenant = "33333333-3333-4333-8333-333333333333"
	const id = "11111111-1111-4111-8111-111111111112"
	t.Setenv("JWT_SECRET", jwtFixtureKey)
	for _, secure := range []string{"", "false"} {
		for _, fault := range []string{"none", "audit", "commit"} {
			t.Run(secure+"/"+fault, func(t *testing.T) {
				t.Setenv("COOKIE_SECURE", secure)
				conn, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal("fixture connection failed")
				}
				defer conn.Close()
				db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				if err != nil {
					t.Fatal("fixture ORM failed")
				}
				old := database.DB
				database.DB = db
				defer func() { database.DB = old }()
				now := time.Now()
				mock.ExpectBegin()
				mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(tenant + ":" + user).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(`SELECT id, expires_at, revoked_at.*ORDER BY id FOR UPDATE`).WithArgs(tenant, user, id, false, id).WillReturnRows(sqlmock.NewRows([]string{"id", "expires", "revoked"}).AddRow(id, now.Add(time.Hour), nil))
				mock.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
				mock.ExpectQuery(`SELECT u.name, u.password_hash.*FOR SHARE OF u, t`).WithArgs(user, tenant, "owner").WillReturnRows(sqlmock.NewRows([]string{"name", "hash"}).AddRow("Fixture", "fixture-hash"))
				mock.ExpectExec(`UPDATE online_sessions SET revoked_at`).WithArgs(id, tenant, user).WillReturnResult(sqlmock.NewResult(0, 1))
				a := mock.ExpectExec(`INSERT INTO online_session_audit`).WithArgs(sqlmock.AnyArg(), tenant, user, id, "logout")
				if fault == "audit" {
					a.WillReturnError(errors.New("private fixture detail"))
					mock.ExpectRollback()
				} else {
					a.WillReturnResult(sqlmock.NewResult(0, 1))
					commit := mock.ExpectCommit()
					if fault == "commit" {
						commit.WillReturnError(errors.New("private uncertain commit"))
					}
				}
				app := fiber.New()
				app.Post("/logout", func(c *fiber.Ctx) error {
					c.Locals("userID", user)
					c.Locals("tenant_id", tenant)
					c.Locals("role", "owner")
					c.Locals("session_id", id)
					return c.Next()
				}, NewAuthHandler(nil).Logout)
				resp, err := app.Test(httptest.NewRequest("POST", "/logout", nil))
				if err != nil {
					t.Fatal("logout request failed")
				}
				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				want := 503
				if fault == "none" {
					want = 204
				}
				if err != nil || resp.StatusCode != want || resp.Header.Get("Cache-Control") != "no-store" || strings.Contains(string(body), "private") || strings.Contains(string(body), jwtFixtureKey) {
					t.Fatal("unsafe logout response")
				}
				if fault == "none" {
					cookies := resp.Cookies()
					issued := cookieRenovacao("fixture-refresh", now.Add(time.Hour))
					if len(cookies) != 1 {
						t.Fatal("missing expiry cookie")
					}
					cleared := cookies[0]
					if cleared.Name != issued.Name || cleared.Path != issued.Path || cleared.Domain != issued.Domain || cleared.Secure != issued.Secure || cleared.HttpOnly != issued.HTTPOnly || cleared.Value != "" || !cleared.Expires.Before(now) || cleared.MaxAge > 0 {
						t.Fatal("expiry cookie incompatible with issuance")
					}
					expectedSite := "SameSite=Strict"
					if secure == "false" {
						expectedSite = "SameSite=Lax"
					}
					if !strings.Contains(resp.Header.Get("Set-Cookie"), expectedSite) {
						t.Fatal("cookie SameSite changed")
					}
				} else if resp.Header.Get("Set-Cookie") != "" {
					t.Fatal("uncertain/failed commit expired cookie")
				}
				if err = mock.ExpectationsWereMet(); err != nil {
					t.Fatal("unexpected database effects")
				}
			})
		}
	}
}
