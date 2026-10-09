package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func ReajustarCatalogo(apply bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		u, e := onlinecatalog.DecodeAdjustment(c.Get("Content-Type"), c.Body(), apply)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		if apply {
			r, e := s.ApplyAdjustment(ctx, a, u)
			if e != nil {
				return managementError(c, e)
			}
			return c.JSON(r)
		}
		p, e := s.PreviewAdjustment(ctx, a, u)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(p)
	}
}
func ConsultarReajusteCatalogo(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.Adjustment(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func HistoricoReajustesCatalogo(c *fiber.Ctx) error {
	limit, offset, e := apicontract.PageQuery(c)
	if !c.Context().QueryArgs().Has("limit") {
		limit = onlinecatalog.MaxBatchHistoryPage
	}
	if e != nil || limit > onlinecatalog.MaxBatchHistoryPage {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	items, e := s.AdjustmentHistory(ctx, a, limit, offset)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(fiber.Map{"items": items, "limit": limit, "offset": offset})
}
