package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/cashaccounting"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/register"
)

func (s *Server) mountCashAccounting(r fiber.Router) {
	r.Get("/accounting/cash/sessions", s.accountingCashSessions)
	r.Get("/accounting/cash/journal", s.accountingCashJournal)
	r.Get("/accounting/cash/report", s.accountingCashReport)
	r.Get("/accounting/cash/report.csv", s.accountingCashCSV)
}
func accountingCashError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, cashaccounting.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(err, cashaccounting.ErrInconsistent), errors.Is(err, cashaccounting.ErrText), errors.Is(err, register.ErrReportInconsistent), errors.Is(err, register.ErrReportOpen), errors.Is(err, cashjournal.ErrInconsistent), errors.Is(err, cashjournal.ErrOpen):
		return c.SendStatus(409)
	case errors.Is(err, cashaccounting.ErrRange), errors.Is(err, register.ErrReportRange), errors.Is(err, cashjournal.ErrRange), errors.Is(err, cashjournal.ErrExportLimit):
		return c.SendStatus(422)
	case errors.Is(err, register.ErrReportNotFound), errors.Is(err, cashjournal.ErrNotFound):
		return c.SendStatus(404)
	default:
		return cashError(c, err)
	}
}
func accountingWindow(c *fiber.Ctx, paged bool) (cashaccounting.Window, int, int, bool) {
	c.Set(fiber.HeaderCacheControl, "no-store")
	w := cashaccounting.Window{}
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
		switch name {
		case "from":
			w.From = string(v)
		case "to":
			w.To = string(v)
		case "limit", "offset":
			if !paged {
				ok = false
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
		default:
			ok = false
		}
	})
	if !seen["from"] || !seen["to"] {
		ok = false
	}
	return w, limit, offset, ok
}
func (s *Server) accountingCashSessions(c *fiber.Ctx) error {
	w, limit, offset, ok := accountingWindow(c, true)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashaccounting.Sessions(c.UserContext(), s.DB, ss.Actor, ss.Device, w, limit, offset)
	if err != nil {
		return accountingCashError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) accountingCashJournal(c *fiber.Ctx) error {
	w, limit, offset, ok := accountingWindow(c, true)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashaccounting.Postings(c.UserContext(), s.DB, ss.Actor, ss.Device, w, limit, offset)
	if err != nil {
		return accountingCashError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) accountingCashReport(c *fiber.Ctx) error {
	w, _, _, ok := accountingWindow(c, false)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, err := cashaccounting.Read(c.UserContext(), s.DB, ss.Actor, ss.Device, w)
	if err != nil {
		return accountingCashError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) accountingCashCSV(c *fiber.Ctx) error {
	c.Set("X-Content-Type-Options", "nosniff")
	w, _, _, ok := accountingWindow(c, false)
	if !ok {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	data, err := cashaccounting.ExportCSV(c.UserContext(), s.DB, ss.Actor, ss.Device, w)
	if err != nil {
		return accountingCashError(c, err)
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="titan-caixas-encerrados.csv"`)
	return c.Send(data)
}
