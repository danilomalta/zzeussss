package delivery

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"testing"
	"titansystem-backend/internal/onlinecatalog"
)

func TestProposalQueryStrictDefaultsAndLiteralSearch(t *testing.T) {
	app := fiber.New()
	for _, kind := range []string{"summary", "page"} {
		kind := kind
		app.Get("/"+kind, func(c *fiber.Ctx) error {
			q, e := proposalQuery(c, kind == "summary")
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
		{"/page?q=proposal46-%25literal", 200}, {"/page?state=approved&product_id=1", 200}, {"/summary?product_id=1", 200},
		{"/page?state=", 400}, {"/page?state=pending", 400}, {"/page?product_id=0", 400}, {"/page?product_id=01", 400}, {"/page?product_id=", 400}, {"/page?limit=", 400}, {"/page?limit=101", 400}, {"/page?offset=-1", 400}, {"/page?state=all&state=planned", 400}, {"/page?q=x&q=y", 400}, {"/summary?state=all", 400}, {"/summary?limit=1", 400}, {"/page?tenant_id=x", 400},
	} {
		resp, e := app.Test(httptest.NewRequest("GET", tc.path, nil))
		if e != nil {
			t.Fatal(e)
		}
		if tc.path == "/page?q=proposal46-%25literal" {
			var q onlinecatalog.ProposalQuery
			e = json.NewDecoder(resp.Body).Decode(&q)
			if e != nil || q.Query != "proposal46-%literal" || q.State != "all" || q.Limit != 50 {
				t.Fatal(q, e)
			}
		}
		resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Fatal(tc.path, resp.StatusCode)
		}
	}
}
