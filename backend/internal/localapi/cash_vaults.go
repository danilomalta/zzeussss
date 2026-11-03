package localapi

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/cashcustody"
	"titansystem-backend/internal/localdb/cashvault"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/register"
)

func (s *Server) mountCashVaults(r fiber.Router) {
	r.Post("/cash/vaults", s.createCashVault)
	r.Get("/cash/vaults", s.listCashVaults)
	r.Get("/cash/vaults/pending-bags", s.pendingVaultBags)
	r.Get("/cash/vaults/operations/:id", s.vaultOperation)
	r.Get("/cash/vaults/:id/statement", s.vaultStatement)
	r.Get("/cash/vaults/:id", s.readCashVault)
	for _, action := range []string{"deposit", "transfer", "supply", "archive"} {
		action := action
		r.Post("/cash/vaults/:id/"+action, func(c *fiber.Ctx) error { return s.applyCashVault(c, action) })
	}
}
func vaultError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, cashvault.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(err, cashvault.ErrNotFound):
		return c.SendStatus(404)
	case errors.Is(err, cashvault.ErrConflict), errors.Is(err, cashvault.ErrInconsistent), errors.Is(err, register.ErrFunds):
		return c.SendStatus(409)
	case errors.Is(err, cashvault.ErrRange):
		return c.SendStatus(422)
	case errors.Is(err, cashcustody.ErrInvalid), errors.Is(err, cashcustody.ErrNotFound), errors.Is(err, cashcustody.ErrInconsistent):
		return bagError(c, err)
	default:
		return cashError(c, err)
	}
}
func vaultBounds(c *fiber.Ctx) (int, int, bool) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	limit, offset := 50, 0
	valid := len(c.Body()) == 0
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		name := string(k)
		if seen[name] || (name != "limit" && name != "offset") || len(v) == 0 {
			valid = false
			return
		}
		seen[name] = true
		for _, b := range v {
			if b < '0' || b > '9' {
				valid = false
				return
			}
		}
		n, err := strconv.Atoi(string(v))
		if err != nil {
			valid = false
			return
		}
		if name == "limit" {
			limit = n
		} else {
			offset = n
		}
	})
	return limit, offset, valid
}
func (s *Server) createCashVault(c *fiber.Ctx) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in cashvault.CreateInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "vault_id", "name"}) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashvault.Create(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in)
	if err != nil {
		return vaultError(c, err)
	}
	code := 201
	if r.Repeated {
		code = 200
	}
	return c.Status(code).JSON(r)
}
func (s *Server) applyCashVault(c *fiber.Ctx, kind string) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	fields := []string{"operation_id"}
	switch kind {
	case "deposit":
		fields = append(fields, "bag_id")
	case "transfer":
		fields = append(fields, "destination_id", "amount_cents", "reason")
	case "supply":
		fields = append(fields, "cash_session_id", "amount_cents", "reason")
	case "archive":
		fields = append(fields, "reason")
	}
	var in cashvault.Input
	if decodeCash(c.Body(), &in, fields) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashvault.Apply(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, c.Params("id"), kind, in)
	if err != nil {
		return vaultError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) readCashVault(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashvault.Get(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if err != nil {
		return vaultError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) vaultOperation(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashvault.Receipt(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if err != nil {
		return vaultError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) listCashVaults(c *fiber.Ctx) error {
	limit, offset, ok := vaultBounds(c)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashvault.List(c.UserContext(), s.DB, ss.Actor, ss.Device, limit, offset)
	if err != nil {
		return vaultError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) vaultStatement(c *fiber.Ctx) error {
	limit, offset, ok := vaultBounds(c)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashvault.ReadStatement(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"), limit, offset)
	if err != nil {
		return vaultError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) pendingVaultBags(c *fiber.Ctx) error {
	limit, offset, ok := vaultBounds(c)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashvault.Pending(c.UserContext(), s.DB, ss.Actor, ss.Device, limit, offset)
	if err != nil {
		return vaultError(c, err)
	}
	return c.JSON(r)
}
