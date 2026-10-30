package localapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"

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
	if len(c.Body()) > 128*1024 {
		return c.SendStatus(413)
	}
	values, err := exactSaleObject(c.Body(), []string{"operation_id", "received_cents"}, "fiscal_invoice")
	if err != nil {
		return c.SendStatus(400)
	}
	var in sale.FinalizeCashPlanInput
	if err := json.Unmarshal(c.Body(), &in); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	var invoice *fiscalcore.Invoice
	if raw, ok := values["fiscal_invoice"]; ok {
		invoice = &fiscalcore.Invoice{}
		if fiscalcore.DecodeInput(raw, invoice) != nil || invoice.Key != (fiscalcore.KeyInput{}) {
			return c.SendStatus(400)
		}
		if invoice.Environment != 2 {
			return c.Status(400).JSON(fiber.Map{"error": "fiscal_checkout_production_disabled"})
		}
		if s.production == nil {
			return c.SendStatus(503)
		}
		plan, e := sale.ReadPlan(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
		if e != nil {
			return saleError(c, e)
		}
		for _, item := range plan.Items {
			if item.Tax == nil || plan.Completion == nil && (item.Tax.ProfileID != s.production.Binding().ProfileID || item.Tax.ProfileRevision != s.production.Binding().ProfileRevision) {
				return c.SendStatus(400)
			}
		}
		if plan.Completion == nil {
			e = s.production.ValidateCheckoutHomologation(c.UserContext(), fiscalproduction.RequestContext{DB: s.DB, License: s.contracts, Actor: session.Actor, Device: session.Device, Token: session.Token}, invoice.Environment)
			if errors.Is(e, fiscalproduction.ErrCheckoutProduction) {
				return c.Status(400).JSON(fiber.Map{"error": "fiscal_checkout_production_disabled"})
			}
			if e != nil {
				return productionFailure(c, e)
			}
		}
		canonical, _ := json.Marshal(invoice)
		sum := sha256.Sum256(canonical)
		in.FiscalRequestSHA256 = hex.EncodeToString(sum[:])
	}
	v, err := sale.FinalizeCashPlan(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, c.Params("id"), in, session.Token)
	if err != nil {
		return saleError(c, err)
	}
	if invoice != nil {
		state, e := s.production.EmitCheckoutHomologation(c.UserContext(), fiscalproduction.RequestContext{DB: s.DB, License: s.contracts, Actor: session.Actor, Device: session.Device, Token: session.Token}, v.Completion.SaleID, *invoice)
		v.Fiscal = &state
		if state.ErrorCode == "fiscal_status_persistence_failed" {
			return c.Status(503).JSON(v)
		}
		if e != nil && state.State == "" {
			return productionFailure(c, e)
		}
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
	values, err := exactSaleObject(c.Body(), []string{"operation_id", "checkout_id", "cash_session_id", "expected_total_cents", "items", "parts"}, "tax_quote_operations")
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
	v, err := sale.CreatePlanWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in, session.Token)
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
