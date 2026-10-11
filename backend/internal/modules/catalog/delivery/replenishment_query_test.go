package delivery

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"testing"
	"titansystem-backend/internal/onlinecatalog"
)

func TestReplenishmentQueryStrictDefaultsAndLiteralSearch(t *testing.T) {
	app := fiber.New()
	for _, kind := range []string{"summary", "page", "detail"} {
		kind := kind
		app.Get("/"+kind, func(c *fiber.Ctx) error {
			q, e := replenishmentQuery(c, kind)
			if e != nil {
				return c.SendStatus(400)
			}
			return c.JSON(q)
		})
	}
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/page?q=restock44-%25literal", 200}, {"/page?state=quantity_exceeds_limit&active=all&review_days=365", 200}, {"/summary?active=false", 200}, {"/detail?review_days=1", 200},
		{"/page?active=", 400}, {"/page?state=", 400}, {"/page?review_days=", 400}, {"/page?review_days=0", 400}, {"/page?review_days=366", 400}, {"/page?review_days=1.5", 400}, {"/page?review_days=-1", 400}, {"/page?review_days=1&review_days=2", 400}, {"/page?limit=", 400}, {"/page?limit=101", 400}, {"/page?tenant_id=x", 400}, {"/page?q=x&q=y", 400}, {"/summary?limit=1", 400}, {"/summary?state=all", 400}, {"/detail?active=all", 400}, {"/detail?q=x", 400},
	} {
		resp, e := app.Test(httptest.NewRequest("GET", tc.path, nil))
		if e != nil {
			t.Fatal(e)
		}
		if tc.path == "/page?q=restock44-%25literal" {
			var q onlinecatalog.ReplenishmentQuery
			e = json.NewDecoder(resp.Body).Decode(&q)
			if e != nil || q.Query != "restock44-%literal" || q.ReviewDays != 7 || q.State != "needs_replenishment" || q.Limit != 50 {
				t.Fatal(q, e)
			}
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatal(tc.path, resp.StatusCode)
		}
	}
}
