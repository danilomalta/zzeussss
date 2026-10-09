package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func LoteCatalogo(apply bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		b, e := onlinecatalog.DecodeBatch(c.Get("Content-Type"), c.Body(), apply)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		if apply {
			r, e := s.ApplyBatch(ctx, a, b)
			if e != nil {
				return managementError(c, e)
			}
			return c.JSON(r)
		}
		p, e := s.PreviewBatch(ctx, a, b)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(p)
	}
}
func ConsultarLoteCatalogo(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.Batch(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func HistoricoLotesCatalogo(c *fiber.Ctx) error {
	limit, offset, e := apicontract.PageQuery(c)
	if e != nil {
		return managementError(c, onlinecatalog.ErrInput)
	}
	if c.Query("limit") == "" {
		limit = onlinecatalog.MaxBatchHistoryPage
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	items, e := s.BatchHistory(ctx, a, limit, offset)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(fiber.Map{"items": items, "limit": limit, "offset": offset})
}
