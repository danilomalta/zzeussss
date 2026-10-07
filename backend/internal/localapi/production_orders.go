package localapi

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionOrders(r fiber.Router) {
	r.Post("/production/orders", s.createProductionOrder)
	r.Post("/production/orders/state", s.changeProductionOrder)
	r.Get("/production/orders", s.productionOrders)
	r.Get("/production/orders/:id/history", s.productionOrderHistory)
	r.Get("/production/orders/:id", s.productionOrder)
}

func orderOffset(c *fiber.Ctx) (int, error) {
	seen := false
	valid := true
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) != "offset" || seen {
			valid = false
		}
		seen = true
	})
	n, err := strconv.Atoi(c.Query("offset", "0"))
	if !valid || err != nil || n < 0 {
		return 0, production.ErrInvalid
	}
	return n, nil
}

func (s *Server) createProductionOrder(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.OrderInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "order_id", "version_id", "location_id", "responsible_id", "planned_batches"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.CreateOrder(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return productionError(c, err)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}

func (s *Server) changeProductionOrder(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.OrderStateInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "order_id", "expected_revision", "status", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.ChangeOrderState(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(v)
}

func (s *Server) productionOrder(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.GetOrder(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(v)
}

func (s *Server) productionOrders(c *fiber.Ctx) error {
	n, err := orderOffset(c)
	if err != nil {
		return productionError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.ListOrders(c.UserContext(), s.DB, session.Actor, session.Device, n)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(fiber.Map{"items": v})
}

func (s *Server) productionOrderHistory(c *fiber.Ctx) error {
	n, err := orderOffset(c)
	if err != nil {
		return productionError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.OrderHistory(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), n)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(fiber.Map{"items": v})
}
