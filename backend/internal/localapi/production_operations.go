package localapi

import (
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionOperations(r fiber.Router) {
	r.Get("/production/operations/:kind/:id", s.productionOperation)
}
func (s *Server) productionOperation(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	id, err := url.PathUnescape(c.Params("id"))
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.GetOperation(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("kind"), id)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(out)
}
