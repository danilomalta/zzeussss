package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/replenishment"
)

func (s *Server) mountReplenishment(r fiber.Router) {
	r.Get("/replenishment/products", s.restockProducts)
	r.Get("/replenishment/suggestions", s.restockSuggestions)
	r.Get("/replenishment/operations/:kind/:id", s.restockOperation)
	r.Post("/replenishment/policies", s.restockPolicy)
	r.Post("/replenishment/suggestions", s.restockSuggest)
	r.Post("/replenishment/reviews", s.restockReview)
}
func restockError(c *fiber.Ctx, e error) error {
	code := ""
	status := 409
	switch {
	case errors.Is(e, replenishment.ErrInvalid):
		code = "invalid_replenishment"
		status = 400
	case errors.Is(e, replenishment.ErrNoPolicy):
		code = "missing_policy"
	case errors.Is(e, replenishment.ErrStale):
		code = "stale_suggestion"
	case errors.Is(e, replenishment.ErrPendingApproval):
		code = "pending_approval"
	case errors.Is(e, replenishment.ErrConflict):
		code = "operation_conflict"
	case errors.Is(e, replenishment.ErrNotFound):
		status = 404
		code = "not_found"
	default:
		return catalogError(c, e)
	}
	return c.Status(status).JSON(fiber.Map{"code": code})
}
func (s *Server) restockProducts(c *fiber.Ctx) error {
	n, e := purchaseOffset(c)
	if e != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, e := replenishment.Products(c.UserContext(), s.DB, v.Actor, v.Device, n)
	if e != nil {
		return restockError(c, e)
	}
	return c.JSON(fiber.Map{"items": out})
}
func (s *Server) restockSuggestions(c *fiber.Ctx) error {
	n, e := purchaseOffset(c)
	if e != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, e := replenishment.Suggestions(c.UserContext(), s.DB, v.Actor, v.Device, n)
	if e != nil {
		return restockError(c, e)
	}
	return c.JSON(fiber.Map{"items": out})
}
func (s *Server) restockOperation(c *fiber.Ctx) error {
	v := c.Locals("session").(localauth.Session)
	out, e := replenishment.Operation(c.UserContext(), s.DB, v.Actor, v.Device, c.Params("kind"), c.Params("id"))
	if e != nil {
		return restockError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) restockPolicy(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in replenishment.PolicyInput
	if e := decodeCash(c.Body(), &in, []string{"operation_id", "product_id", "minimum_milli", "target_milli"}); e != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, e := replenishment.SetPolicyWithContract(c.UserContext(), s.DB, s.contracts, v.Actor, v.Device, in)
	if e != nil {
		return restockError(c, e)
	}
	return c.JSON(fiber.Map{"operation_id": in.OperationID, "revision": out.Revision, "repeated": out.Repeated})
}
func (s *Server) restockSuggest(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in replenishment.SuggestInput
	if e := decodeCash(c.Body(), &in, []string{"operation_id", "product_id"}); e != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, e := replenishment.SuggestWithContract(c.UserContext(), s.DB, s.contracts, v.Actor, v.Device, in)
	if e != nil {
		return restockError(c, e)
	}
	return c.JSON(fiber.Map{"operation_id": in.OperationID, "suggestion_id": out.SuggestionID, "observed_milli": out.ObservedMilli, "recommended_milli": out.RecommendedMilli, "policy_revision": out.PolicyRevision, "needed": out.Needed, "repeated": out.Repeated})
}
func (s *Server) restockReview(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in replenishment.ReviewInput
	if e := decodeCash(c.Body(), &in, []string{"operation_id", "suggestion_id", "decision", "reason"}); e != nil {
		return c.SendStatus(400)
	}
	v := c.Locals("session").(localauth.Session)
	out, e := replenishment.ReviewWithContract(c.UserContext(), s.DB, s.contracts, v.Actor, v.Device, in)
	if e != nil {
		return restockError(c, e)
	}
	return c.JSON(fiber.Map{"operation_id": in.OperationID, "suggestion_id": out.SuggestionID, "decision": out.Decision, "repeated": out.Repeated})
}
