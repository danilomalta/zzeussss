package localapi

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionCapacity(r fiber.Router) {
	r.Get("/production/capacity", s.productionCapacity)
	r.Post("/production/capacity/alternatives", s.productionAlternatives)
}

func capacityError(c *fiber.Ctx, err error) error {
	if errors.Is(err, production.ErrCapacity) {
		return c.SendStatus(409)
	}
	return productionError(c, err)
}

func (s *Server) productionCapacity(c *fiber.Ctx) error {
	seen := map[string]bool{}
	valid := true
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || (key != "version_id" && key != "location_id") {
			valid = false
		}
		seen[key] = true
	})
	if !valid || len(seen) != 2 {
		return c.SendStatus(400)
	}
	in := production.CapacityInput{LocationID: c.Query("location_id"), VersionIDs: []string{c.Query("version_id")}}
	return s.sendCapacity(c, in)
}

func (s *Server) productionAlternatives(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.CapacityInput
	if err := decodeCash(c.Body(), &in, []string{"location_id", "version_ids"}); err != nil {
		return c.SendStatus(400)
	}
	return s.sendCapacity(c, in)
}

func (s *Server) sendCapacity(c *fiber.Ctx, in production.CapacityInput) error {
	session := c.Locals("session").(localauth.Session)
	v, err := production.Capacities(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if err != nil {
		return capacityError(c, err)
	}
	return c.JSON(v)
}
