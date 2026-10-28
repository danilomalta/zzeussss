package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) readFiscalDANFE(c *fiber.Ctx) error {
	if !fiscalRead(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	opts := fiscalcore.PrintOptions{}
	valid := true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key, value := string(k), string(v)
		if seen[key] || value == "" {
			valid = false
			return
		}
		seen[key] = true
		switch key {
		case "width_mm":
			switch value {
			case "58":
				opts.WidthMM = 58
			case "80":
				opts.WidthMM = 80
			default:
				valid = false
			}
		case "timezone":
			opts.Timezone = value
		default:
			valid = false
		}
	})
	if !valid || !seen["width_mm"] || !seen["timezone"] {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	ss := c.Locals("session").(localauth.Session)
	out, err := fiscal.ReadHomologationDANFE(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Params("id"), ss.Token, opts)
	if err != nil {
		return fiscalFailure(c, err)
	}
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `inline; filename="DANFE-HOMOLOGACAO-`+out.Key+`.html"`)
	c.Set("Content-Security-Policy", "default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	c.Set("Referrer-Policy", "no-referrer")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-Titan-Fiscal-Scope", "homologation")
	c.Set("X-Titan-Source-SHA256", out.SourceSHA256)
	c.Set("X-Titan-Artifact-SHA256", out.HTMLSHA256)
	return c.Send(out.HTML)
}
