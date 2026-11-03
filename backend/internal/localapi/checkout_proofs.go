package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func (s *Server) mountCheckoutProofs(r fiber.Router) {
	r.Post("/checkout/plans/:id/parts/:part/proofs", s.submitCheckoutProof)
	r.Get("/checkout/proofs", s.listCheckoutProofs)
	r.Get("/checkout/proofs/report", s.reportCheckoutProofs)
	r.Get("/checkout/proofs/report.csv", s.exportCheckoutProofs)
	r.Get("/checkout/proofs/operations/:id", s.checkoutProofOperation)
	r.Get("/checkout/proofs/:id", s.readCheckoutProof)
	for _, action := range []string{"accept", "reject", "withdraw", "revoke"} {
		action := action
		r.Post("/checkout/proofs/:id/"+action, func(c *fiber.Ctx) error { return s.changeCheckoutProof(c, action) })
	}
}
func proofError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, sale.ErrProofInvalid):
		return c.SendStatus(400)
	case errors.Is(err, sale.ErrProofNotFound):
		return c.SendStatus(404)
	case errors.Is(err, sale.ErrProofConflict), errors.Is(err, sale.ErrProofIntegrity), errors.Is(err, sale.ErrProofText):
		return c.SendStatus(409)
	case errors.Is(err, sale.ErrProofRange):
		return c.SendStatus(422)
	default:
		return saleError(c, err)
	}
}
func (s *Server) submitCheckoutProof(c *fiber.Ctx) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in sale.SubmitProofInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "proof_id", "provider", "terminal", "reference", "occurred_at"}) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := sale.SubmitPaymentProof(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, c.Params("id"), c.Params("part"), in)
	if err != nil {
		return proofError(c, err)
	}
	code := 201
	if r.Repeated {
		code = 200
	}
	return c.Status(code).JSON(r)
}
func (s *Server) changeCheckoutProof(c *fiber.Ctx, action string) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in sale.ReviewProofInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "reason"}) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := sale.ReviewPaymentProof(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, c.Params("id"), action, in)
	if err != nil {
		return proofError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) readCheckoutProof(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := sale.ReadPaymentProof(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if err != nil {
		return proofError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) checkoutProofOperation(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := sale.PaymentProofReceipt(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if err != nil {
		return proofError(c, err)
	}
	return c.JSON(r)
}
func proofBounds(c *fiber.Ctx) (string, int, int, bool) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	state := "submitted"
	limit, offset := 50, 0
	ok := len(c.Body()) == 0
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		name := string(k)
		if seen[name] || (name != "limit" && name != "offset" && name != "state") || len(v) == 0 {
			ok = false
			return
		}
		seen[name] = true
		if name == "state" {
			state = string(v)
			return
		}
		for _, b := range v {
			if b < '0' || b > '9' {
				ok = false
				return
			}
		}
		n, err := strconv.Atoi(string(v))
		if err != nil {
			ok = false
			return
		}
		if name == "limit" {
			limit = n
		} else {
			offset = n
		}
	})
	return state, limit, offset, ok
}
func (s *Server) listCheckoutProofs(c *fiber.Ctx) error {
	state, limit, offset, ok := proofBounds(c)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := sale.ListPaymentProofs(c.UserContext(), s.DB, ss.Actor, ss.Device, state, limit, offset)
	if err != nil {
		return proofError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) reportCheckoutProofs(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := sale.ReportPaymentProofs(c.UserContext(), s.DB, ss.Actor, ss.Device)
	if err != nil {
		return proofError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) exportCheckoutProofs(c *fiber.Ctx) error {
	c.Set("X-Content-Type-Options", "nosniff")
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	data, err := sale.ExportPaymentProofsCSV(c.UserContext(), s.DB, ss.Actor, ss.Device)
	if err != nil {
		return proofError(c, err)
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="titan-comprovantes-checkout.csv"`)
	return c.Send(data)
}
