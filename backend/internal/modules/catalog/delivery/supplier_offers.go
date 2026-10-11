package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func GravarCondicaoFornecedor(update bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("offer_id")
		if !noManagementQuery(c) || (update && !onlinecatalog.ValidUUID(id)) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		v, e := onlinecatalog.DecodeSupplierOffer(c.Get("Content-Type"), c.Body(), update)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		var receipt onlinecatalog.SupplierOfferReceipt
		if update {
			receipt, e = s.UpdateSupplierOffer(ctx, a, id, v)
		} else {
			receipt, e = s.CreateSupplierOffer(ctx, a, v)
		}
		if e != nil {
			return managementError(c, e)
		}
		if !update {
			return c.Status(fiber.StatusCreated).JSON(receipt)
		}
		return c.JSON(receipt)
	}
}
func ConsultarCondicaoFornecedor(c *fiber.Ctx) error {
	id := c.Params("offer_id")
	if !onlinecatalog.ValidUUID(id) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.SupplierOffer(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func OperacaoCondicaoFornecedor(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.SupplierOfferOperation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func supplierOfferQuery(c *fiber.Ctx) (onlinecatalog.SupplierOfferQuery, error) {
	q := onlinecatalog.SupplierOfferQuery{Active: "all", Limit: 50}
	if len(c.Body()) != 0 {
		return q, onlinecatalog.ErrInput
	}
	l, o, e := apicontract.PageQuery(c, "supplier_id", "product_id", "active")
	if e != nil {
		return q, onlinecatalog.ErrInput
	}
	q.Limit = l
	q.Offset = o
	values := map[string]string{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) { values[string(k)] = string(v) })
	if v, ok := values["active"]; ok {
		q.Active = v
	}
	if v, ok := values["supplier_id"]; ok {
		if !onlinecatalog.ValidUUID(v) {
			return q, onlinecatalog.ErrInput
		}
		q.SupplierID = v
	}
	if v, ok := values["product_id"]; ok {
		id, e := onlinecatalog.ProductID(v)
		if e != nil {
			return q, e
		}
		q.ProductID = id
	}
	return q, onlinecatalog.ValidateSupplierOfferQuery(q)
}
func ListarCondicoesFornecedores(c *fiber.Ctx) error {
	q, e := supplierOfferQuery(c)
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.SupplierOffers(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
