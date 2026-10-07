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
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"titansystem-backend/internal/core/database"
)

func TestPasswordRejectsAmbiguousInputBeforeDatabase(t *testing.T) {
	old := database.DB
	database.DB = nil
	t.Cleanup(func() { database.DB = old })
	good := `{"current_password":"old-test-password","new_password":"new-test-password"}`
	cases := []struct {
		body, media string
		status      int
	}{
		{good, "application/json", 503},
		{good, "text/plain", 415},
		{`{"current_password":"secret-submitted","new_password":"short"}`, "application/json", 400},
		{`{"current_password":"a","current_password":"b","new_password":"new-test-password"}`, "application/json", 400},
		{`{"current_password":null,"new_password":"new-test-password"}`, "application/json", 400},
		{`{"current_password":"old-test-password","new_password":"new-test-password","tenant_id":"foreign"}`, "application/json", 400},
		{good + ` {}`, "application/json", 400},
		{`{"current_password":"a","new_password":{}}`, "application/json", 400},
		{`{"new_password":"new-test-password"}`, "application/json", 400},
		{strings.Repeat("x", 2049), "application/json", 413},
		{string([]byte{0xff}), "application/json", 400},
	}
	for _, tc := range cases {
		app := fiber.New()
		app.Post("/password", NewAuthHandler(nil).ChangePassword)
		req := httptest.NewRequest("POST", "/password", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.media)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.status || resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Set-Cookie") != "" || strings.Contains(string(body), "secret-submitted") || strings.Contains(string(body), "old-test-password") || strings.Contains(string(body), "new-test-password") {
			t.Fatalf("input response: %d expected %d", resp.StatusCode, tc.status)
		}
	}
}

func TestPasswordCookieExpiresOnlyAfterSuccessfulCommit(t *testing.T) {
	const user = "22222222-2222-4222-8222-222222222222"
	const tenant = "33333333-3333-4333-8333-333333333333"
	const id = "11111111-1111-4111-8111-111111111112"
	hash, err := bcrypt.GenerateFromPassword([]byte("old-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_SECRET", "test-only-secret")
	for _, fault := range []string{"none", "audit", "commit"} {
		t.Run(fault, func(t *testing.T) {
			conn, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			db, e := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if e != nil {
				t.Fatal(e)
			}
			old := database.DB
			database.DB = db
			defer func() { database.DB = old }()
			now := time.Now()
			m.ExpectBegin()
			m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(tenant + ":" + user).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT u.password_hash.*FOR UPDATE OF u FOR SHARE OF t`).WithArgs(user, tenant, "owner").WillReturnRows(sqlmock.NewRows([]string{"hash"}).AddRow(string(hash)))
			m.ExpectQuery(`SELECT id, role, expires_at, revoked_at`).WithArgs(tenant, user).WillReturnRows(sqlmock.NewRows([]string{"id", "role", "expires", "revoked"}).AddRow(id, "owner", now.Add(time.Hour), nil))
			m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
			m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(now))
			m.ExpectExec(`UPDATE users SET password_hash`).WithArgs(sqlmock.AnyArg(), user, tenant, string(hash)).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`UPDATE online_sessions SET revoked_at`).WithArgs(id, tenant, user).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`INSERT INTO online_session_audit`).WithArgs(sqlmock.AnyArg(), tenant, user, id, "revoked").WillReturnResult(sqlmock.NewResult(0, 1))
			a := m.ExpectExec(`INSERT INTO online_password_changes`).WithArgs(sqlmock.AnyArg(), tenant, user, id, 1)
			if fault == "audit" {
				a.WillReturnError(errors.New("private detail"))
				m.ExpectRollback()
			} else {
				a.WillReturnResult(sqlmock.NewResult(0, 1))
				if fault == "commit" {
					m.ExpectCommit().WillReturnError(errors.New("uncertain"))
				} else {
					m.ExpectCommit()
				}
			}
			app := fiber.New()
			app.Post("/password", func(c *fiber.Ctx) error {
				c.Locals("userID", user)
				c.Locals("tenant_id", tenant)
				c.Locals("role", "owner")
				c.Locals("session_id", id)
				return c.Next()
			}, NewAuthHandler(nil).ChangePassword)
			req := httptest.NewRequest("POST", "/password", strings.NewReader(`{"current_password":"old-test-password","new_password":"new-test-password"}`))
			req.Header.Set("Content-Type", "application/json")
			resp, e := app.Test(req, 5000)
			if e != nil {
				t.Fatal(e)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			want := 503
			if fault == "none" {
				want = 204
			}
			cookie := resp.Header.Get("Set-Cookie")
			if resp.StatusCode != want || strings.Contains(string(body), "password") || strings.Contains(string(body), "private") {
				t.Fatalf("response %d", resp.StatusCode)
			}
			if fault == "none" {
				if !strings.Contains(cookie, "titan_session_rt=") || !strings.Contains(strings.ToLower(cookie), "httponly") {
					t.Fatal("cookie not expired")
				}
			} else if cookie != "" {
				t.Fatal("failure changed cookie")
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
