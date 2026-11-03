package localapi

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stock"
)

func (s *Server) mountStockReservations(r fiber.Router) {
	r.Get("/stock/reservations", s.stockReservations)
}
func (s *Server) stockReservations(c *fiber.Ctx) error {
	product, location := "", ""
	offset := int64(0)
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
		case "offset":
			for _, ch := range value {
				if ch < '0' || ch > '9' {
					valid = false
				}
			}
			var err error
			offset, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				valid = false
			}
		default:
			valid = false
		}
	})
	if !valid {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := stock.ActiveReservations(c.UserContext(), s.DB, session.Actor, session.Device, product, location, offset)
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
