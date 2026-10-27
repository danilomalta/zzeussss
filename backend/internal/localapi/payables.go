package localapi

import (
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/payables"
)

func (s *Server) mountPayables(r fiber.Router) {
	r.Post("/finance/payables", s.createPayable)
	r.Get("/finance/payables", s.listPayables)
	r.Get("/finance/payables/report", s.payablesReport)
	r.Get("/finance/payables/report.csv", s.payablesCSV)
	r.Get("/finance/payables/operations/:id", s.payableReceipt)
	r.Get("/finance/payables/:id", s.readPayable)
	for _, action := range []string{"approve", "reject", "cancel"} {
		action := action
		r.Post("/finance/payables/:id/"+action, func(c *fiber.Ctx) error { return s.changePayable(c, action) })
	}
}
func payableError(c *fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, payables.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(e, payables.ErrNotFound):
		return c.SendStatus(404)
	case errors.Is(e, payables.ErrConflict), errors.Is(e, payables.ErrInconsistent), errors.Is(e, payables.ErrText):
		return c.SendStatus(409)
	case errors.Is(e, payables.ErrRange):
		return c.SendStatus(422)
	default:
		return cashError(c, e)
	}
}
func (s *Server) createPayable(c *fiber.Ctx) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 32768 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in payables.CreateInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "payable_id", "supplier_id", "document_reference", "description", "issued_on", "currency", "total_cents", "installments"}) != nil {
		return c.SendStatus(400)
	}
	var nested struct {
		Installments []json.RawMessage `json:"installments"`
	}
	if json.Unmarshal(c.Body(), &nested) != nil {
		return c.SendStatus(400)
	}
	for _, v := range nested.Installments {
		var p payables.Installment
		if decodeCash(v, &p, []string{"id", "due_on", "amount_cents"}) != nil {
			return c.SendStatus(400)
		}
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.Create(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in)
	if e != nil {
		return payableError(c, e)
	}
	code := 201
	if r.Repeated {
		code = 200
	}
	return c.Status(code).JSON(r)
}
func (s *Server) changePayable(c *fiber.Ctx, action string) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in payables.ActionInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "reason"}) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.Transition(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, c.Params("id"), action, in)
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) readPayable(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.Get(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) payableReceipt(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.Receipt(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
func payableQuery(c *fiber.Ctx, report bool) (payables.Window, string, int, int, bool) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	w := payables.Window{}
	state := ""
	limit, offset := 50, 0
	ok := len(c.Body()) == 0
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		name := string(k)
		if seen[name] || len(v) == 0 {
			ok = false
			return
		}
		seen[name] = true
		if report {
			switch name {
			case "from":
				w.From = string(v)
			case "to":
				w.To = string(v)
			case "as_of":
				w.AsOf = string(v)
			default:
				ok = false
			}
			return
		}
		switch name {
		case "state":
			state = string(v)
		case "limit", "offset":
			for _, b := range v {
				if b < '0' || b > '9' {
					ok = false
					return
				}
			}
			n, e := strconv.Atoi(string(v))
			if e != nil {
				ok = false
				return
			}
			if name == "limit" {
				limit = n
			} else {
				offset = n
			}
		default:
			ok = false
		}
	})
	if report && (!seen["from"] || !seen["to"] || !seen["as_of"]) {
		ok = false
	}
	return w, state, limit, offset, ok
}
func (s *Server) listPayables(c *fiber.Ctx) error {
	_, state, limit, offset, ok := payableQuery(c, false)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.List(c.UserContext(), s.DB, ss.Actor, ss.Device, state, limit, offset)
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) payablesReport(c *fiber.Ctx) error {
	w, _, _, _, ok := payableQuery(c, true)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.ReadReport(c.UserContext(), s.DB, ss.Actor, ss.Device, w)
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) payablesCSV(c *fiber.Ctx) error {
	c.Set("X-Content-Type-Options", "nosniff")
	w, _, _, _, ok := payableQuery(c, true)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	data, e := payables.ExportCSV(c.UserContext(), s.DB, ss.Actor, ss.Device, w)
	if e != nil {
		return payableError(c, e)
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="titan-contas-a-pagar.csv"`)
	return c.Send(data)
}
