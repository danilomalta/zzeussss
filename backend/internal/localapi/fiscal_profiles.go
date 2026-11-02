package localapi

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) listFiscalProfiles(c *fiber.Ctx) error {
	if !fiscalRead(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	limit, after, valid := 20, "", true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] {
			valid = false
		}
		seen[key] = true
		switch key {
		case "limit":
			n, e := strconv.Atoi(string(v))
			if e != nil || n < 1 || n > 50 {
				valid = false
			}
			limit = n
		case "after":
			after = string(v)
			if after == "" {
				valid = false
			}
		default:
			valid = false
		}
	})
	if !valid {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.ListProfiles(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, ss.Token, after, limit)
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(out)
}
