package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/companyregistration"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func companyFailure(c *fiber.Ctx, err error) error {
	code := 500
	switch {
	case errors.Is(err, companyregistration.ErrInvalid):
		code = 400
	case errors.Is(err, companyregistration.ErrConflict):
		code = 409
	case errors.Is(err, companyregistration.ErrMissing):
		code = 404
	case errors.Is(err, identity.ErrDenied):
		code = 403
	case errors.Is(err, localauth.ErrDenied):
		code = 401
	}
	return c.Status(code).JSON(fiber.Map{"error": "company_registration_unavailable"})
}
func (s *Server) companyRegistration(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	ss := c.Locals("session").(localauth.Session)
	if c.Method() == "GET" {
		if len(c.Body()) != 0 {
			return companyFailure(c, companyregistration.ErrInvalid)
		}
		operation := ""
		valid := true
		seen := false
		c.Context().QueryArgs().VisitAll(func(k, v []byte) {
			if string(k) != "operation_id" || seen || len(v) == 0 {
				valid = false
			}
			seen = true
			operation = string(v)
		})
		if !valid {
			return companyFailure(c, companyregistration.ErrInvalid)
		}
		out, err := companyregistration.Read(c.UserContext(), s.DB, ss.Actor, ss.Device, ss.Token, operation)
		if err != nil {
			return companyFailure(c, err)
		}
		return c.JSON(out)
	}
	var in companyregistration.Input
	if !noBagQuery(c) || len(c.Body()) > 8192 || decodeCash(c.Body(), &in, []string{"operation_id", "expected_revision", "stage", "company_name", "responsible", "cnpj", "legal_name", "uf", "state_registration", "municipality_ibge", "crt", "provider", "environment"}) != nil {
		return companyFailure(c, companyregistration.ErrInvalid)
	}
	out, err := companyregistration.Save(c.UserContext(), s.DB, ss.Actor, ss.Device, ss.Token, in)
	if err != nil {
		return companyFailure(c, err)
	}
	return c.JSON(out)
}
