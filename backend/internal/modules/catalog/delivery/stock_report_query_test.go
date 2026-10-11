package delivery

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"testing"
	"titansystem-backend/internal/onlinecatalog"
)

func TestStockQueryLiteralDefaultsAndStrictInput(t *testing.T) {
	app := fiber.New()
	app.Get("/summary", func(c *fiber.Ctx) error {
		q, e := stockReportQuery(c, "all", true, false)
		if e != nil {
			return c.SendStatus(400)
		}
		return c.JSON(q)
	})
	app.Get("/zero", func(c *fiber.Ctx) error {
		q, e := stockReportQuery(c, "zero_stock", false, false)
		if e != nil {
			return c.SendStatus(400)
		}
		return c.JSON(q)
	})
	app.Get("/csv", func(c *fiber.Ctx) error {
		q, e := stockReportQuery(c, "all", false, true)
		if e != nil {
			return c.SendStatus(400)
		}
		return c.JSON(q)
	})
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/zero?q=Stock42-%25literal", 200}, {"/csv?issue=no_positive_demand&horizon_days=365", 200}, {"/summary?active=all", 200},
		{"/zero?issue=low_coverage", 400}, {"/zero?active=", 400}, {"/zero?horizon_days=", 400}, {"/zero?horizon_days=366", 400},
		{"/zero?horizon_days=1.5", 400}, {"/zero?horizon_days=-1", 400}, {"/zero?q=x&q=y", 400}, {"/zero?limit=101", 400},
		{"/csv?issue=", 400}, {"/csv?tenant_id=x", 400}, {"/summary?limit=1", 400}, {"/summary?offset=0", 400},
	} {
		resp, e := app.Test(httptest.NewRequest("GET", tc.path, nil))
		if e != nil {
			t.Fatal(e)
		}
		if tc.path == "/zero?q=Stock42-%25literal" {
			var q onlinecatalog.StockReportQuery
			e = json.NewDecoder(resp.Body).Decode(&q)
			if e != nil || q.Query != "Stock42-%literal" || q.Active != "true" || q.HorizonDays != 7 {
				t.Fatal(q, e)
			}
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatal(tc.path, resp.StatusCode)
		}
	}
}
