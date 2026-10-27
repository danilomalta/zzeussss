package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/supplycycle"
)

func (s *Server) tradeMessages(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset", "direction", "kind") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.ListTradeMessages(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Query("direction"), c.Query("kind"), limit, offset)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) tradeMessage(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.ReadTradeMessage(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) tradePeers(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.ListTradePeers(c.UserContext(), s.DB, ss.Actor, ss.Device, limit, offset)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}

func (s *Server) tradeRecovery(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.ReadTradeRecovery(c.UserContext(), s.DB, ss.Actor, ss.Device)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) tradeRecoveryReview(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.TradeRecoveryInput
	if e := cycleDecode(c, &in, "operation_id", "generation", "verification_reference"); e != nil {
		return e
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.ReviewRestoredTrade(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
