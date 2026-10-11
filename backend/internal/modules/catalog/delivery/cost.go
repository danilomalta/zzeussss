package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func ConsultarCusto(c *fiber.Ctx) error  { return costRead(c, false) }
func ConsultarMargem(c *fiber.Ctx) error { return costRead(c, true) }
func costRead(c *fiber.Ctx, margin bool) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	if e != nil || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	if margin {
		r, e := s.ProductMargin(ctx, a, id)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
	r, e := s.ProductCost(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func AlterarCusto(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	if e != nil || !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	v, e := onlinecatalog.DecodeCost(c.Get("Content-Type"), c.Body())
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.SetCost(ctx, a, id, v)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func HistoricoCusto(c *fiber.Ctx) error {
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
	items, e := s.CostHistory(ctx, a, id, limit, offset)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(fiber.Map{"items": items, "limit": limit, "offset": offset})
}
func ConsultarOperacaoCusto(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.CostOperation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
