package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionQuality(r fiber.Router) {
	r.Post("/production/quality-reviews", s.recordProductionQuality)
	r.Get("/production/lots/:id/quality/history", s.productionQualityHistory)
	r.Get("/production/lots/:id/quality", s.productionQuality)
}
func qualityError(c *fiber.Ctx, err error) error {
	if errors.Is(err, production.ErrQuality) {
		return c.SendStatus(409)
	}
	return productionError(c, err)
}
func (s *Server) recordProductionQuality(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.QualityInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "lot_id", "expected_revision", "status", "criterion", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.RecordQualityReview(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return qualityError(c, err)
	}
	status := 201
	if out.Repeated {
		status = 200
	}
	return c.Status(status).JSON(out)
}
func (s *Server) productionQuality(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.GetLotQuality(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return qualityError(c, err)
	}
	return c.JSON(out)
}
func (s *Server) productionQualityHistory(c *fiber.Ctx) error {
	offset, err := orderOffset(c)
	if err != nil {
		return qualityError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.LotQualityHistory(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), offset)
	if err != nil {
		return qualityError(c, err)
	}
	return c.JSON(fiber.Map{"items": out})
}
