package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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

	claims := jwt.MapClaims{
		"sub": "operador", "tenant_id": "empresa-a", "role": "admin",
		"name": "Operador", "type": "refresh", "exp": time.Now().Add(time.Minute).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret-test-only"))
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs("operador", "empresa-a", "admin").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
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
