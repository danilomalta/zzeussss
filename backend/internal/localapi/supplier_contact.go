package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/supplycycle"
)

func (s *Server) supplierContact(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	if c.Method() == "GET" {
		if len(c.Body()) != 0 {
			return c.SendStatus(400)
		}
		r, e := supplycycle.SupplierContactAlert(c.UserContext(), s.DB, ss.Actor, ss.Device, ss.Token, id)
		if e != nil {
			return cycleError(c, e)
		}
		return c.JSON(r)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.ContactPolicyInput
	if e := cycleDecode(c, &in, "operation_id", "expected_revision", "enabled", "threshold", "since", "count_mode"); e != nil {
		return e
	}
	r, e := supplycycle.ConfigureContactPolicy(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, ss.Token, id, in)
	return cycleReply(c, r, e)
}
