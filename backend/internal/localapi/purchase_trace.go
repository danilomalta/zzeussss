package localapi

import (
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseTrace(r fiber.Router) {
	r.Get("/purchase-orders/:id/trace", s.purchaseTrace)
}
func (s *Server) purchaseTrace(c *fiber.Ctx) error {
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	id, err := url.PathUnescape(c.Params("id"))
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.Trace(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
