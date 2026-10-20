package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/payables"
)

func (s *Server) mountPayableCash(r fiber.Router) {
	r.Get("/finance/payable-cash/operations/:id", s.payableCashReceipt)
	r.Post("/finance/payable-cash/:id/return", s.returnPayableCash)
	r.Post("/finance/payables/:id/cash-payments", s.recordPayableCash)
	r.Get("/finance/payables/:id/cash-payments", s.listPayableCash)
}
func (s *Server) recordPayableCash(c *fiber.Ctx) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in payables.CashInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "payment_id", "installment_id", "withdrawal_operation_id", "recipient", "reference"}) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.PayCash(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, c.Params("id"), in)
	if e != nil {
		return payableError(c, e)
	}
	code := 201
	if r.Repeated {
		code = 200
	}
	return c.Status(code).JSON(r)
}
func (s *Server) returnPayableCash(c *fiber.Ctx) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in payables.CashReturnInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "supply_operation_id", "reason"}) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.ReturnCash(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, c.Params("id"), in)
	if e != nil {
		return payableError(c, e)
	}
	code := 201
	if r.Repeated {
		code = 200
	}
	return c.Status(code).JSON(r)
}
func (s *Server) listPayableCash(c *fiber.Ctx) error {
	limit, offset, ok := vaultBounds(c)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.CashPayments(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"), limit, offset)
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) payableCashReceipt(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.CashReceipt(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
