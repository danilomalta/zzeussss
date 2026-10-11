package delivery

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"testing"
	"titansystem-backend/internal/onlinecatalog"
)

func TestCostReportQueryStrictAndLiteral(t *testing.T) {
	app := fiber.New()
	app.Get("/summary", func(c *fiber.Ctx) error {
		q, e := costReportQuery(c, true, false)
		if e != nil {
			return c.SendStatus(400)
		}
		return c.JSON(q)
	})
	app.Get("/entries", func(c *fiber.Ctx) error {
		q, e := costReportQuery(c, false, false)
		if e != nil {
			return c.SendStatus(400)
		}
		return c.JSON(q)
	})
	app.Get("/products/:id", func(c *fiber.Ctx) error {
		q, e := costReportQuery(c, false, true)
		if e != nil {
			return c.SendStatus(400)
		}
		return c.JSON(q)
	})
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/entries?q=cost41-%25literal", 200}, {"/entries?from=2026-10-11T00:00:00Z&until=2026-10-12T00:00:00Z", 200},
		{"/entries?change=", 400}, {"/entries?actor_id=", 400}, {"/entries?from=", 400}, {"/entries?product_id=0", 400},
		{"/entries?tenant_id=x", 400}, {"/entries?q=a&q=b", 400}, {"/entries?limit=101", 400}, {"/entries?until=yesterday", 400},
		{"/summary?limit=1", 400}, {"/summary?offset=0", 400}, {"/products/1?product_id=2", 400}, {"/products/01", 400}, {"/products/1", 200},
	} {
		resp, e := app.Test(httptest.NewRequest("GET", tc.path, nil))
		if e != nil {
			t.Fatal(e)
		}
		if tc.want == 200 && tc.path == "/entries?q=cost41-%25literal" {
			var q onlinecatalog.CostReportQuery
			if json.NewDecoder(resp.Body).Decode(&q) != nil || q.Query != "cost41-%literal" {
				t.Fatal(q)
			}
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatal(tc.path, resp.StatusCode)
		}
	}
}
