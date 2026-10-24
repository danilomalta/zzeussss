package localapi

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) closedCashJournal(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	limit, offset := 50, 0
	valid := len(c.Body()) == 0
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(key, value []byte) {
		k := string(key)
		if seen[k] || (k != "limit" && k != "offset") || len(value) == 0 {
			valid = false
			return
		}
		seen[k] = true
		for _, b := range value {
			if b < '0' || b > '9' {
				valid = false
				return
			}
		}
		n, err := strconv.Atoi(string(value))
		if err != nil {
			valid = false
			return
		}
		if k == "limit" {
			limit = n
		} else {
			offset = n
		}
	})
	if !valid {
		return c.SendStatus(400)
	}
	id, err := journalIDParam(c)
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashjournal.Read(c.UserContext(), s.DB, session.Actor, session.Device, id, limit, offset)
	if err != nil {
		return cashJournalError(c, err)
	}
	return c.JSON(v)
}

func cashJournalError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, cashjournal.ErrExportLimit):
		return c.SendStatus(413)
	case errors.Is(err, cashjournal.ErrExportText):
		return c.SendStatus(422)
	case errors.Is(err, cashjournal.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(err, cashjournal.ErrNotFound):
		return c.SendStatus(404)
	case errors.Is(err, cashjournal.ErrOpen), errors.Is(err, cashjournal.ErrInconsistent):
		return c.SendStatus(409)
	case errors.Is(err, cashjournal.ErrRange):
		return c.SendStatus(422)
	case err != nil:
		return catalogError(c, err)
	default:
		return c.SendStatus(500)
	}
}

// Decode exactly once: Fiber keeps the original escaped path by default.
func journalIDParam(c *fiber.Ctx) (string, error) { return url.PathUnescape(c.Params("id")) }
