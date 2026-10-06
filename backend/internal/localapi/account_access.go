package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"mime"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) resetStaffPassword(c *fiber.Ctx) error  { return s.adminAccess(c, "password_reset") }
func (s *Server) revokeStaffSessions(c *fiber.Ctx) error { return s.adminAccess(c, "sessions_revoked") }

func accessBody(c *fiber.Ctx, target any, fields []string) error {
	if len(c.Body()) > 4096 {
		return fiber.ErrRequestEntityTooLarge
	}
	media, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	if err != nil || media != "application/json" {
		return fiber.ErrUnsupportedMediaType
	}
	if err := decodeCash(c.Body(), target, fields); err != nil {
		return fiber.ErrBadRequest
	}
	return nil
}

func (s *Server) adminAccess(c *fiber.Ctx, kind string) error {
	var input localauth.AdminInput
	fields := []string{"operation_id", "current_password", "reason"}
	if kind == "password_reset" {
		fields = append(fields, "new_password")
	}
	if err := accessBody(c, &input, fields); err != nil {
		return err
	}
	session := c.Locals("session").(localauth.Session)
	result, err := localauth.AdministerAccess(c.UserContext(), s.DB, s.Device, session.Token, c.Params("id"), kind, input)
	if err != nil {
		return accessError(c, err)
	}
	return c.JSON(result)
}

func (s *Server) recoverOwner(c *fiber.Ctx) error {
	var input struct {
		IdentityID  string `json:"identity_id"`
		RecoveryKey string `json:"recovery_key"`
		NewPassword string `json:"new_password"`
	}
	if err := accessBody(c, &input, []string{"identity_id", "recovery_key", "new_password"}); err != nil {
		return err
	}
	if err := localauth.RecoverOwner(c.UserContext(), s.DB, s.Device, input.IdentityID, input.RecoveryKey, input.NewPassword); err != nil {
		return accessError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) revokeOwnRecovery(c *fiber.Ctx) error {
	values, err := accountStrings(c, []string{"current_password"})
	if err != nil {
		return err
	}
	session := c.Locals("session").(localauth.Session)
	if err := localauth.RevokeOwnRecovery(c.UserContext(), s.DB, s.Device, session.Token, values["current_password"]); err != nil {
		return accessError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func accessError(c *fiber.Ctx, err error) error {
	code, status := "internal_error", fiber.StatusInternalServerError
	switch {
	case errors.Is(err, localauth.ErrDenied):
		code, status = "access_verification_failed", fiber.StatusUnauthorized
	case errors.Is(err, localauth.ErrAccessDenied):
		code, status = "access_management_denied", fiber.StatusForbidden
	case errors.Is(err, localauth.ErrAccessInput):
		code, status = "invalid_access_input", fiber.StatusBadRequest
	case errors.Is(err, localauth.ErrAccessConflict):
		code, status = "access_operation_conflict", fiber.StatusConflict
	}
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code}})
}
