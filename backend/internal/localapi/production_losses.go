package localapi

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionLosses(r fiber.Router) {
	r.Post("/production/losses", s.recordProductionLoss)
	r.Post("/production/losses/void", s.voidProductionLoss)
	r.Get("/production/losses/:id/history", s.productionLossHistory)
	r.Get("/production/losses/:id", s.productionLoss)
	r.Get("/production/results/:id/losses", s.productionResultLosses)
}

func lossError(c *fiber.Ctx, err error) error {
	if errors.Is(err, production.ErrLoss) {
		return c.SendStatus(409)
	}
	return productionError(c, err)
}

func (s *Server) recordProductionLoss(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.LossInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "loss_id", "result_id", "quantity_milli", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.RecordProductionLoss(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return lossError(c, err)
	}
	status := 201
	if out.Repeated {
		status = 200
	}
	return c.Status(status).JSON(out)
}

func (s *Server) voidProductionLoss(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.VoidLossInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "loss_id", "expected_revision", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.VoidProductionLoss(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return lossError(c, err)
	}
	return c.JSON(out)
}

func (s *Server) productionLoss(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.GetProductionLoss(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return lossError(c, err)
	}
	return c.JSON(out)
}

func (s *Server) productionLossHistory(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.ProductionLossHistory(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return lossError(c, err)
	}
	return c.JSON(fiber.Map{"items": out})
}

func (s *Server) productionResultLosses(c *fiber.Ctx) error {
	offset, err := orderOffset(c)
	if err != nil {
		return lossError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.ListProductionLosses(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), offset)
	if err != nil {
		return lossError(c, err)
	}
	return c.JSON(out)
}
