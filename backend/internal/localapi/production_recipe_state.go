package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountRecipeState(r fiber.Router) {
	r.Get("/production/recipes/:id/state", s.getRecipeState)
	r.Post("/production/recipe-state", s.changeRecipeState)
}
func (s *Server) getRecipeState(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	out, err := production.GetRecipeState(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(out)
}
func (s *Server) changeRecipeState(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in production.RecipeStateInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "recipe_id", "expected_revision", "status", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.ChangeRecipeState(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(out)
}
