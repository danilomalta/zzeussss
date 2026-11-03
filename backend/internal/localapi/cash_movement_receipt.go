package localapi

import (
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/payables"
)

func (s *Server) mountCashMovementReceipt(r fiber.Router) {
	r.Get("/cash/movements/operations/:id", s.cashMovementReceipt)
}
func (s *Server) cashMovementReceipt(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	result, e := payables.AuditedMovementReceipt(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(result)
}
