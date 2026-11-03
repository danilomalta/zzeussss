package localapi

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionProductUsage(r fiber.Router) {
	r.Get("/catalog/products/:id/production-usage", s.productionProductUsage)
}
func (s *Server) productionProductUsage(c *fiber.Ctx) error {
	kind, offset := "versions", int64(0)
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
		case "kind":
			kind = value
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
	out, err := production.GetProductUsage(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), kind, offset)
	if errors.Is(err, production.ErrTrace) {
		return c.SendStatus(409)
	}
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(out)
}
