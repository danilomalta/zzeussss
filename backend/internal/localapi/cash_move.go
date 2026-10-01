package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/register"
)

func (s *Server) moveCash(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	var in register.MovementInput
	if err := decodeCash(c.Body(), &in, []string{"session_id", "operation_id", "kind", "amount_cents", "reason"}); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := register.MoveWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if errors.Is(err, register.ErrFunds) {
		return c.SendStatus(fiber.StatusConflict)
	}
	if err != nil {
		return cashError(c, err)
	}
	return c.JSON(result)
}
