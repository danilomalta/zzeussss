package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/cashcount"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountVaultCounts(r fiber.Router) {
	// Reports must precede the generic /cash/vaults/:id route.
	r.Get("/cash/vaults/report", s.vaultReport)
	r.Get("/cash/vaults/report.csv", s.vaultReportCSV)
	r.Post("/cash/vault-counts", s.createVaultCount)
	r.Get("/cash/vault-counts", s.listVaultCounts)
	r.Get("/cash/vault-counts/operations/:id", s.vaultCountOperation)
	r.Get("/cash/vault-counts/:id", s.readVaultCount)
	for _, action := range []string{"submit", "accept", "reject", "cancel"} {
		action := action
		r.Post("/cash/vault-counts/:id/"+action, func(c *fiber.Ctx) error { return s.changeVaultCount(c, action) })
	}
}
func countError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, cashcount.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(err, cashcount.ErrNotFound):
		return c.SendStatus(404)
	case errors.Is(err, cashcount.ErrConflict), errors.Is(err, cashcount.ErrInconsistent), errors.Is(err, cashcount.ErrText):
		return c.SendStatus(409)
	case errors.Is(err, cashcount.ErrRange):
		return c.SendStatus(422)
	default:
		return vaultError(c, err)
	}
}
func (s *Server) createVaultCount(c *fiber.Ctx) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in cashcount.CreateInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "count_id", "vault_id", "counter_id", "reason"}) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashcount.Create(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in)
	if err != nil {
		return countError(c, err)
	}
	code := 201
	if r.Repeated {
		code = 200
	}
	return c.Status(code).JSON(r)
}
func (s *Server) changeVaultCount(c *fiber.Ctx, action string) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	fields := []string{"operation_id", "reason"}
	if action == "submit" {
		fields = []string{"operation_id", "counted_cents"}
	}
	var in cashcount.ActionInput
	if decodeCash(c.Body(), &in, fields) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashcount.Transition(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, c.Params("id"), action, in)
	if err != nil {
		return countError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) readVaultCount(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashcount.Get(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if err != nil {
		return countError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) vaultCountOperation(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashcount.Receipt(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if err != nil {
		return countError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) listVaultCounts(c *fiber.Ctx) error {
	limit, offset, ok := vaultBounds(c)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashcount.List(c.UserContext(), s.DB, ss.Actor, ss.Device, limit, offset)
	if err != nil {
		return countError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) vaultReport(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashcount.ReportVaults(c.UserContext(), s.DB, ss.Actor, ss.Device)
	if err != nil {
		return countError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) vaultReportCSV(c *fiber.Ctx) error {
	c.Set("X-Content-Type-Options", "nosniff")
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	data, err := cashcount.ExportCSV(c.UserContext(), s.DB, ss.Actor, ss.Device)
	if err != nil {
		return countError(c, err)
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="titan-cofres.csv"`)
	return c.Send(data)
}
