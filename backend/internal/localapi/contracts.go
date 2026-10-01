package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) installContract(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 64*1024 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	envelope, err := decodeEnvelope(c.Body())
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := s.contracts.Install(c.UserContext(), session.Actor, session.Device, envelope)
	if err != nil {
		switch {
		case errors.Is(err, identity.ErrDenied):
			return c.SendStatus(fiber.StatusForbidden)
		case errors.Is(err, entitlementstore.ErrStale), errors.Is(err, entitlementstore.ErrConflict), errors.Is(err, entitlementstore.ErrClockRollback):
			return c.SendStatus(fiber.StatusConflict)
		case errors.Is(err, entitlements.ErrTrust), errors.Is(err, entitlements.ErrSignature),
			errors.Is(err, entitlements.ErrClaims), errors.Is(err, entitlements.ErrTenant), errors.Is(err, entitlements.ErrValidity):
			return c.SendStatus(fiber.StatusBadRequest)
		default:
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}
	return c.JSON(fiber.Map{"revision": result.Revision, "repeated": result.Repeated})
}

// Reject unknown or duplicate fields so company IDs and issuer keys cannot be
// smuggled into the request. Company and device come only from the session.
func decodeEnvelope(body []byte) (entitlements.Envelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return entitlements.Envelope{}, entitlements.ErrClaims
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] || (name != "key_id" && name != "payload" && name != "signature") {
			return entitlements.Envelope{}, entitlements.ErrClaims
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return entitlements.Envelope{}, entitlements.ErrClaims
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return entitlements.Envelope{}, entitlements.ErrClaims
	}
	if _, err := decoder.Token(); err != io.EOF || len(seen) != 3 {
		return entitlements.Envelope{}, entitlements.ErrClaims
	}
	var envelope entitlements.Envelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return entitlements.Envelope{}, entitlements.ErrClaims
	}
	return envelope, nil
}
