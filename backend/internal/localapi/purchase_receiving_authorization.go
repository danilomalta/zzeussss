package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseReceivingAuthorization(r fiber.Router) {
	r.Post("/purchase-orders/:id/authorize-receiving", s.authorizePurchaseReceiving)
}
func (s *Server) authorizePurchaseReceiving(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in purchases.ReceivingAuthorizationInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "reference", "reason"}) != nil {
		return c.SendStatus(400)
	}
	id, err := supplierPath(c)
	if err != nil {
		return purchaseError(c, err)
	}
	in.OrderID = id
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.AuthorizeReceiving(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
