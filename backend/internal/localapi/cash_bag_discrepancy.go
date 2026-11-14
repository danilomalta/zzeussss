package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/cashcustody"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) readCashBagReview(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashcustody.ReadReview(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return bagError(c, err)
	}
	return c.JSON(v)
}
func (s *Server) writeCashBagReview(c *fiber.Ctx) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in cashcustody.ReviewInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "expected_revision", "action", "counted_cents", "reason"}) != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashcustody.WriteReview(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, c.Params("id"), in)
	if err != nil {
		return bagError(c, err)
	}
	return c.JSON(v)
}
