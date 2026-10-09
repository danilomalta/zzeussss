package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func AlterarCodigoProduto(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		product, e := onlinecatalog.ProductID(c.Params("id"))
		if e != nil || !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		codeID := c.Params("barcode_id")
		if action == "active" && !onlinecatalog.ValidUUID(codeID) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		v, e := onlinecatalog.DecodeBarcode(c.Get("Content-Type"), c.Body(), action)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.ChangeBarcode(ctx, a, product, codeID, action, v)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
}
func ConsultarOperacaoCodigo(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.BarcodeOperation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func LocalizarCodigoProduto(c *fiber.Ctx) error {
	invalid := false
	seen := false
	code := ""
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) != "code" || seen {
			invalid = true
		}
		seen = true
		code = string(v)
	})
	if !seen || invalid || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	if _, e := onlinecatalog.CanonicalBarcode(code); e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.LookupBarcode(ctx, a, code)
	if e != nil {
		return managementError(c, e)
	}
	c.Set("X-Content-Type-Options", "nosniff")
	return c.JSON(r)
}
func ListarCodigosProduto(history bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		product, e := onlinecatalog.ProductID(c.Params("id"))
		extra := []string{}
		if !history {
			extra = append(extra, "state")
		}
		limit, offset, pe := apicontract.PageQuery(c, extra...)
		if e != nil || pe != nil || len(c.Body()) != 0 || limit > 50 {
			return managementError(c, onlinecatalog.ErrInput)
		}
		state := "all"
		if c.Context().QueryArgs().Has("state") {
			state = c.Query("state")
		}
		if !history && state != "all" && state != "active" && state != "inactive" {
			return managementError(c, onlinecatalog.ErrInput)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		if history {
			r, e := s.BarcodeHistory(ctx, a, product, limit, offset)
			if e != nil {
				return managementError(c, e)
			}
			return c.JSON(r)
		}
		r, e := s.ListBarcodes(ctx, a, product, state, limit, offset)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
}
