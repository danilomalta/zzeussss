package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionResults(r fiber.Router) {
	r.Post("/production/results", s.completeProduction)
	r.Get("/production/results/:id", s.productionResult)
}

func (s *Server) completeProduction(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.ResultInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "result_id", "order_id", "expected_revision", "produced_milli", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.CompleteProduction(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return materialsError(c, err)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}

func (s *Server) productionResult(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.GetProductionResult(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(v)
}
