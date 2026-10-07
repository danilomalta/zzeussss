package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"strings"
	"titansystem-backend/internal/core/database"
)

func TestRefreshRejectsRevokedMembership(t *testing.T) {
	t.Setenv("JWT_SECRET", "secret-test-only")
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })

	id := "11111111-1111-4111-8111-111111111112"
	token := "v1." + id + "." + strings.Repeat("A", 43)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT tenant_id, user_id FROM online_sessions`).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"tenant", "user"}).AddRow("empresa-a", "operador"))
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs("empresa-a:operador").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT tenant_id, user_id, role, expires_at, revoked_at, clock_timestamp`).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"tenant_id", "user_id", "role", "expires_at", "revoked_at", "now"}).AddRow("empresa-a", "operador", "admin", time.Now().Add(time.Hour), time.Now().Add(-time.Minute), time.Now()))
	mock.ExpectRollback()
	app := fiber.New()
	app.Post("/refresh", NewAuthHandler(nil).RefreshToken)
	req := httptest.NewRequest("POST", "/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "titan_session_rt", Value: token})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("esperado 401, recebido %d", resp.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
