package localapi

import (
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseReceiptVoids(r fiber.Router) {
	r.Post("/purchase-orders/:id/receipts/:receipt_id/void", s.voidPurchaseReceipt)
}
func (s *Server) voidPurchaseReceipt(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in purchases.ReceiptVoidInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "reason"}) != nil {
		return c.SendStatus(400)
	}
	id, err := supplierPath(c)
	if err != nil {
		return purchaseError(c, err)
	}
	in.OrderID = id
	in.ReceiptID, err = url.PathUnescape(c.Params("receipt_id"))
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.VoidReceipt(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
