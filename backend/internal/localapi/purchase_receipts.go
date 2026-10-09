package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseReceipts(r fiber.Router) {
	r.Post("/purchase-orders/:id/receipts", s.recordPurchaseReceipt)
}
func (s *Server) recordPurchaseReceipt(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in purchases.ReceiptInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "delivery_reference", "location_id", "unit", "delivered_milli", "accepted_milli", "reason"}) != nil {
		return c.SendStatus(400)
	}
	id, err := supplierPath(c)
	if err != nil {
		return purchaseError(c, err)
	}
	in.OrderID = id
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.RecordReceiving(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return purchaseError(c, err)
	}
	status := 201
	if out.Repeated {
		status = 200
	}
	return c.Status(status).JSON(out)
}
