package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseReceivingOperations(r fiber.Router) {
	r.Get("/purchase-receiving/operations/:kind/:id/receipt", s.purchaseReceivingOperation)
}
func (s *Server) purchaseReceivingOperation(c *fiber.Ctx) error {
	id, err := supplierPath(c)
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := purchases.ReceivingOperation(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("kind"), id)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(v)
}
