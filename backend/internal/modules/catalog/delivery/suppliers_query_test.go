package delivery

import (
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSupplierQueryRejectsUnknownDuplicateAndEmptyStates(t *testing.T) {
	for _, tc := range []struct {
		query, body string
		ok          bool
	}{{"?q=%25_literal&active=false&limit=1&offset=2", "", true}, {"?active=", "", false}, {"?active=TRUE", "", false}, {"?q=a&q=b", "", false}, {"?tenant_id=x", "", false}, {"?limit=101", "", false}, {"?offset=", "", false}, {"", "{}", false}} {
		app := fiber.New()
		app.Get("/", func(c *fiber.Ctx) error {
			q, e := supplierQuery(c)
			if (e == nil) != tc.ok {
				t.Errorf("query %s: %v", tc.query, e)
			}
			if tc.ok && (q.Query != "%_literal" || q.Active != "false" || q.Offset != 2) {
				t.Errorf("parsed %v", q)
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
