package localapi

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func (s *Server) saleHistory(c *fiber.Ctx) error {
	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	items, err := sale.History(c.UserContext(), s.DB, session.Actor, session.Device, limit, offset)
	if err != nil {
		return saleError(c, err)
	}
	return c.JSON(fiber.Map{"items": items})
}
