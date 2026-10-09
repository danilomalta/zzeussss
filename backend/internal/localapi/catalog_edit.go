package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountCatalogEdit(r fiber.Router) {
	r.Get("/catalog/products/:id/edit-state", s.productEditState)
	r.Post("/catalog/products/:id/update", s.editProduct)
}
func productEditError(c *fiber.Ctx, err error) error {
	if errors.Is(err, catalog.ErrEditConflict) {
		return c.SendStatus(409)
	}
	if errors.Is(err, catalog.ErrEditNotFound) {
		return c.SendStatus(404)
	}
	return catalogError(c, err)
}
func (s *Server) productEditState(c *fiber.Ctx) error {
	id, err := url.PathUnescape(c.Params("id"))
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := catalog.EditState(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if err != nil {
		return productEditError(c, err)
	}
	return c.JSON(out)
}
func (s *Server) editProduct(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in catalog.EditInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "expected_revision", "sku", "name", "unit", "barcode", "price_cents", "cost_cents"}); err != nil {
		return c.SendStatus(400)
	}
	id, err := url.PathUnescape(c.Params("id"))
	if err != nil {
		return c.SendStatus(400)
	}
	in.ProductID = id
	session := c.Locals("session").(localauth.Session)
	out, err := catalog.Edit(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return productEditError(c, err)
	}
	return c.JSON(out)
}
