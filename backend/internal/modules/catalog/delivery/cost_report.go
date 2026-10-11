package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func costReportQuery(c *fiber.Ctx, summary, product bool) (onlinecatalog.CostReportQuery, error) {
	q := onlinecatalog.CostReportQuery{Change: "all", Limit: 50}
	allowed := []string{"q", "actor_id", "change", "from", "until"}
	if !product {
		allowed = append(allowed, "product_id")
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
	if v, ok := values["change"]; ok {
		q.Change = v
	}
	q.ActorID = values["actor_id"]
	q.From = values["from"]
	q.Until = values["until"]
	for _, key := range []string{"actor_id", "from", "until", "product_id"} {
		if v, ok := values[key]; ok && v == "" {
			return q, onlinecatalog.ErrInput
		}
	}
	if product {
		q.ProductID, e = onlinecatalog.ProductID(c.Params("id"))
	} else if v, ok := values["product_id"]; ok {
		q.ProductID, e = onlinecatalog.ProductID(v)
	}
	if e != nil || onlinecatalog.ValidateCostReport(q) != nil {
		return q, onlinecatalog.ErrInput
	}
	return q, nil
}
func ResumoHistoricoCustos(c *fiber.Ctx) error {
	q, e := costReportQuery(c, true, false)
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.CostReportSummary(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func DetalheHistoricoCustos(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.CostReportDetail(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func RelatorioHistoricoCustos(product, csvReport bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q, e := costReportQuery(c, false, product)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.CostReport(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		if !csvReport {
			return c.JSON(r)
		}
		report, e := onlinecatalog.CostReportCSV(r)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="cost-history.csv"`)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Cache-Control", "no-store")
		c.Set("X-Titan-Report-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Titan-Report-Count", strconv.Itoa(len(r.Items)))
		c.Set("X-Titan-Report-Has-More", strconv.FormatBool(r.HasMore))
		if r.NextOffset != nil {
			c.Set("X-Titan-Report-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		c.Set("X-Titan-Source-Hash", r.SourceHash)
		c.Set("X-Titan-Report-Scope", "online-cost-history")
		c.Set("X-Titan-CSV-Text-Escape", "apostrophe")
		return c.SendString(report)
	}
}
