package routes

import (
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"testing"
	"titansystem-backend/pkg/middleware"
)

// Business fixtures use separate apps for separate scenarios. Every app still
// carries the real limiter: reset test state, never bypass production policy.
func TestFreshCatalogHarnessPreservesProductionRateLimit(t *testing.T) {
	newHarness := func() *fiber.App {
		app := fiber.New()
		app.Use(middleware.RateLimitGeral())
		app.Get("/probe", func(c *fiber.Ctx) error { return c.SendStatus(200) })
		return app
	}
	app := newHarness()
	for i := 0; i < 101; i++ {
		req := httptest.NewRequest("GET", "/probe", nil)
		resp, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		want := 200
		if i == 100 {
			want = 429
		}
		if resp.StatusCode != want {
			t.Fatalf("request %d status %d want %d", i+1, resp.StatusCode, want)
		}
	}
	fresh := newHarness()
	resp, e := fresh.Test(httptest.NewRequest("GET", "/probe", nil))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("fresh harness inherited prior rate budget")
	}
}
