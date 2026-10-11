package delivery

import (
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupplierOfferQueryStrictFilters(t *testing.T) {
	for _, tc := range []struct {
		query, body string
		ok          bool
	}{{"?product_id=1&active=false&limit=1&offset=2", "", true}, {"?supplier_id=", "", false}, {"?product_id=0", "", false}, {"?product_id=01", "", false}, {"?product_id=1&product_id=2", "", false}, {"?active=", "", false}, {"?q=x", "", false}, {"?tenant_id=x", "", false}, {"?limit=101", "", false}, {"", "{}", false}} {
		app := fiber.New()
		app.Get("/", func(c *fiber.Ctx) error {
			q, e := supplierOfferQuery(c)
			if (e == nil) != tc.ok {
				t.Errorf("query%s: %v", tc.query, e)
			}
			if tc.ok && (q.ProductID != 1 || q.Active != "false" || q.Offset != 2) {
				t.Errorf("parsed%v", q)
			}
			return c.SendStatus(200)
		})
		resp, e := app.Test(httptest.NewRequest("GET", "/"+tc.query, strings.NewReader(tc.body)))
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
	}
}
