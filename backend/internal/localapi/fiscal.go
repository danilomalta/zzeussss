package localapi

import (
	"encoding/json"
	"errors"
	"strconv"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) mountFiscal(r fiber.Router) {
	r.Post("/fiscal/nfe55/sales/:id/preview", s.fiscalNFe55Preview)
	r.Post("/fiscal/nfe55/sales/:id/emit", s.fiscalNFe55Emit)
	r.Post("/fiscal/nfe55/sales/:id/query", s.fiscalNFe55Query)
	r.Post("/fiscal/nfe55/sales/:id/cancel", s.fiscalNFe55Cancel)
	r.Get("/fiscal/nfe55/sales/:id/danfe", s.fiscalNFe55DANFE)

	r.Post("/fiscal/tax-policies", s.publishTaxPolicy)
	r.Get("/fiscal/tax-policies/:id", s.readTaxPolicy)
	r.Post("/fiscal/tax-quotes", s.createTaxQuote)
	r.Get("/fiscal/tax-quotes/:id", s.readTaxQuote)
	r.Get("/fiscal/capabilities", s.fiscalCapabilities)
	r.Get("/fiscal/requirements/:uf", s.fiscalRequirements)
	r.Post("/fiscal/onboarding", s.configureFiscal)
	r.Get("/fiscal/onboarding/:id/provider", s.fiscalCustomerCredentials)
	r.Post("/fiscal/onboarding/:id/provider", s.fiscalCustomerCredentials)
	r.Get("/fiscal/onboarding/:id", s.fiscalOnboarding)
	r.Post("/fiscal/profiles", s.configureFiscal)
	r.Get("/fiscal/profiles/:id", s.readFiscalProfile)
	r.Post("/fiscal/intents", s.prepareFiscal)
	r.Get("/fiscal/intents", s.listFiscalIntents)
	r.Get("/fiscal/intents/:id", s.readFiscalIntent)
	r.Get("/fiscal/intents/:id/danfe", s.readFiscalDANFE)
	r.Post("/fiscal/intents/:id/production/prepare", s.prepareProduction)
	r.Post("/fiscal/intents/:id/production/approve", s.approveProduction)
	r.Post("/fiscal/intents/:id/production/emit", s.emitProduction)
	r.Post("/fiscal/intents/:id/production/query", s.queryProduction)
	r.Post("/fiscal/intents/:id/production/cancel", s.cancelProduction)
	r.Get("/fiscal/intents/:id/production/cancellation", s.readProductionCancellation)
	r.Get("/fiscal/intents/:id/production/cancellation/xml", s.readProductionCancellationXML)
	r.Get("/fiscal/intents/:id/production", s.readProduction)
	r.Get("/fiscal/intents/:id/production/xml", s.readProductionXML)
	r.Get("/fiscal/operations/:id/receipt", s.readFiscalOperation)
}

