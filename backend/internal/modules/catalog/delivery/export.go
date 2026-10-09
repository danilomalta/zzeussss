package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func exportQuery(c *fiber.Ctx) (onlinecatalog.ExportQuery, error) {
	limit, offset, e := apicontract.PageQuery(c, "action", "q", "ativo")
	if e != nil || len(c.Body()) != 0 {
		return onlinecatalog.ExportQuery{}, onlinecatalog.ErrInput
	}
	q := onlinecatalog.ExportQuery{Action: c.Query("action", "price"), Query: c.Query("q"), Active: c.Query("ativo", "all"), Limit: limit, Offset: offset}
	// Defaults apply only when the parameter is absent, not when supplied empty.
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		switch string(k) {
		case "action":
			q.Action = string(v)
		case "ativo":
			q.Active = string(v)
		}
	})
	return q, onlinecatalog.ValidateExport(q)
}
func ExportarCatalogo(rawCSV bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q, e := exportQuery(c)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.Export(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		if !rawCSV {
			return c.JSON(r)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="titan-catalog-`+q.Action+`.csv"`)
		c.Set("X-Catalog-Source-Hash", r.SourceHash)
		c.Set("X-Catalog-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Catalog-Row-Count", strconv.Itoa(len(r.Items)))
		c.Set("X-Catalog-Limit", strconv.Itoa(r.Limit))
		c.Set("X-Catalog-Offset", strconv.Itoa(r.Offset))
		c.Set("X-Catalog-Has-More", strconv.FormatBool(r.HasMore))
		if r.NextOffset != nil {
			c.Set("X-Catalog-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		return c.SendString(r.CSV)
	}
}
