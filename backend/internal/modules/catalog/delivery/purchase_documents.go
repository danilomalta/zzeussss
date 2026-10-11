package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/onlinecatalog"
)

func PrepararDocumentoCompra(c *fiber.Ctx) error {
	if !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	v, e := onlinecatalog.DecodePurchaseDocument(c.Get("Content-Type"), c.Body())
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.PreparePurchaseDocument(ctx, a, v)
	if e != nil {
		return managementError(c, e)
	}
	return c.Status(201).JSON(r)
}
func ConsultarDocumentoCompra(download bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("document_id")
		if !onlinecatalog.ValidUUID(id) || !noManagementQuery(c) || len(c.Body()) != 0 {
			return managementError(c, onlinecatalog.ErrInput)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.PurchaseDocument(ctx, a, id)
		if e != nil {
			return managementError(c, e)
		}
		if !download {
			return c.JSON(r)
		}
		data, e := onlinecatalog.PurchaseDocumentJSON(r)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Content-Type", "application/json; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="internal-purchase-`+id+`.json"`)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Cache-Control", "no-store")
		c.Set("X-Titan-Document-SHA256", r.SHA256)
		c.Set("X-Titan-Document-Scope", "internal-purchase")
		return c.SendString(data)
	}
}
func CancelarDocumentoCompra(c *fiber.Ctx) error {
	id := c.Params("document_id")
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
	r, e := s.CancelPurchaseDocument(ctx, a, id, v)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func OperacaoDocumentoCompra(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.PurchaseDocumentOperation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
