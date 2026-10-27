package localapi

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stock"
)

func (s *Server) mountStockAvailability(r fiber.Router) {
	r.Get("/stock/availability", s.stockAvailability)
}
func (s *Server) stockAvailability(c *fiber.Ctx) error {
	product, location := "", ""
	seen := map[string]bool{}
	valid := true
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key, value := string(k), string(v)
		if seen[key] || value == "" {
			valid = false
			return
		}
		seen[key] = true
		switch key {
		case "product_id":
			product = value
		case "location_id":
			location = value
		default:
			valid = false
		}
	})
	if !valid {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := stock.AvailableBalance(c.UserContext(), s.DB, session.Actor, session.Device, product, location)
	if errors.Is(err, stock.ErrAvailability) {
		return c.SendStatus(409)
	}
	if errors.Is(err, stock.ErrAvailabilityReference) {
		return c.SendStatus(404)
	}
	if err != nil {
		return stockError(c, err)
	}
	return c.JSON(out)
}
