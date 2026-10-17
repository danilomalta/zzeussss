package localapi

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionLots(r fiber.Router) {
	r.Post("/production/lots", s.recordProductionLot)
	r.Post("/production/lots/void", s.voidProductionLot)
	r.Get("/production/lots/:id/history", s.productionLotHistory)
	r.Get("/production/lots/:id", s.productionLot)
	r.Get("/production/results/:id/lots", s.productionResultLots)
}

func lotError(c *fiber.Ctx, err error) error {
	if errors.Is(err, production.ErrLot) {
		return c.SendStatus(409)
	}
	return productionError(c, err)
}

func (s *Server) recordProductionLot(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.LotInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "lot_id", "result_id", "quantity_milli", "code", "manufactured_on", "expires_on", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.RecordProductionLot(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return lotError(c, err)
	}
	status := 201
	if out.Repeated {
		status = 200
	}
	return c.Status(status).JSON(out)
}

func (s *Server) voidProductionLot(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.VoidLotInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "lot_id", "expected_revision", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.VoidProductionLot(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return lotError(c, err)
	}
	return c.JSON(out)
}

func (s *Server) productionLot(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.GetProductionLot(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return lotError(c, err)
	}
	return c.JSON(out)
}

func (s *Server) productionLotHistory(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.ProductionLotHistory(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return lotError(c, err)
	}
	return c.JSON(fiber.Map{"items": out})
}

func (s *Server) productionResultLots(c *fiber.Ctx) error {
	offset, err := orderOffset(c)
	if err != nil {
		return lotError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.ListProductionLots(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), offset)
	if err != nil {
		return lotError(c, err)
	}
	return c.JSON(out)
}
