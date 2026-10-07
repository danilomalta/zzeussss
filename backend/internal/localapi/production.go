package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProduction(r fiber.Router) {
	r.Post("/production/recipe-versions", s.publishRecipe)
	r.Get("/production/recipe-versions", s.recipeVersions)
	r.Get("/production/recipe-versions/:id", s.recipeVersion)
}

func productionError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, production.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(err, production.ErrConflict), errors.Is(err, production.ErrMaterials), errors.Is(err, production.ErrResult):
		return c.SendStatus(409)
	case errors.Is(err, production.ErrNotFound):
		return c.SendStatus(404)
	default:
		return catalogError(c, err)
	}
}

// Validate exact objects at BOTH levels; ordinary Unmarshal accepts duplicate
// nested fields. Quantities decode to int64 without floating point conversion.
func decodeRecipe(body []byte) (production.PublishInput, error) {
	var envelope struct {
		Ingredients json.RawMessage `json:"ingredients"`
	}
	fields := []string{"operation_id", "recipe_id", "version_id", "expected_revision", "name", "output_product_id", "output_unit", "yield_milli", "ingredients"}
	if err := decodeCash(body, &envelope, fields); err != nil {
		return production.PublishInput{}, production.ErrInvalid
	}
	raw := bytes.TrimSpace(envelope.Ingredients)
	if len(raw) == 0 || raw[0] != '[' {
		return production.PublishInput{}, production.ErrInvalid
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) < 1 || len(items) > 100 {
		return production.PublishInput{}, production.ErrInvalid
	}
	for _, item := range items {
		var ingredient production.Ingredient
		if err := decodeCash(item, &ingredient, []string{"product_id", "unit", "quantity_milli"}); err != nil {
			return production.PublishInput{}, production.ErrInvalid
		}
	}
	var in production.PublishInput
	if err := json.Unmarshal(body, &in); err != nil {
		return production.PublishInput{}, production.ErrInvalid
	}
	return in, nil
}

func productionQuery(c *fiber.Ctx, list bool) bool {
	seen := map[string]bool{}
	ok := true
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || !list || (key != "recipe_id" && key != "limit" && key != "offset") {
			ok = false
		}
		seen[key] = true
	})
	return ok
}

func (s *Server) publishRecipe(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 65536 {
		return c.SendStatus(413)
	}
	in, err := decodeRecipe(c.Body())
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.Publish(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return productionError(c, err)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}

func (s *Server) recipeVersions(c *fiber.Ctx) error {
	if !productionQuery(c, true) {
		return c.SendStatus(400)
	}
	limit, e1 := strconv.Atoi(c.Query("limit", "50"))
	offset, e2 := strconv.Atoi(c.Query("offset", "0"))
	if e1 != nil || e2 != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.List(c.UserContext(), s.DB, session.Actor, session.Device, c.Query("recipe_id"), limit, offset)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(fiber.Map{"items": v})
}

func (s *Server) recipeVersion(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := production.Get(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(v)
}
