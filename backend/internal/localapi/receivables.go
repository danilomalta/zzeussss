package localapi

import (
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"mime"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/receivables"
)

func (s *Server) mountReceivables(r fiber.Router) {
	r.Get("/finance/receivables", s.listReceivables)
	r.Post("/finance/receivables", s.createReceivable)
	r.Get("/finance/receivables/operations/:id", s.receivableOperation)
	r.Post("/finance/receivables/operations/:id/reconcile", s.reconcileReceivable)
	r.Get("/finance/receivables/customer-orders/:id", s.receivableCustomerOrder)
	r.Get("/finance/receivables/:id", s.readReceivable)
	r.Post("/finance/receivables/:id/receive-cash", s.receiveReceivableCash)
	r.Post("/finance/receivables/:id/cancel", s.cancelReceivable)
}
func receivableError(c *fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, localauth.ErrDenied):
		return c.SendStatus(401)
	case errors.Is(e, receivables.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(e, receivables.ErrNotFound):
		return c.SendStatus(404)
	case errors.Is(e, receivables.ErrConflict):
		return c.SendStatus(409)
	default:
		return cashError(c, e)
	}
}
func (s *Server) listReceivables(c *fiber.Ctx) error {
	if len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	limit, offset := 20, 0
	valid := true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] {
			valid = false
		}
		seen[key] = true
		n, e := strconv.Atoi(string(v))
		if e != nil {
			valid = false
		}
		switch key {
		case "limit":
			limit = n
		case "offset":
			offset = n
		default:
			valid = false
		}
	})
	if !valid {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := receivables.List(c.UserContext(), s.DB, ss.Actor, ss.Device, limit, offset)
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (s *Server) readReceivable(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := receivables.Read(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (s *Server) receivableOperation(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := receivables.Operation(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if e != nil {
		return receivableError(c, e)
	}
	return receivableRecoveryJSON(c, out)
}
func receivableRecoveryJSON(c *fiber.Ctx, out receivables.Result) error {
	c.Set("Cache-Control", "no-store")
	if out.Resolution != "" {
		return c.JSON(fiber.Map{"operation_id": out.OperationID, "document_id": out.DocumentID, "action": out.Action, "repeated": out.Repeated, "resolution": out.Resolution})
	}
	return c.JSON(out)
}
func receivableBody(c *fiber.Ctx, fields []string, target any, optional ...string) bool {
	if !noBagQuery(c) || len(c.Body()) > 32768 {
		return false
	}
	media, _, e := mime.ParseMediaType(c.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return false
	}
	if _, e = exactSaleObject(c.Body(), fields, optional...); e != nil {
		return false
	}
	return json.Unmarshal(c.Body(), target) == nil
}
func (s *Server) createReceivable(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in receivables.CreateInput
	if !receivableBody(c, []string{"operation_id", "id", "customer_reference", "document_reference", "description", "issued_on", "total_cents", "installments"}, &in, "customer_order") {
		return c.SendStatus(400)
	}
	if in.CustomerOrder != nil {
		var body map[string]json.RawMessage
		if json.Unmarshal(c.Body(), &body) != nil {
			return c.SendStatus(400)
		}
		if _, e := exactSaleObject(body["customer_order"], []string{"message_id", "terms_reference", "confirmed"}); e != nil {
			return c.SendStatus(400)
		}
	}
	var raw struct {
		Installments []json.RawMessage `json:"installments"`
	}
	if json.Unmarshal(c.Body(), &raw) != nil {
		return c.SendStatus(400)
	}
	for _, part := range raw.Installments {
		if _, e := exactSaleObject(part, []string{"id", "due_on", "amount_cents"}); e != nil {
			return c.SendStatus(400)
		}
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := receivables.Create(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, ss.Token, in)
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	code := 201
	if out.Repeated {
		code = 200
	}
	return c.Status(code).JSON(out)
}
func (s *Server) receiveReceivableCash(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in receivables.ReceiptInput
	if !receivableBody(c, []string{"operation_id", "installment_id", "session_id", "amount_cents", "expected_received_cents"}, &in) {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := receivables.ReceiveCash(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, ss.Token, c.Params("id"), in)
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (s *Server) cancelReceivable(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in receivables.CancelInput
	if !receivableBody(c, []string{"operation_id", "reason"}, &in) {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := receivables.Cancel(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, ss.Token, c.Params("id"), in)
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}

func (s *Server) reconcileReceivable(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in receivables.ReconcileInput
	if !receivableBody(c, []string{"document_id", "action", "confirmed"}, &in) {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := receivables.Reconcile(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, ss.Token, c.Params("id"), in)
	if e != nil {
		return receivableError(c, e)
	}
	return receivableRecoveryJSON(c, out)
}

func (s *Server) receivableCustomerOrder(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := receivables.CustomerOrder(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
