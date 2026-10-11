package delivery

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"testing"
	"titansystem-backend/internal/onlinecatalog"
)

func TestMarginQueryDecodesPercentOnceAndPreservesLiteralSearch(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		q, e := marginQuery(c, "missing_cost", false, false)
		if e != nil {
			return c.SendStatus(400)
		}
		return c.JSON(q)
	})
	for _, tc := range []struct{ url, want string }{
		{"/?q=escape40-%25literal", "escape40-%literal"},
		{"/?q=escape40-%2525literal", "escape40-%25literal"},
		{"/?q=escape40-%5Fliteral", "escape40-_literal"},
	} {
		resp, e := app.Test(httptest.NewRequest("GET", tc.url, nil))
		if e != nil {
			t.Fatal(e)
		}
		var q onlinecatalog.MarginQuery
		e = json.NewDecoder(resp.Body).Decode(&q)
		resp.Body.Close()
		if e != nil || resp.StatusCode != 200 || q.Query != tc.want {
			t.Fatal(q.Query, tc.want, e)
		}
	}
	resp, e := app.Test(httptest.NewRequest("GET", "/?q=escape40-%25literal&q=other", nil))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatal("repeated search accepted")
	}
}
