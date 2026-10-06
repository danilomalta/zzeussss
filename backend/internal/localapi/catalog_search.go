package localapi

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) searchCatalog(c *fiber.Ctx) error {
	valid := true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		name := string(k)
		if seen[name] || name != "q" && name != "unit" && name != "pending" && name != "offset" {
			valid = false
		}
		seen[name] = true
	})
	if !valid {
		return c.SendStatus(400)
	}
	offset, e := strconv.Atoi(c.Query("offset", "0"))
	if e != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, e := catalog.Search(c.UserContext(), s.DB, v.Actor, v.Device, catalog.SearchInput{Query: c.Query("q"), Unit: c.Query("unit"), Pending: c.Query("pending"), Offset: offset})
	if e != nil {
		return catalogError(c, e)
	}
	return c.JSON(out)
}
