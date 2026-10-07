package localapi

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/stockreservation"
)

func (s *Server) mountProductionMaterials(r fiber.Router) {
	r.Post("/production/material-reservations", s.reserveProductionMaterials)
	r.Post("/production/material-reservations/state", s.changeProductionMaterials)
	r.Get("/production/material-reservations/:id", s.productionMaterials)
}

func materialsError(c *fiber.Ctx, err error) error {
	if errors.Is(err, stockreservation.ErrUnavailable) || errors.Is(err, production.ErrCapacity) {
		return c.SendStatus(409)
	}
	return productionError(c, err)
}

func (s *Server) reserveProductionMaterials(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.ReserveInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "reservation_id", "order_id", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.ReserveMaterials(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return materialsError(c, err)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}

func (s *Server) changeProductionMaterials(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.MaterialChangeInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "reservation_id", "action", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.ChangeMaterials(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return materialsError(c, err)
	}
	return c.JSON(v)
}

func (s *Server) productionMaterials(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.GetMaterials(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return materialsError(c, err)
	}
	return c.JSON(v)
}
