package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseRejections(r fiber.Router) {
	r.Post("/purchase-orders/:id/rejections", s.rejectPurchaseDelivery)
}
func (s *Server) rejectPurchaseDelivery(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in purchases.RejectionInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "delivery_reference", "unit", "delivered_milli", "reason"}) != nil {
		return c.SendStatus(400)
	}
	id, err := supplierPath(c)
	if err != nil {
		return purchaseError(c, err)
	}
	in.OrderID = id
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.RejectDelivery(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return purchaseError(c, err)
	}
	status := 201
	if out.Repeated {
		status = 200
	}
	return c.Status(status).JSON(out)
}
