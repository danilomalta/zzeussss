package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
)

// Profiles are configured through the existing owner/contract/revision gate.
// Diagnostics are authenticated, offline, and cannot enable transmission.
func (s *Server) fiscalOnboarding(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	p, err := fiscal.ReadProfile(c.UserContext(), s.DB, a.Actor, a.Device, c.Params("id"), 0)
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(fiscalproduction.DiagnoseOnboarding(s.fiscalProviders, s.production, a, p))
}
