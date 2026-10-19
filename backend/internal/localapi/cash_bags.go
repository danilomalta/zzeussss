package localapi

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/cashcustody"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountCashBags(r fiber.Router) {
	r.Post("/cash/bags", s.createCashBag)
	r.Get("/cash/bags", s.listCashBags)
	r.Get("/cash/bags/operations/:id", s.cashBagReceipt)
	r.Get("/cash/bags/:id", s.readCashBag)
	for _, action := range []string{"dispatch", "cancel", "receive", "reject"} {
		action := action
		r.Post("/cash/bags/:id/"+action, func(c *fiber.Ctx) error { return s.transitionCashBag(c, action) })
	}
	r.Get("/cash/sessions/:id/custody", s.cashCustodyCoverage)
}
func bagError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, cashcustody.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(err, cashcustody.ErrNotFound):
		return c.SendStatus(404)
	case errors.Is(err, cashcustody.ErrConflict), errors.Is(err, cashcustody.ErrInconsistent), errors.Is(err, cashcustody.ErrOpen):
		return c.SendStatus(409)
	case errors.Is(err, cashcustody.ErrRange):
		return c.SendStatus(422)
	default:
		return catalogError(c, err)
	}
}
func noBagQuery(c *fiber.Ctx) bool {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Context().QueryArgs().Len() == 0
}
func (s *Server) createCashBag(c *fiber.Ctx) error {
	if !noBagQuery(c) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in cashcustody.CreateInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "bag_id", "withdrawal_operation_id", "seal_id"}) != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashcustody.Create(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return bagError(c, err)
	}
	code := 201
	if v.Repeated {
		code = 200
	}
	return c.Status(code).JSON(v)
}
func (s *Server) transitionCashBag(c *fiber.Ctx, action string) error {
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
	if action == "receive" {
		fields = append(fields, "seal_id", "counted_cents")
	}
	if action == "cancel" || action == "reject" {
		fields = append(fields, "reason")
	}
	var in cashcustody.ActionInput
	if decodeCash(c.Body(), &in, fields) != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashcustody.Transition(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, c.Params("id"), action, in)
	if err != nil {
		return bagError(c, err)
	}
	return c.JSON(v)
}
func (s *Server) readCashBag(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashcustody.Read(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return bagError(c, err)
	}
	return c.JSON(v)
}
func (s *Server) cashBagReceipt(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashcustody.Receipt(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return bagError(c, err)
	}
	return c.JSON(v)
}
func (s *Server) listCashBags(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	in := cashcustody.ListInput{Limit: 50}
	valid := true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || len(v) == 0 {
			valid = false
			return
		}
		seen[key] = true
		switch key {
		case "state":
			in.State = string(v)
		case "session_id":
			in.SessionID = string(v)
		case "limit", "offset":
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
			if key == "limit" {
				in.Limit = n
			} else {
				in.Offset = n
			}
		default:
			valid = false
		}
	})
	if !valid {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashcustody.List(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if err != nil {
		return bagError(c, err)
	}
	return c.JSON(v)
}
func (s *Server) cashCustodyCoverage(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := cashcustody.Reconcile(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return bagError(c, err)
	}
	return c.JSON(v)
}
