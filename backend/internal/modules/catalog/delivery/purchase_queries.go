package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func purchaseQuery(c *fiber.Ctx, summary bool) (onlinecatalog.PurchaseQuery, error) {
	q := onlinecatalog.PurchaseQuery{State: "all", Limit: 50}
	if len(c.Body()) != 0 {
		return q, onlinecatalog.ErrInput
	}
	allowed := []string{"q", "product_id", "supplier_id"}
	if !summary {
		allowed = append(allowed, "state", "limit", "offset")
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
			if a == key {
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
	if v, ok := values["state"]; ok {
		q.State = v
	}
	if v, ok := values["product_id"]; ok {
		id, e := onlinecatalog.ProductID(v)
		if e != nil {
			return q, e
		}
		q.ProductID = id
	}
	if v, ok := values["supplier_id"]; ok {
		if !onlinecatalog.ValidUUID(v) {
			return q, onlinecatalog.ErrInput
		}
		q.SupplierID = v
	}
	if !summary {
		l, o, e := apicontract.PageQuery(c, "q", "state", "product_id", "supplier_id")
		if e != nil {
			return q, onlinecatalog.ErrInput
		}
		q.Limit = l
		q.Offset = o
	}
	return q, onlinecatalog.ValidatePurchaseQuery(q)
}
func ResumoComprasInternas(c *fiber.Ctx) error {
	q, e := purchaseQuery(c, true)
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.PurchaseDraftSummary(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func ListarComprasInternas(export bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q, e := purchaseQuery(c, false)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.PurchaseDrafts(ctx, a, q)
		if e != nil {
			return managementError(c, e)
		}
		if !export {
			return c.JSON(r)
		}
		data, e := onlinecatalog.PurchaseDraftCSV(r)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="internal-purchases.csv"`)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Titan-Report-Total", strconv.FormatInt(r.Total, 10))
		c.Set("X-Titan-Report-Limit", strconv.Itoa(r.Limit))
		c.Set("X-Titan-Report-Offset", strconv.Itoa(r.Offset))
		c.Set("X-Titan-Report-Has-More", strconv.FormatBool(r.HasMore))
		if r.NextOffset != nil {
			c.Set("X-Titan-Report-Next-Offset", strconv.Itoa(*r.NextOffset))
		}
		c.Set("X-Titan-Source-Hash", r.SourceHash)
		c.Set("X-Titan-Report-Scope", "online-internal-purchases")
		c.Set("X-Titan-CSV-Text-Escape", "apostrophe")
		return c.SendString(data)
	}
}
func HistoricoCompraInterna(c *fiber.Ctx) error {
	id := c.Params("purchase_id")
	limit, offset, e := apicontract.PageQuery(c)
	if !onlinecatalog.ValidUUID(id) || e != nil || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.PurchaseDraftHistory(ctx, a, id, limit, offset)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func ConferirCompraInterna(c *fiber.Ctx) error {
	id := c.Params("purchase_id")
	if !onlinecatalog.ValidUUID(id) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.CheckPurchaseDraft(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
