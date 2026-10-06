package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"io"
	"mime"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) ownSessions(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	items, err := localauth.ListSessions(c.UserContext(), s.DB, s.Device, session.Token)
	if err != nil {
		return accountSecurityError(c, err)
	}
	return c.JSON(fiber.Map{"items": items, "limit": 100})
}

func (s *Server) changeOwnPassword(c *fiber.Ctx) error {
	values, err := accountStrings(c, []string{"current_password", "new_password"})
	if err != nil {
		return err
	}
	session := c.Locals("session").(localauth.Session)
	if err := localauth.ChangeOwnPassword(c.UserContext(), s.DB, s.Device, session.Token, values["current_password"], values["new_password"]); err != nil {
		return accountSecurityError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) revokeOtherSessions(c *fiber.Ctx) error {
	values, err := accountStrings(c, []string{"current_password"})
	if err != nil {
		return err
	}
	session := c.Locals("session").(localauth.Session)
	if err := localauth.RevokeOtherSessions(c.UserContext(), s.DB, s.Device, session.Token, values["current_password"]); err != nil {
		return accountSecurityError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func accountSecurityError(c *fiber.Ctx, err error) error {
	status, code := fiber.StatusInternalServerError, "internal_error"
	if errors.Is(err, localauth.ErrDenied) {
		status, code = fiber.StatusUnauthorized, "account_verification_failed"
	}
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code}})
}

// Exact, small string-only JSON; duplicated, unknown, null and trailing values
// are rejected. Never echo input containing passwords in an error.
func accountStrings(c *fiber.Ctx, fields []string) (map[string]string, error) {
	if len(c.Body()) > 2048 {
		return nil, fiber.NewError(fiber.StatusRequestEntityTooLarge)
	}
	media, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	if err != nil || media != "application/json" {
		return nil, fiber.NewError(fiber.StatusUnsupportedMediaType)
	}
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fiber.ErrBadRequest
	}
	allowed := make(map[string]bool)
	for _, field := range fields {
		allowed[field] = true
	}
	result := make(map[string]string)
	for dec.More() {
		key, err := dec.Token()
		name, ok := key.(string)
		if err != nil || !ok || !allowed[name] {
			return nil, fiber.ErrBadRequest
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fiber.ErrBadRequest
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fiber.ErrBadRequest
		}
		var value string
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &value) != nil || value == "" || len(value) > 72 {
			return nil, fiber.ErrBadRequest
		}
		result[name] = value
	}
	end, err := dec.Token()
	if err != nil || end != json.Delim('}') || len(result) != len(fields) {
		return nil, fiber.ErrBadRequest
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fiber.ErrBadRequest
	}
	return result, nil
}
