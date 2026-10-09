package delivery

import (
	"context"
	"errors"
	"github.com/gofiber/fiber/v2"
	"time"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/onlinecatalog"
	"titansystem-backend/pkg/middleware"
)

func managementContext(c *fiber.Ctx) (context.Context, context.CancelFunc, onlinecatalog.Store, onlinecatalog.Actor, error) {
	c.Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(c.UserContext(), 10*time.Second)
	tenant, e := middleware.TenantID(c)
	role, _ := c.Locals("role").(string)
	session, _ := c.Locals("session_id").(string)
	a := onlinecatalog.Actor{Tenant: tenant, User: middleware.UserID(c), Session: session, Role: role}
	if e != nil || database.DB == nil {
		return ctx, cancel, onlinecatalog.Store{}, a, onlinecatalog.ErrUnavailable
	}
	db, e := database.DB.DB()
	return ctx, cancel, onlinecatalog.Store{DB: db}, a, e
}
func managementError(c *fiber.Ctx, e error) error {
	status := 503
	switch {
	case errors.Is(e, onlinecatalog.ErrInput):
		status = 400
	case errors.Is(e, onlinecatalog.ErrMissing):
		status = 404
	case errors.Is(e, onlinecatalog.ErrConflict):
		status = 409
	case errors.Is(e, onlinecatalog.ErrDenied):
		status = 403
	}
	return c.Status(status).JSON(fiber.Map{"erro": map[int]string{400: "entrada de catálogo inválida", 404: "registro não encontrado", 409: "versão, SKU ou operação em conflito", 403: "operação não autorizada", 503: "catálogo indisponível; consulte a operação antes de repetir"}[status]})
}
func noManagementQuery(c *fiber.Ctx) bool { return c.Context().QueryArgs().Len() == 0 }
func ConsultarProduto(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	if e != nil || !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	p, e := s.Product(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(p)
}
func AlterarProduto(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, e := onlinecatalog.ProductID(c.Params("id"))
		if e != nil || !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		change, e := onlinecatalog.Decode(c.Get("Content-Type"), c.Body(), action)
		if e != nil {
			return managementError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		r, e := s.Mutate(ctx, a, id, action, change)
		if e != nil {
			return managementError(c, e)
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(r)
	}
}
func ConsultarOperacaoCatalogo(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.Receipt(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(r)
}
func HistoricoProduto(c *fiber.Ctx) error {
	id, e := onlinecatalog.ProductID(c.Params("id"))
	limit, offset, paginationError := apicontract.PageQuery(c)
	if e != nil || paginationError != nil {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	items, e := s.History(ctx, a, id, limit, offset)
	if e != nil {
		return managementError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"items": items, "limit": limit, "offset": offset})
}
