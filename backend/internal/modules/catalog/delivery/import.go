package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/onlinecatalog"
)

func ImportarCatalogoCSV(apply bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		u, e := onlinecatalog.DecodeImport(c.Get("Content-Type"), c.Body(), apply)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		if apply {
			r, e := s.ApplyImport(ctx, a, u)
			if e != nil {
				return managementError(c, e)
			}
			return c.JSON(r)
		}
		p, e := s.PreviewImport(ctx, a, u)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(p)
	}
}
func ConsultarImportacaoCatalogo(administrative bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		op := c.Params("operation_id")
		if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.Import(ctx, a, op, administrative)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
}
