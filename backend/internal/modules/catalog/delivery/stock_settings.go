package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func ConsultarPlanejamentoEstoque(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	if e != nil || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	v, e := s.ProductStockSettings(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(v)
}
func AlterarPlanejamentoEstoque(preview bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, e := onlinecatalog.ProductID(c.Params("id"))
		if e != nil || !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		v, e := onlinecatalog.DecodeStockSettings(c.Get("Content-Type"), c.Body())
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		if preview {
			r, e := s.PreviewStockSettings(ctx, a, id, v)
			if e != nil {
				return managementError(c, e)
			}
			return c.JSON(r)
		}
		r, e := s.SetStockSettings(ctx, a, id, v)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
}
func ConsultarOperacaoPlanejamento(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.StockSettingsOperation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func HistoricoPlanejamentoEstoque(c *fiber.Ctx) error {
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
	r, e := s.StockSettingsHistory(ctx, a, id, limit, offset)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
