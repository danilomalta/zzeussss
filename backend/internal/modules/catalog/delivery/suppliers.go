package delivery

import (
	"github.com/gofiber/fiber/v2"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func GravarFornecedor(update bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("supplier_id")
		if !noManagementQuery(c) || (update && !onlinecatalog.ValidUUID(id)) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		v, e := onlinecatalog.DecodeSupplier(c.Get("Content-Type"), c.Body(), update)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		var receipt onlinecatalog.SupplierReceipt
		if update {
			receipt, e = s.UpdateSupplier(ctx, a, id, v)
		} else {
			receipt, e = s.CreateSupplier(ctx, a, v)
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
func ConsultarFornecedor(c *fiber.Ctx) error {
	id := c.Params("supplier_id")
	if !onlinecatalog.ValidUUID(id) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.Supplier(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func OperacaoFornecedor(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.SupplierOperation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func supplierQuery(c *fiber.Ctx) (onlinecatalog.SupplierQuery, error) {
	q := onlinecatalog.SupplierQuery{Active: "all", Limit: 50}
	if len(c.Body()) != 0 {
		return q, onlinecatalog.ErrInput
	}
	l, o, e := apicontract.PageQuery(c, "q", "active")
	if e != nil {
		return q, onlinecatalog.ErrInput
	}
	q.Limit = l
	q.Offset = o
	q.Query = strings.TrimSpace(c.Query("q"))
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) == "active" {
			q.Active = string(v)
		}
	})
	return q, onlinecatalog.ValidateSupplierQuery(q)
}
func ListarFornecedores(c *fiber.Ctx) error {
	q, e := supplierQuery(c)
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.Suppliers(ctx, a, q)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