func fiscalFailure(c *fiber.Ctx, err error) error {
	code, kind := 500, "fiscal_storage_error"
	switch {
	case errors.Is(err, fiscal.ErrDocumentUnavailable):
		code, kind = 409, "fiscal_document_not_printable"
	case errors.Is(err, fiscal.ErrInvalid):
		code, kind = 400, "invalid_fiscal_request"
	case errors.Is(err, fiscal.ErrConflict):
		code, kind = 409, "fiscal_conflict"
	case errors.Is(err, fiscal.ErrNotFound):
		code, kind = 404, "fiscal_not_found"
	case errors.Is(err, localauth.ErrDenied):
		code, kind = 401, "fiscal_session_invalid"
	case errors.Is(err, identity.ErrDenied), errors.Is(err, modules.ErrUnavailable):
		code, kind = 403, "fiscal_not_permitted"
	case errors.Is(err, entitlementstore.ErrNotInstalled), errors.Is(err, entitlementstore.ErrClockRollback), errors.Is(err, entitlementstore.ErrCorrupt):
		code, kind = 503, "fiscal_contract_unavailable"
	case errors.Is(err, fiscal.ErrIntegrity):
		code, kind = 409, "fiscal_integrity_failure"
	}
	return c.Status(code).JSON(fiber.Map{"error": kind})
}
func fiscalRead(c *fiber.Ctx) bool {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return len(c.Body()) == 0
}
func fiscalWrite(c *fiber.Ctx) bool {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return noBagQuery(c) && len(c.Body()) <= 8192
}
func (s *Server) fiscalCapabilities(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	err := identity.CanOperate(c.UserContext(), s.DB, ss.Actor, ss.Device, identity.ViewAccounting)
	if errors.Is(err, identity.ErrDenied) {
		err = identity.CanOperate(c.UserContext(), s.DB, ss.Actor, ss.Device, identity.Sell)
	}
	if err != nil {
		return fiscalFailure(c, err)
	}
	out := fiber.Map{"scope": "durable_preparation_and_homologation_archive_print", "homologation_archive_print_available": true, "models": []string{"55", "65"}, "profile_configuration": "active_owner_and_fiscal_contract", "readiness": fiscal.Unavailable(), "nfse_available": false, "sped_generation_available": false, "transmission_available": false, "automatic_retry_available": false, "production_cancellation_available": false}
	if s.production != nil && s.production.Validate() == nil {
		b := s.production.Binding()
		if b.TenantID == ss.Actor.TenantID && b.StoreID == ss.Actor.StoreID && b.DeviceID == ss.Device.DeviceID {
			out["scope"] = "synchronous_production_emitter"
			out["transmission_available"] = true
			out["readiness"] = fiscal.Readiness{Blockers: []string{"reviewed_production_document_and_approval_required"}}
			out["production_cancellation_available"] = b.CancellationURL != ""
			out["production_model"] = b.Model
			out["production_configuration_sha256"] = b.SHA256()
		}
	}
	return c.JSON(out)
}
func (s *Server) configureFiscal(c *fiber.Ctx) error {
	var in fiscal.ProfileInput
	if !fiscalWrite(c) || decodeFiscalProfile(c.Body(), &in) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.Configure(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in, ss.Token)
	if err != nil {
		return fiscalFailure(c, err)
	}
	if out.Repeated {
		return c.JSON(out)
	}
	return c.Status(201).JSON(out)
}
func (s *Server) readFiscalProfile(c *fiber.Ctx) error {
	if !fiscalRead(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	revision, valid := 0, true
	seen := false
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) != "revision" || seen || len(v) == 0 {
			valid = false
			return
		}
		seen = true
		for _, b := range v {
			if b < '0' || b > '9' {
				valid = false
				return
			}
		}
		var err error
		revision, err = strconv.Atoi(string(v))
		if err != nil || revision < 1 || revision > 1000000 {
			valid = false
		}
	})
	if !valid {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.ReadProfile(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"), revision)
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(out)
}
func (s *Server) prepareFiscal(c *fiber.Ctx) error {
	var in fiscal.IntentInput
	if !fiscalWrite(c) || decodeCash(c.Body(), &in, []string{"operation_id", "sale_id", "profile_id", "profile_revision"}) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.Prepare(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in, ss.Token)
	if err != nil {
		return fiscalFailure(c, err)
	}
	if out.Repeated {
		return c.JSON(out)
	}
	return c.Status(201).JSON(out)
}
func (s *Server) readFiscalIntent(c *fiber.Ctx) error    { return s.fiscalReceipt(c, false) }
func (s *Server) readFiscalOperation(c *fiber.Ctx) error { return s.fiscalReceipt(c, true) }
func (s *Server) fiscalReceipt(c *fiber.Ctx, operation bool) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.ReadIntent(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"), operation)
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(out)
}
func (s *Server) listFiscalIntents(c *fiber.Ctx) error {
	if !fiscalRead(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	limit, offset := 50, 0
	sale := ""
	valid := true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || len(v) == 0 {
			valid = false
			return
		}
		seen[key] = true
		if key == "sale_id" {
			sale = string(v)
			return
		}
		if key != "limit" && key != "offset" {
			valid = false
			return
		}
		for _, b := range v {
			if b < '0' || b > '9' {
				valid = false
				return
			}
		}
		n, err := strconv.Atoi(string(v))
		if err != nil {
			valid = false
			return
		}
		if key == "limit" {
			limit = n
		} else {
			offset = n
		}
	})
	if !valid {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.List(c.UserContext(), s.DB, ss.Actor, ss.Device, sale, limit, offset)
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(out)
}

func (s *Server) fiscalRequirements(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	err := identity.CanOperate(c.UserContext(), s.DB, ss.Actor, ss.Device, identity.ViewAccounting)
	if errors.Is(err, identity.ErrDenied) {
		err = identity.CanOperate(c.UserContext(), s.DB, ss.Actor, ss.Device, identity.Sell)
	}
	if err != nil {
		return fiscalFailure(c, err)
	}
	out, err := fiscal.Requirements(c.Params("uf"))
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(out)
}

func decodeFiscalProfile(body []byte, in *fiscal.ProfileInput) error {
	if _, err := exactSaleObject(body, []string{"operation_id", "profile_id", "expected_revision", "cnpj", "legal_name", "state", "municipality_ibge", "state_registration", "crt", "model", "environment", "series"}, "timezone", "require_tax_pricing"); err != nil {
		return err
	}
	return json.Unmarshal(body, in)
}
