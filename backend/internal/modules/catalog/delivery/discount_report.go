package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

// Fixed status routes cannot be overridden with query parameters.
func DiscountReport(status string, csvReport bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		extra := []string{"q", "active"}
		if csvReport {
			extra = append(extra, "status")
		}
		limit, offset, e := apicontract.PageQuery(c, extra...)
		q := onlinecatalog.DiscountReportQuery{Query: strings.TrimSpace(c.Query("q")), Active: c.Query("active", "all"), Status: status, Limit: limit, Offset: offset}
		if csvReport {
			q.Status = c.Query("status", "all")
		}
		if e != nil || len(c.Body()) != 0 || onlinecatalog.ValidateDiscountReport(q) != nil {
			return managementError(c, onlinecatalog.ErrInput)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.DiscountReport(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		if !csvReport {
			return c.JSON(r)
		}
		report, e := onlinecatalog.DiscountReportCSV(r)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="discount-suggestions.csv"`)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Titan-Report-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Titan-Report-Count", strconv.Itoa(len(r.Items)))
		c.Set("X-Titan-Report-Has-More", strconv.FormatBool(r.HasMore))
		if r.NextOffset != nil {
			c.Set("X-Titan-Report-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		c.Set("X-Titan-Source-Hash", r.SourceHash)
		c.Set("X-Titan-Report-Scope", "online-discount-suggestions")
		c.Set("X-Titan-CSV-Text-Escape", "apostrophe")
		return c.SendString(report)
	}
}
func DiscountReportSummary(c *fiber.Ctx) error {
	bad := false
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || (key != "q" && key != "active") {
			bad = true
		}
		seen[key] = true
	})
	q := onlinecatalog.DiscountReportQuery{Query: strings.TrimSpace(c.Query("q")), Active: c.Query("active", "all"), Status: "all", Limit: 50}
	if bad || len(c.Body()) != 0 || onlinecatalog.ValidateDiscountReport(q) != nil {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.DiscountReportSummary(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
