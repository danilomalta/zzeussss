package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseCancel(r fiber.Router) {
	r.Post("/purchase-orders/:id/cancel", s.cancelPurchase)
}
func (s *Server) cancelPurchase(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in purchases.CancelInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "expected_status", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	id, err := supplierPath(c)
	if err != nil {
		return purchaseError(c, err)
	}
	in.OrderID = id
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.Cancel(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
