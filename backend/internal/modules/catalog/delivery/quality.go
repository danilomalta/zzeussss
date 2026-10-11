package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

// Fixed issue routes cannot be overridden with query parameters.
func CatalogQuality(issue string, csvReport bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		extra := []string{"q", "active"}
		if csvReport {
			extra = append(extra, "issue")
		}
		limit, offset, e := apicontract.PageQuery(c, extra...)
		q := onlinecatalog.QualityQuery{Query: strings.TrimSpace(c.Query("q")), Active: c.Query("active", "active"), Issue: issue, Limit: limit, Offset: offset}
		if csvReport {
			q.Issue = c.Query("issue", "any")
		}
		if e != nil || len(c.Body()) != 0 || onlinecatalog.ValidateQuality(q) != nil {
			return managementError(c, onlinecatalog.ErrInput)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.Quality(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		if !csvReport {
			return c.JSON(r)
		}
		report, e := onlinecatalog.QualityCSV(r)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="catalog-pendencies.csv"`)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Titan-Report-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Titan-Report-Count", strconv.Itoa(len(r.Items)))
		c.Set("X-Titan-Report-Has-More", strconv.FormatBool(r.HasMore))
		if r.NextOffset != nil {
			c.Set("X-Titan-Report-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		c.Set("X-Titan-Source-Hash", r.SourceHash)
		c.Set("X-Titan-Report-Scope", "online-catalog")
		c.Set("X-Titan-CSV-Text-Escape", "apostrophe")
		return c.SendString(report)
	}
}
func CatalogQualitySummary(c *fiber.Ctx) error {
	bad := false
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || (key != "q" && key != "active") {
			bad = true
		}
		seen[key] = true
	})
	q := onlinecatalog.QualityQuery{Query: strings.TrimSpace(c.Query("q")), Active: c.Query("active", "active"), Issue: "any", Limit: 50}
	if bad || len(c.Body()) != 0 || onlinecatalog.ValidateQuality(q) != nil {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.QualitySummary(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
