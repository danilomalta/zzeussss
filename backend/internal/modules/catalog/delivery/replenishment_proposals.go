package delivery

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/onlinecatalog"
)

func CriarPropostaReposicao(c *fiber.Ctx) error {
	if !noManagementQuery(c) {
		return managementError(c, onlinecatalog.ErrInput)
	}
	v, e := onlinecatalog.DecodeReplenishmentProposal(c.Get("Content-Type"), c.Body())
	if e != nil {
		return managementError(c, e)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.CreateReplenishmentProposal(ctx, a, v)
	if e != nil {
		return managementError(c, e)
	}
	return c.Status(fiber.StatusCreated).JSON(r)
}
func ConsultarPropostaReposicao(c *fiber.Ctx) error {
	id := c.Params("proposal_id")
	if !onlinecatalog.ValidUUID(id) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.ReplenishmentProposal(ctx, a, id)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
func DecidirPropostaReposicao(kind string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("proposal_id")
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
		r, e := s.DecideReplenishmentProposal(ctx, a, id, kind, v)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(r)
	}
}
func ConsultarOperacaoPropostaReposicao(c *fiber.Ctx) error {
	op := c.Params("operation_id")
	if !onlinecatalog.ValidUUID(op) || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.ReplenishmentProposalOperation(ctx, a, op)
	if e != nil {
		return managementError(c, e)
	}
	return c.JSON(r)
}
