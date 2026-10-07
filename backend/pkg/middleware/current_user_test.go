package middleware

import (
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

func TestCurrentUserAndRoleOnProtectedRoute(t *testing.T) {
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
	app := fiber.New()
	app.Post("/produtos", AuthGuard(), CurrentUser(), RequireRoles("admin", "owner", "manager", "stock"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	for _, tc := range []struct {
		name   string
		role   string
		active bool
		want   int
	}{
		{"admin ativo", "admin", true, 204},
		{"caixa ativo sem permissão de cadastro", "cashier", true, 403},
		{"vínculo revogado", "admin", false, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"sid": "11111111-1111-4111-8111-111111111112",
				"sub": "operador", "tenant_id": "empresa-a", "role": tc.role,
				"type": "access", "exp": time.Now().Add(time.Minute).Unix(),
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret-test-only"))
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("operador", "empresa-a", tc.role, "11111111-1111-4111-8111-111111111112").
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tc.active))
			req := httptest.NewRequest("POST", "/produtos", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("esperado %d, recebido %d", tc.want, resp.StatusCode)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
