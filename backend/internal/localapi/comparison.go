package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/comparison"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountComparison(r fiber.Router) {
	r.Get("/comparison-sites", s.comparisonSites)
	r.Post("/comparison-sites", s.saveComparisonSite)
	r.Get("/comparison-site-operations/:id", s.comparisonSiteOperation)
}
func comparisonError(c *fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, comparison.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(e, comparison.ErrConflict), errors.Is(e, comparison.ErrLimit):
		return c.SendStatus(409)
	case errors.Is(e, comparison.ErrMissing):
		return c.SendStatus(404)
	default:
		return catalogError(c, e)
	}
}
func (s *Server) comparisonSites(c *fiber.Ctx) error {
	if len(c.Context().QueryArgs().String()) > 0 {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, e := comparison.List(c.UserContext(), s.DB, session.Actor, session.Device)
	if e != nil {
		return comparisonError(c, e)
	}
	return c.JSON(fiber.Map{"items": v, "limit": comparison.MaxSites, "automatic_state": "not_configured"})
}
func (s *Server) saveComparisonSite(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in comparison.Input
	if e := decodeCash(c.Body(), &in, []string{"operation_id", "id", "expected_revision", "name", "origin", "search_template", "status"}); e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, e := comparison.Save(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if e != nil {
		return comparisonError(c, e)
	}
	status := 201
	if v.Repeated || in.ExpectedRevision > 0 {
		status = 200
	}
	return c.Status(status).JSON(v)
}
func (s *Server) comparisonSiteOperation(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	v, e := comparison.Operation(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if e != nil {
		return comparisonError(c, e)
	}
	return c.JSON(v)
}
