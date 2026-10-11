package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func stockReportQuery(c *fiber.Ctx, issue string, summary, csvReport bool) (onlinecatalog.StockReportQuery, error) {
	q := onlinecatalog.StockReportQuery{Active: "true", Issue: issue, HorizonDays: 7, Limit: 50}
	allowed := []string{"q", "active", "horizon_days"}
	if csvReport {
		allowed = append(allowed, "issue")
	}
	values := map[string]string{}
	bad := false
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if _, ok := values[key]; ok {
			bad = true
		}
		ok := false
		for _, a := range allowed {
			if key == a {
				ok = true
			}
		}
		if !summary && (key == "limit" || key == "offset") {
			ok = true
		}
		if !ok {
			bad = true
		}
		values[key] = string(v)
	})
	if bad || len(c.Body()) != 0 {
		return q, onlinecatalog.ErrInput
	}
	var e error
	if !summary {
		q.Limit, q.Offset, e = apicontract.PageQuery(c, allowed...)
		if e != nil {
			return q, onlinecatalog.ErrInput
		}
	}
	q.Query = strings.TrimSpace(values["q"])
	if v, ok := values["active"]; ok {
		q.Active = v
	}
	if csvReport {
		if v, ok := values["issue"]; ok {
			q.Issue = v
		}
	}
	if v, ok := values["horizon_days"]; ok {
		if len(v) < 1 || len(v) > 3 {
			return q, onlinecatalog.ErrInput
		}
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				return q, onlinecatalog.ErrInput
			}
		}
		q.HorizonDays, e = strconv.ParseInt(v, 10, 64)
	}
	if e != nil || onlinecatalog.ValidateStockReport(q) != nil {
		return q, onlinecatalog.ErrInput
	}
	return q, nil
}
func ResumoEstoqueOnline(c *fiber.Ctx) error {
	q, e := stockReportQuery(c, "all", true, false)
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.StockReportSummary(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func RelatorioEstoqueOnline(issue string, csvReport bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q, e := stockReportQuery(c, issue, false, csvReport)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.StockReport(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		if !csvReport {
			return c.JSON(r)
		}
		report, e := onlinecatalog.StockReportCSV(r)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="online-stock-analysis.csv"`)
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Titan-Report-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Titan-Report-Count", strconv.Itoa(len(r.Items)))
		c.Set("X-Titan-Report-Has-More", strconv.FormatBool(r.HasMore))
		if r.NextOffset != nil {
			c.Set("X-Titan-Report-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		c.Set("X-Titan-Source-Hash", r.SourceHash)
		c.Set("X-Titan-Report-Scope", "online-stock-analysis")
		c.Set("X-Titan-CSV-Text-Escape", "apostrophe")
		return c.SendString(report)
	}
}
