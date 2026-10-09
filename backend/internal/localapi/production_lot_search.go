package localapi

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionLotSearch(r fiber.Router) {
	r.Get("/production/lot-search", s.productionLotSearch)
}
func parseLotSearch(c *fiber.Ctx) (production.LotSearch, error) {
	in := production.LotSearch{}
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
		case "status":
			in.Status = value
		case "quality":
			in.Quality = value
		case "product_id":
			in.ProductID = value
		case "location_id":
			in.LocationID = value
		case "expiry":
			in.Expiry = value
		case "expires_from":
			in.ExpiresFrom = value
		case "expires_to":
			in.ExpiresTo = value
		case "offset":
			for _, ch := range value {
				if ch < '0' || ch > '9' {
					valid = false
				}
			}
			var err error
			in.Offset, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				valid = false
			}
		default:
			valid = false
		}
	})
	if !valid {
		return in, production.ErrInvalid
	}
	return in, nil
}
func (s *Server) productionLotSearch(c *fiber.Ctx) error {
	in, err := parseLotSearch(c)
	if err != nil {
		return productionError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.SearchProductionLots(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if errors.Is(err, production.ErrTrace) || errors.Is(err, production.ErrQuality) {
		return c.SendStatus(409)
	}
	if err != nil {
		return lotError(c, err)
	}
	return c.JSON(out)
}
