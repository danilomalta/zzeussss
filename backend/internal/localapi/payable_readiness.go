package localapi

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/payables"
)

func (s *Server) mountPayableReadiness(r fiber.Router) {
	r.Get("/finance/payables/:id/installment-balances", s.payableInstallmentBalances)
	r.Get("/finance/payables/:id/cash-sources", s.payablePaymentSources)
	r.Get("/finance/payable-cash/:id/return-sources", s.payableReturnSources)
}
func (s *Server) payableInstallmentBalances(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.Balances(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"))
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
func readinessQuery(c *fiber.Ctx, needPart bool) (string, int, int, bool) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	part := ""
	limit, offset := 50, 0
	ok := len(c.Body()) == 0
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || len(v) == 0 {
			ok = false
			return
		}
		seen[key] = true
		if key == "installment_id" && needPart {
			part = string(v)
			return
		}
		if key != "limit" && key != "offset" {
			ok = false
			return
		}
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
		if key == "limit" {
			limit = n
		} else {
			offset = n
		}
	})
	if needPart && !seen["installment_id"] {
		ok = false
	}
	return part, limit, offset, ok
}
func (s *Server) payablePaymentSources(c *fiber.Ctx) error {
	part, limit, offset, ok := readinessQuery(c, true)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.PaymentSources(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"), part, limit, offset)
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) payableReturnSources(c *fiber.Ctx) error {
	_, limit, offset, ok := readinessQuery(c, false)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.ReturnSources(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"), limit, offset)
	if e != nil {
		return payableError(c, e)
	}
	return c.JSON(r)
}
