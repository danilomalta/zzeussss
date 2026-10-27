package localapi

import (
	"github.com/gofiber/fiber/v2"
	"net/url"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountSupplierHistory(r fiber.Router) {
	r.Get("/purchase-suppliers/:id/history", s.supplierHistory)
}
func (s *Server) supplierHistory(c *fiber.Ctx) error {
	bad := false
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) != "offset" {
			bad = true
		}
	})
	if bad || len(c.Context().QueryArgs().PeekMulti("offset")) > 1 {
		return c.SendStatus(400)
	}
	raw := "0"
	if c.Context().QueryArgs().Has("offset") {
		raw = string(c.Context().QueryArgs().Peek("offset"))
	}
	if raw == "" || len(raw) > 16 {
		return c.SendStatus(400)
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return c.SendStatus(400)
		}
	}
	offset, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return c.SendStatus(400)
	}
	id, err := url.PathUnescape(c.Params("id"))
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.SupplierHistory(c.UserContext(), s.DB, session.Actor, session.Device, id, offset)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
