package localapi

import (
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountSupplierEdit(r fiber.Router) {
	r.Get("/purchase-suppliers/:id/edit-state", s.supplierEditState)
	r.Post("/purchase-suppliers/:id/update", s.editSupplier)
}
func supplierPath(c *fiber.Ctx) (string, error) {
	if c.Context().QueryArgs().Len() != 0 {
		return "", purchases.ErrInvalid
	}
	id, err := url.PathUnescape(c.Params("id"))
	if err != nil {
		return "", purchases.ErrInvalid
	}
	return id, nil
}
func (s *Server) supplierEditState(c *fiber.Ctx) error {
	id, err := supplierPath(c)
	if err != nil {
		return purchaseError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.SupplierEditState(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
func (s *Server) editSupplier(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in purchases.SupplierEditInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "expected_revision", "name", "status", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	id, err := supplierPath(c)
	if err != nil {
		return purchaseError(c, err)
	}
	in.SupplierID = id
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.EditSupplier(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
