package localapi

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionSearch(r fiber.Router) {
	r.Get("/production/order-search", s.productionOrderSearch)
}
func parseOrderSearch(c *fiber.Ctx) (production.OrderSearch, error) {
	in := production.OrderSearch{}
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
		case "location_id":
			in.LocationID = value
		case "responsible_id":
			in.ResponsibleID = value
		case "version_id":
			in.VersionID = value
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
func (s *Server) productionOrderSearch(c *fiber.Ctx) error {
	in, err := parseOrderSearch(c)
	if err != nil {
		return productionError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.SearchOrders(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if errors.Is(err, production.ErrTrace) {
		return c.SendStatus(409)
	}
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(out)
}
