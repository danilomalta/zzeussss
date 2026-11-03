package localapi

import (
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/production"
)

func (s *Server) mountProductionStages(r fiber.Router) {
	r.Post("/production/stage-plans", s.configureProductionStages)
	r.Post("/production/stages/state", s.changeProductionStage)
	r.Get("/production/orders/:id/stages/history", s.productionStageHistory)
	r.Get("/production/orders/:id/stages", s.productionStages)
}
func decodeStagePlan(body []byte) (production.StagePlanInput, error) {
	var envelope struct {
		Stages json.RawMessage `json:"stages"`
	}
	if err := decodeCash(body, &envelope, []string{"operation_id", "order_id", "stages", "reason"}); err != nil {
		return production.StagePlanInput{}, production.ErrInvalid
	}
	raw := bytes.TrimSpace(envelope.Stages)
	if len(raw) == 0 || raw[0] != '[' {
		return production.StagePlanInput{}, production.ErrInvalid
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) < 1 || len(items) > 20 {
		return production.StagePlanInput{}, production.ErrInvalid
	}
	for _, item := range items {
		var stage production.StageDefinition
		if err := decodeCash(item, &stage, []string{"stage_id", "name", "responsible_id"}); err != nil {
			return production.StagePlanInput{}, production.ErrInvalid
		}
	}
	var in production.StagePlanInput
	if err := json.Unmarshal(body, &in); err != nil {
		return production.StagePlanInput{}, production.ErrInvalid
	}
	return in, nil
}
func (s *Server) configureProductionStages(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 16384 {
		return c.SendStatus(413)
	}
	in, err := decodeStagePlan(c.Body())
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.ConfigureStagePlan(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return productionError(c, err)
	}
	status := 201
	if out.Repeated {
		status = 200
	}
	return c.Status(status).JSON(out)
}
func (s *Server) changeProductionStage(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.StageStateInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "order_id", "stage_id", "expected_revision", "status", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.ChangeStageState(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(out)
}
func (s *Server) productionStages(c *fiber.Ctx) error {
	if !productionQuery(c, false) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.GetStagePlan(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(out)
}
func (s *Server) productionStageHistory(c *fiber.Ctx) error {
	offset, err := orderOffset(c)
	if err != nil {
		return productionError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := production.StageHistory(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), offset)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(fiber.Map{"items": out})
}
