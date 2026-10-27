package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionTrace(r fiber.Router) {
	r.Get("/production/orders/:id/trace", s.productionOrderTrace)
}
func traceLotOffset(c *fiber.Ctx) (int, error) {
	seen := false
	valid := true
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) != "lot_offset" || seen {
			valid = false
		}
		seen = true
	})
	n, err := strconv.Atoi(c.Query("lot_offset", "0"))
	if !valid || err != nil || n < 0 || int64(n) > production.MaxQuantity {
		return 0, production.ErrInvalid
	}
	return n, nil
}
func (s *Server) productionOrderTrace(c *fiber.Ctx) error {
	offset, err := traceLotOffset(c)
	if err != nil {
		return productionError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.GetOrderTrace(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), offset)
	if errors.Is(err, production.ErrTrace) || errors.Is(err, production.ErrLot) || errors.Is(err, production.ErrLoss) || errors.Is(err, production.ErrQuality) {
		return c.SendStatus(409)
	}
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(out)
}
