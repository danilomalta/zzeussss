package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func ConsultarSugestaoDesconto(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	if e != nil || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.DiscountSuggestion(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func RevisarSugestaoDesconto(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, e := onlinecatalog.ProductID(c.Params("id"))
		if e != nil || !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		v, e := onlinecatalog.DecodeDiscountReview(c.Get("Content-Type"), c.Body())
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.ReviewDiscount(ctx, a, id, action, v)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
}
func ConsultarDecisaoDesconto(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.DiscountReview(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func HistoricoDecisaoDesconto(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	limit, offset, pe := apicontract.PageQuery(c)
	if e != nil || pe != nil || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	items, e := s.DiscountReviewHistory(ctx, a, id, limit, offset)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(fiber.Map{"items": items, "limit": limit, "offset": offset})
}
