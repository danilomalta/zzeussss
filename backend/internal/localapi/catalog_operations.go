package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) catalogEditingDetails(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	id, e := supplierPath(c)
	if e != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, e := catalog.EditingSnapshot(c.UserContext(), s.DB, v.Actor, v.Device, id)
	if e != nil {
		return productEditError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) catalogOperation(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	id, e := supplierPath(c)
	if e != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, e := catalog.CatalogOperation(c.UserContext(), s.DB, v.Actor, v.Device, c.Params("kind"), id)
	if e != nil {
		return productEditError(c, e)
	}
	return c.JSON(out)
}
