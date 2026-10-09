package delivery

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"strings"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinecatalog"
)

func createManaged(c *fiber.Ctx, req CreateProductRequest) error {
	if !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	op := c.Get("Idempotency-Key")
	if !onlinecatalog.ValidUUID(op) {
		return c.Status(428).JSON(fiber.Map{"erro": "Idempotency-Key UUID obrigatório; guarde a identidade antes do envio"})
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.Create(ctx, a, op, onlinecatalog.NewProduct{Name: req.Nome, Description: req.Descricao, SKU: req.SKU, PriceCents: req.priceCents, Stock: int64(req.Estoque)})
	if e != nil {
		return managementError(c, e)
	}
	p := r.Snapshot
	// Keep the legacy product keys; use an exact JSON decimal, never float money.
	return c.Status(201).JSON(fiber.Map{"ID": p.ID, "nome": p.Name, "descricao": p.Description, "sku": p.SKU, "preco": json.Number(fmt.Sprintf("%d.%02d", p.PriceCents/100, p.PriceCents%100)), "price_cents": p.PriceCents, "estoque": p.Stock, "ativo": p.Active, "version": p.Version, "operation_id": r.OperationID, "CreatedAt": p.CreatedAt, "UpdatedAt": p.UpdatedAt})
}
func ConsultarCadastro(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.Creation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func AuditoriaCadastro(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	if e != nil || !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.CreationOfProduct(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func BuscarProdutos(c *fiber.Ctx) error {
	limit, offset, e := apicontract.PageQuery(c, "q", "active")
	if e != nil {
		return managementError(c, onlinecatalog.ErrInput)
	}
	active := c.Query("active", "all")
	q := strings.TrimSpace(c.Query("q"))
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	items, e := s.Search(ctx, a, q, active, limit, offset)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(fiber.Map{"items": items, "limit": limit, "offset": offset})
}
