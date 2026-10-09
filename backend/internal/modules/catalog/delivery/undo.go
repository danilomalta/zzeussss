package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/onlinecatalog"
)

func DesfazerLoteCatalogo(apply bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		u, e := onlinecatalog.DecodeUndo(c.Get("Content-Type"), c.Body(), apply)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		if apply {
			r, e := s.ApplyUndo(ctx, a, u)
			if e != nil {
				return managementError(c, e)
			}
			return c.JSON(r)
		}
		p, e := s.PreviewUndo(ctx, a, u)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(p)
	}
}
func ConsultarReversaoCatalogo(status bool) fiber.Handler {
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
		var r onlinecatalog.UndoReceipt
		if status {
			r, e = s.UndoStatus(ctx, a, op)
		} else {
			r, e = s.Undo(ctx, a, op)
		}
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
}
