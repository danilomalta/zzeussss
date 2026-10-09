package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func adjustmentExportQuery(c *fiber.Ctx) (onlinecatalog.AdjustmentExportQuery, error) {
	limit, offset, e := apicontract.PageQuery(c, "actor_id", "product_id", "from", "until")
	if e != nil || len(c.Body()) != 0 {
		return onlinecatalog.AdjustmentExportQuery{}, onlinecatalog.ErrInput
	}
	if !c.Context().QueryArgs().Has("limit") {
		limit = 10
	}
	q := onlinecatalog.AdjustmentExportQuery{ActorID: c.Query("actor_id"), From: c.Query("from"), Until: c.Query("until"), Limit: limit, Offset: offset}
	for _, key := range []string{"actor_id", "product_id", "from", "until"} {
		if c.Context().QueryArgs().Has(key) && c.Query(key) == "" {
			return q, onlinecatalog.ErrInput
		}
	}
	if c.Context().QueryArgs().Has("product_id") {
		q.ProductID, e = onlinecatalog.ProductID(c.Query("product_id"))
		if e != nil {
			return q, e
		}
	}
	return q, onlinecatalog.ValidateAdjustmentExport(q)
}
func ExportarReajustes(rawCSV bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q, e := adjustmentExportQuery(c)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.ExportAdjustments(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		if !rawCSV {
			return c.JSON(r)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="titan-catalog-adjustment-history.csv"`)
		c.Set("X-Catalog-Source-Hash", r.SourceHash)
		c.Set("X-Catalog-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Catalog-Row-Count", strconv.Itoa(r.RowCount))
		c.Set("X-Catalog-Limit", strconv.Itoa(r.Limit))
		c.Set("X-Catalog-Offset", strconv.Itoa(r.Offset))
		c.Set("X-Catalog-Has-More", strconv.FormatBool(r.HasMore))
		c.Set("X-Catalog-Text-Prefix", r.TextPrefix)
		if r.NextOffset != nil {
			c.Set("X-Catalog-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		return c.SendString(r.CSV)
	}
}
