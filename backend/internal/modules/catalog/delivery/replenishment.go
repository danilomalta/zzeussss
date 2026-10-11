package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

// One allow-list for each route: detail accepts only review_days; summary has no paging/state.
func replenishmentQuery(c *fiber.Ctx, kind string) (onlinecatalog.ReplenishmentQuery, error) {
	q := onlinecatalog.ReplenishmentQuery{Active: "true", State: "needs_replenishment", ReviewDays: 7, Limit: 50}
	allowed := []string{"review_days"}
	if kind != "detail" {
		allowed = append(allowed, "q", "active")
	}
	if kind == "page" {
		allowed = append(allowed, "state", "limit", "offset")
	}
	values := map[string]string{}
	bad := len(c.Body()) != 0
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
		if !ok {
			bad = true
		}
		values[key] = string(v)
	})
	if bad {
		return q, onlinecatalog.ErrInput
	}
	q.Query = strings.TrimSpace(values["q"])
	if v, ok := values["active"]; ok {
		q.Active = v
	}
	if v, ok := values["state"]; ok {
		q.State = v
	}
	if kind == "summary" || kind == "detail" {
		q.State = "all"
	}
	if v, ok := values["review_days"]; ok {
		if len(v) < 1 || len(v) > 3 {
			return q, onlinecatalog.ErrInput
		}
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				return q, onlinecatalog.ErrInput
			}
		}
		n, e := strconv.ParseInt(v, 10, 64)
		if e != nil {
			return q, onlinecatalog.ErrInput
		}
		q.ReviewDays = n
	}
	if kind == "page" {
		var e error
		q.Limit, q.Offset, e = apicontract.PageQuery(c, "review_days", "q", "active", "state")
		if e != nil {
			return q, onlinecatalog.ErrInput
		}
	}
	if onlinecatalog.ValidateReplenishmentQuery(q) != nil {
		return q, onlinecatalog.ErrInput
	}
	return q, nil
}
func ResumoReposicaoOnline(c *fiber.Ctx) error {
	q, e := replenishmentQuery(c, "summary")
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.ReplenishmentSummary(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func ListarReposicaoOnline(csvReport bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q, e := replenishmentQuery(c, "page")
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.Replenishments(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		if !csvReport {
			return c.JSON(r)
		}
		data, e := onlinecatalog.ReplenishmentCSV(r)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="online-replenishment.csv"`)
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Titan-Report-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Titan-Report-Count", strconv.Itoa(len(r.Items)))
		c.Set("X-Titan-Report-Has-More", strconv.FormatBool(r.HasMore))
		if r.NextOffset != nil {
			c.Set("X-Titan-Report-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		c.Set("X-Titan-Source-Hash", r.SourceHash)
		c.Set("X-Titan-Report-Scope", "online-replenishment-planning")
		c.Set("X-Titan-CSV-Text-Escape", "apostrophe")
		return c.SendString(data)
	}
}
func ConsultarReposicaoOnline(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	q, qe := replenishmentQuery(c, "detail")
	if e != nil || qe != nil {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.ProductReplenishment(ctx, a, id, q.ReviewDays)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func SimularReposicaoOnline(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	if e != nil || !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	v, e := onlinecatalog.DecodeReplenishmentSimulation(c.Get("Content-Type"), c.Body())
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.SimulateReplenishment(ctx, a, id, v)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
