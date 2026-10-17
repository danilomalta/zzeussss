package localapi

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func (s *Server) mountCheckoutPlans(r fiber.Router) {
	r.Post("/checkout/plans", s.createCheckoutPlan)
	r.Get("/checkout/plans/:id", s.readCheckoutPlan)
	r.Post("/checkout/plans/:id/cancel", s.cancelCheckoutPlan)
	r.Post("/checkout/plans/:id/finalize-cash", s.finalizeCashCheckoutPlan)
}

func (s *Server) finalizeCashCheckoutPlan(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	if _, err := exactSaleObject(c.Body(), []string{"operation_id", "received_cents"}); err != nil {
		return c.SendStatus(400)
	}
	var in sale.FinalizeCashPlanInput
	if err := json.Unmarshal(c.Body(), &in); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := sale.FinalizeCashPlan(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, c.Params("id"), in)
	if err != nil {
		return saleError(c, err)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}

func (s *Server) createCheckoutPlan(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 128*1024 {
		return c.SendStatus(413)
	}
	values, err := exactSaleObject(c.Body(), []string{"operation_id", "checkout_id", "cash_session_id", "expected_total_cents", "items", "parts"})
	if err != nil {
		return c.SendStatus(400)
	}
	for _, check := range []struct {
		name   string
		fields []string
		max    int
	}{
		{"items", []string{"product_id", "location_id", "quantity_milli"}, 500},
		{"parts", []string{"part_id", "method", "amount_cents"}, 20},
	} {
		var entries []json.RawMessage
		if err = json.Unmarshal(values[check.name], &entries); err != nil || len(entries) == 0 || len(entries) > check.max {
			return c.SendStatus(400)
		}
		for _, entry := range entries {
			if _, err = exactSaleObject(entry, check.fields); err != nil {
				return c.SendStatus(400)
			}
		}
	}
	var in sale.PlanInput
	if err = json.Unmarshal(c.Body(), &in); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := sale.CreatePlanWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return saleError(c, err)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}

func (s *Server) readCheckoutPlan(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := sale.ReadPlan(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return saleError(c, err)
	}
	return c.JSON(v)
}

func (s *Server) cancelCheckoutPlan(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	if _, err := exactSaleObject(c.Body(), []string{"operation_id", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	var in sale.CancelPlanInput
	if err := json.Unmarshal(c.Body(), &in); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := sale.CancelPlan(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), in)
	if err != nil {
		return saleError(c, err)
	}
	return c.JSON(v)
}
