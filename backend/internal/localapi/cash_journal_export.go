package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) exportCashJournal(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if c.Context().QueryArgs().Len() != 0 || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	data, err := cashjournal.ExportCSV(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return cashJournalError(c, err)
	}
	// Fixed filename: no user-controlled value is interpolated into headers.
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="titan-caixa-lancamentos.csv"`)
	return c.Send(data)
}
