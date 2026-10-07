package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestAuthGuardTokenTypes(t *testing.T) {
	const secret = "chave-exclusiva-para-teste"
	t.Setenv("JWT_SECRET", secret)

	cases := []struct {
		name      string
		tokenType string
		status    int
	}{
		{"aceita access", "access", 204},
		{"rejeita refresh", "refresh", 401},
		{"rejeita sem tipo", "", 401},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"sid": "11111111-1111-4111-8111-111111111112",
				"sub":       "11111111-1111-4111-8111-111111111111",
				"tenant_id": "22222222-2222-4222-8222-222222222222",
				"role":      "admin",
				"exp":       time.Now().Add(time.Minute).Unix(),
			}
			if tc.tokenType != "" {
				claims["type"] = tc.tokenType
			}

			token, err := jwt.NewWithClaims(
				jwt.SigningMethodHS256, claims,
			).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}

			app := fiber.New()
			app.Get("/protegida", AuthGuard(), func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})

			req := httptest.NewRequest("GET", "/protegida", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.status {
				t.Fatalf("esperado %d, recebido %d",
					tc.status, resp.StatusCode)
			}
		})
	}
}

func TestAuthGuardRequiresOperatorAndRole(t *testing.T) {
	const secret = "chave-exclusiva-para-teste"
	t.Setenv("JWT_SECRET", secret)
	for _, missing := range []string{"sub", "role", "sid"} {
		t.Run(missing, func(t *testing.T) {
			claims := jwt.MapClaims{"sid": "11111111-1111-4111-8111-111111111112",
				"sub":       "11111111-1111-4111-8111-111111111111",
				"tenant_id": "22222222-2222-4222-8222-222222222222",
				"role":      "admin",
				"type":      "access",
				"exp":       time.Now().Add(time.Minute).Unix(),
			}
			delete(claims, missing)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			app := fiber.New()
			app.Get("/protegida", AuthGuard(), func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})
			req := httptest.NewRequest("GET", "/protegida", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != fiber.StatusUnauthorized {
				t.Fatalf("esperado 401, recebido %d", resp.StatusCode)
			}
		})
	}
}
