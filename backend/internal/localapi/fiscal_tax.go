package localapi

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
)

func taxWrite(c *fiber.Ctx, target any) bool {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return noBagQuery(c) && len(c.Body()) <= 65536 && fiscalcore.DecodeInput(c.Body(), target) == nil
}
func (s *Server) publishTaxPolicy(c *fiber.Ctx) error {
	var in fiscal.TaxPolicyInput
	if !taxWrite(c, &in) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	// Missing/null approval state must not silently become an inactive revision.
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(c.Body(), &fields)
	if string(fields["active"]) != "true" && string(fields["active"]) != "false" {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	var presence struct {
		Rule struct {
			Components []struct {
				AddsToTotal *bool `json:"adds_to_total"`
			} `json:"components"`
		} `json:"rule"`
	}
	_ = json.Unmarshal(c.Body(), &presence)
	for _, component := range presence.Rule.Components {
		if component.AddsToTotal == nil {
			return fiscalFailure(c, fiscal.ErrInvalid)
		}
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.PublishTaxPolicy(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in, ss.Token)
	if err != nil {
		return fiscalFailure(c, err)
	}
	if out.Repeated {
		return c.JSON(out)
	}
	return c.Status(201).JSON(out)
}
func (s *Server) readTaxPolicy(c *fiber.Ctx) error {
	if !fiscalRead(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	revision := 0
	valid := true
	seen := false
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) != "revision" || seen {
			valid = false
			return
		}
		seen = true
		n, err := strconv.Atoi(string(v))
		if err != nil || n < 1 || strconv.Itoa(n) != string(v) {
			valid = false
			return
		}
		revision = n
	})
	if !valid {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.ReadTaxPolicy(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"), revision)
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(out)
}
func (s *Server) createTaxQuote(c *fiber.Ctx) error {
	var in fiscal.TaxQuoteInput
	if !taxWrite(c, &in) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.CreateTaxQuote(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in, ss.Token)
	if err != nil {
		return fiscalFailure(c, err)
	}
	if out.Repeated {
		return c.JSON(out)
	}
	return c.Status(201).JSON(out)
}
func (s *Server) readTaxQuote(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.ReadTaxQuote(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(out)
}
