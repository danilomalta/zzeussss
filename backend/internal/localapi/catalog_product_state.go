package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountProductState(r fiber.Router) {
	r.Get("/catalog/products/:id/state", s.productState)
	r.Post("/catalog/products/:id/state", s.changeProductState)
}
func productStateError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, catalog.ErrEditConflict), errors.Is(err, catalog.ErrProductInactive):
		return c.SendStatus(409)
	case errors.Is(err, catalog.ErrEditNotFound):
		return c.SendStatus(404)
	default:
		return catalogError(c, err)
	}
}
func (s *Server) productState(c *fiber.Ctx) error {
	id, err := supplierPath(c)
	if err != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, err := catalog.GetProductState(c.UserContext(), s.DB, v.Actor, v.Device, id)
	if err != nil {
		return productStateError(c, err)
	}
	return c.JSON(out)
}
func (s *Server) changeProductState(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in catalog.ProductStateInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "expected_revision", "status", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	id, err := supplierPath(c)
	if err != nil {
		return c.SendStatus(400)
	}
	in.ProductID = id
	v := c.Locals("session").(localauth.Session)
	out, err := catalog.ChangeProductState(c.UserContext(), s.DB, s.contracts, v.Actor, v.Device, in)
	if err != nil {
		return productStateError(c, err)
	}
	return c.JSON(out)
}
