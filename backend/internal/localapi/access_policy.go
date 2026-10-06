package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/accesspolicy"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mutatePolicy(c *fiber.Ctx) error {
	var input accesspolicy.Input
	if err := accessBody(c, &input, []string{"operation_id", "current_password", "kind", "department_id", "target_id", "permission", "value", "name", "expected_revision", "reason"}); err != nil {
		return err
	}
	session := c.Locals("session").(localauth.Session)
	result, err := accesspolicy.Mutate(c.UserContext(), s.DB, s.Device, session.Token, input)
	if err != nil {
		return policyError(c, err)
	}
	return c.JSON(result)
}
func (s *Server) readPolicy(c *fiber.Ctx) error {
	if !policyQuery(c, false) {
		return policyError(c, accesspolicy.ErrInput)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := accesspolicy.Read(c.UserContext(), s.DB, s.Device, session.Token, c.Query("department_id"))
	if err != nil {
		return policyError(c, err)
	}
	return c.JSON(out)
}
func (s *Server) auditAccess(c *fiber.Ctx) error {
	if !policyQuery(c, true) {
		return policyError(c, accesspolicy.ErrInput)
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || strconv.Itoa(n) != raw || n < 1 || n > 100 {
			return policyError(c, accesspolicy.ErrInput)
		}
		limit = n
	}
	session := c.Locals("session").(localauth.Session)
	out, err := accesspolicy.Audit(c.UserContext(), s.DB, s.Device, session.Token, c.Query("department_id"), c.Query("cursor"), limit)
	if err != nil {
		return policyError(c, err)
	}
	return c.JSON(out)
}
func policyQuery(c *fiber.Ctx, audit bool) bool {
	valid := true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || (key != "department_id" && (!audit || (key != "limit" && key != "cursor"))) {
			valid = false
		}
		seen[key] = true
	})
	return valid
}
func policyError(c *fiber.Ctx, err error) error {
	status, code := 500, "internal_error"
	switch {
	case errors.Is(err, localauth.ErrDenied):
		status, code = 401, "session_or_password_invalid"
	case accesspolicy.IsDenied(err):
		status, code = 403, "policy_access_denied"
	case errors.Is(err, accesspolicy.ErrInput):
		status, code = 400, "invalid_policy_input"
	case errors.Is(err, accesspolicy.ErrConflict):
		status, code = 409, "policy_operation_conflict"
	}
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code}})
}
