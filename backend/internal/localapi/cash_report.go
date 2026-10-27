package localapi

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/register"
)

func (s *Server) closedCashReport(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if c.Context().QueryArgs().Len() != 0 || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := register.ReadClosedReport(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	switch {
	case errors.Is(err, register.ErrReportNotFound):
		return c.SendStatus(404)
	case errors.Is(err, register.ErrReportOpen), errors.Is(err, register.ErrReportInconsistent):
		return c.SendStatus(409)
	case errors.Is(err, register.ErrReportRange):
		return c.SendStatus(422)
	case err != nil:
		return cashError(c, err)
	default:
		return c.JSON(v)
	}
}
