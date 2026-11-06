package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/fiscal"
)

func (s *Server) fiscalAccountingExport(c *fiber.Ctx) error {
	if !fiscalRead(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	seen := map[string]bool{}
	valid := true
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		name := string(k)
		if seen[name] || (name != "from" && name != "until" && name != "lang") {
			valid = false
		}
		seen[name] = true
	})
	if !valid || len(seen) != 3 {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	zip, err := s.fiscalProviders.AccountingExport(c.UserContext(), s.productionContext(c), c.Query("from"), c.Query("until"), c.Query("lang"))
	if err != nil {
		return fiscalFailure(c, err)
	}
	if err = s.recordReportExport(c, "fiscal_homologation_period", c.Query("from"), c.Query("until"), zip); err != nil {
		return fiscalFailure(c, err)
	}
	c.Set("Cache-Control", "no-store")
	c.Set("Content-Disposition", `attachment; filename="fiscal-homologacao.zip"`)
	c.Type("zip")
	return c.Send(zip)
}
