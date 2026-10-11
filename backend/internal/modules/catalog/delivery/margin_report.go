package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func marginQuery(c *fiber.Ctx, issue string, summary, csvReport bool) (onlinecatalog.MarginQuery, error) {
	limit, offset := 50, 0
	var e error
	values := map[string]string{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) { values[string(k)] = string(v) })
	value := func(key, fallback string) string {
		if v, present := values[key]; present {
			return v
		}
		return fallback
	}
	if summary {
		seen := map[string]bool{}
		bad := false
		c.Context().QueryArgs().VisitAll(func(k, v []byte) {
			key := string(k)
			if seen[key] || (key != "q" && key != "active" && key != "target_bps") {
				bad = true
			}
			seen[key] = true
		})
		if bad {
			return onlinecatalog.MarginQuery{}, onlinecatalog.ErrInput
		}
	} else {
		extra := []string{"q", "active", "target_bps"}
		if csvReport {
			extra = append(extra, "issue")
		}
		limit, offset, e = apicontract.PageQuery(c, extra...)
	}
	raw := value("target_bps", "3000")
	if len(raw) == 0 || len(raw) > 5 {
		return onlinecatalog.MarginQuery{}, onlinecatalog.ErrInput
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return onlinecatalog.MarginQuery{}, onlinecatalog.ErrInput
		}
	}
	target, pe := strconv.ParseInt(raw, 10, 64)
	q := onlinecatalog.MarginQuery{Query: strings.TrimSpace(value("q", "")), Active: value("active", "all"), Issue: issue, TargetBPS: target, Limit: limit, Offset: offset}
	if csvReport {
		q.Issue = value("issue", "all")
	}
	if e != nil || pe != nil || len(c.Body()) != 0 || onlinecatalog.ValidateMarginQuery(q) != nil {
		return q, onlinecatalog.ErrInput
	}
	return q, nil
}
func ResumoMargens(c *fiber.Ctx) error {
	q, e := marginQuery(c, "all", true, false)
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.MarginSummary(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func RelatorioMargens(issue string, csvReport bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q, e := marginQuery(c, issue, false, csvReport)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.Margins(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		if !csvReport {
			return c.JSON(r)
		}
		report, e := onlinecatalog.MarginsCSV(r)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="product-margins.csv"`)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Titan-Report-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Titan-Report-Count", strconv.Itoa(len(r.Items)))
		c.Set("X-Titan-Report-Has-More", strconv.FormatBool(r.HasMore))
		if r.NextOffset != nil {
			c.Set("X-Titan-Report-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		c.Set("X-Titan-Source-Hash", r.SourceHash)
		c.Set("X-Titan-Report-Scope", "online-product-margins")
		c.Set("X-Titan-CSV-Text-Escape", "apostrophe")
		return c.SendString(report)
	}
}
