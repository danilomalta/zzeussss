package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestAuthGuardRejectsInadequateKeyWithoutDisclosure(t *testing.T) {
	for i, key := range []string{"short", strings.Repeat("x", 64), strings.Repeat("A", 43)} {
		t.Setenv("JWT_SECRET", key)
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sid": "11111111-1111-4111-8111-111111111112", "sub": "operator", "tenant_id": "company", "role": "owner", "type": "access", "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte(key))
		if err != nil {
			t.Fatal("synthetic signing failed")
		}
		app := fiber.New()
		app.Get("/private", AuthGuard(), func(c *fiber.Ctx) error {
			t.Fatal("inadequate key reached protected handler")
			return nil
		})
		req := httptest.NewRequest("GET", "/private", nil)
		req.Header.Set("Authorization", "Bearer "+raw)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal("synthetic request failed")
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 401 || resp.Header.Get("Set-Cookie") != "" || strings.Contains(string(body), key) || strings.Contains(string(body), raw) {
			t.Fatalf("inadequate JWT case %d accepted or disclosed", i)
		}
	}
}
