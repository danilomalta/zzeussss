package middleware

import (
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsCreationIdentityOnlyForConfiguredOrigin(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:3000")
	for _, origin := range []string{"http://localhost:3000", "http://untrusted.invalid"} {
		app := fiber.New()
		app.Use(CORS())
		app.Post("/api/v1/produtos/", func(c *fiber.Ctx) error { return c.SendStatus(204) })
		r := httptest.NewRequest("OPTIONS", "/api/v1/produtos/", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "authorization,content-type,idempotency-key")
		response, e := app.Test(r)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if origin == "http://localhost:3000" && !strings.Contains(strings.ToLower(response.Header.Get("Access-Control-Allow-Headers")), "idempotency-key") {
			t.Fatal("header blocked")
		}
		if origin != "http://localhost:3000" && response.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("foreign origin allowed")
		}
	}
}
