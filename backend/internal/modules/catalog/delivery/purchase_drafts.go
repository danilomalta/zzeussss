package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/onlinecatalog"
)

func CriarCompraInterna(c *fiber.Ctx) error {
	if !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	v, e := onlinecatalog.DecodePurchaseDraft(c.Get("Content-Type"), c.Body())
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.CreatePurchaseDraft(ctx, a, v)
	if e != nil {
		return managementError(c, e)
	}
	return c.Status(fiber.StatusCreated).JSON(r)
}
func ConsultarCompraInterna(c *fiber.Ctx) error {
	id := c.Params("purchase_id")
	if !onlinecatalog.ValidUUID(id) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.PurchaseDraft(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func DecidirCompraInterna(kind string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("purchase_id")
		if !onlinecatalog.ValidUUID(id) || !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		v, e := onlinecatalog.DecodeReplenishmentDecision(c.Get("Content-Type"), c.Body())
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.DecidePurchaseDraft(ctx, a, id, kind, v)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
}
func OperacaoCompraInterna(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.PurchaseOperation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
