package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountSupplierOperation(r fiber.Router) {
	r.Get("/purchase-supplier-operations/:id/receipt", func(c *fiber.Ctx) error {
		id, err := supplierPath(c)
		if err != nil {
			return purchaseError(c, err)
		}
		session := c.Locals("session").(localauth.Session)
		out, err := purchases.SupplierEditOperation(c.UserContext(), s.DB, session.Actor, session.Device, id)
		if err != nil {
			return purchaseError(c, err)
		}
		return c.JSON(out)
	})
}
