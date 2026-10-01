package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/register"
)

func (s *Server) mountCash(router fiber.Router) {
	router.Get("/cash/current", s.currentCash)
	router.Post("/cash/open", s.openCash)
	router.Post("/cash/close", s.closeCash)
}

func (s *Server) currentCash(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	current, err := register.Current(c.UserContext(), s.DB, session.Actor, session.Device)
	if err != nil {
		return cashError(c, err)
	}
	return c.JSON(fiber.Map{"session": current})
}

func (s *Server) openCash(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	var input register.OpenInput
	if err := decodeCash(c.Body(), &input, []string{"session_id", "opening_cents"}); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := register.OpenWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, input)
	if err != nil {
		return cashError(c, err)
	}
	status := fiber.StatusCreated
	if result.Repeated {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(fiber.Map{"session_id": result.SessionID, "repeated": result.Repeated})
}

func (s *Server) closeCash(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	var input register.CloseInput
	if err := decodeCash(c.Body(), &input, []string{"session_id", "operation_id", "declared_cents"}); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := register.CloseWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, input)
	if err != nil {
		return cashError(c, err)
	}
	return c.JSON(result)
}

func cashError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, register.ErrInvalid):
		return c.SendStatus(fiber.StatusBadRequest)
	case errors.Is(err, register.ErrConflict), errors.Is(err, register.ErrClosed):
		return c.SendStatus(fiber.StatusConflict)
	default:
		return catalogError(c, err)
	}
}

// A flat exact object: required fields, no duplicates, extra fields, nulls
// or trailing JSON. Integer fields are decoded directly into int64.
func decodeCash(body []byte, target any, fields []string) error {
	allowed := make(map[string]bool, len(fields))
	for _, name := range fields {
		allowed[name] = true
	}
	seen := make(map[string]bool, len(fields))
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return register.ErrInvalid
	}
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || !allowed[name] || seen[name] {
			return register.ErrInvalid
		}
		seen[name] = true
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return register.ErrInvalid
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(seen) != len(fields) {
		return register.ErrInvalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return register.ErrInvalid
	}
	if err := json.Unmarshal(body, target); err != nil {
		return register.ErrInvalid
	}
	return nil
}
