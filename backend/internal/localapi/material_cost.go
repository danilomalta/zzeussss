package localapi

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountMaterialCost(r fiber.Router) {
	r.Get("/production/recipe-versions/:id/material-cost", s.recipeMaterialCost)
	r.Get("/production/orders/:id/material-cost", s.orderMaterialCost)
}
func (s *Server) recipeMaterialCost(c *fiber.Ctx) error {
	if !inventoryQuery(c, "batches") {
		return c.SendStatus(400)
	}
	batches := int64(1)
	if c.Context().QueryArgs().Has("batches") {
		raw := c.Query("batches")
		if raw == "" {
			return c.SendStatus(400)
		}
		for _, r := range raw {
			if r < '0' || r > '9' {
				return c.SendStatus(400)
			}
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return c.SendStatus(400)
		}
		batches = n
	}
	session := c.Locals("session").(localauth.Session)
	result, err := production.RecipeMaterialCost(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), batches)
	if err != nil {
		return productionError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(result)
}
func (s *Server) orderMaterialCost(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := production.OrderMaterialCost(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return productionError(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(result)
}
