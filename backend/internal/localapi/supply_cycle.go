package localapi

import (
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/supplycycle"
)

func (s *Server) mountSupplyCycle(r fiber.Router) {
	g := r.Group("", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Post("/purchase-orders/:id/supplier-decisions", s.cycleDecision)
	g.Get("/purchase-orders/:id/supply-cycle", s.cycleRead)
	g.Post("/purchase-orders/:id/deliveries", s.cyclePlan)
	g.Get("/purchase-deliveries/:id", s.cycleDelivery)
	g.Get("/purchase-deliveries/:id/history", s.cycleHistory)
	g.Post("/purchase-deliveries/:id/actions", s.cycleAction)
	g.Post("/purchase-deliveries/:id/receive", s.cycleReceive)
	g.Post("/purchase-deliveries/:id/payable", s.cycleFinance)
	g.Get("/purchase-cycle-operations/:id", s.cycleOperation)
	g.Get("/finance/receiving-origins/:id", s.cycleOrigin)
}
func cycleError(c *fiber.Ctx, e error) error {
	if errors.Is(e, supplycycle.ErrLimit) {
		return c.Status(422).JSON(fiber.Map{"code": "supply_cycle_source_limit", "max_records": supplycycle.MaxDeliveries})
	}
	if supplycycle.IsConflict(e) {
		return c.SendStatus(409)
	}
	if errors.Is(e, payables.ErrInvalid) || errors.Is(e, payables.ErrRange) || errors.Is(e, payables.ErrNotFound) {
		return payableError(c, e)
	}
	return purchaseError(c, e)
}
func cycleID(c *fiber.Ctx) (string, error) { return url.PathUnescape(c.Params("id")) }
func cycleDecode(c *fiber.Ctx, in any, fields ...string) error {
	if !inventoryQuery(c) {
		return fiber.ErrBadRequest
	}
	if len(c.Body()) > 32768 {
		return fiber.ErrRequestEntityTooLarge
	}
	if decodeCash(c.Body(), in, fields) != nil {
		return fiber.ErrBadRequest
	}
	return nil
}
func cycleReply(c *fiber.Ctx, r supplycycle.Result, e error) error {
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) cycleDecision(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.DecisionInput
	if e := cycleDecode(c, &in, "operation_id", "expected_revision", "decision", "accepted_milli", "promised_at", "contact", "evidence_reference", "observed_at", "reason"); e != nil {
		return e
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.RecordDecision(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	return cycleReply(c, r, e)
}
func (s *Server) cyclePlan(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.PlanInput
	if e := cycleDecode(c, &in, "operation_id", "delivery_id", "decision_revision", "delivery_reference", "planned_milli", "appointment_id", "window_evidence_reference", "receiving_case_id", "reason"); e != nil {
		return e
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.Plan(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	return cycleReply(c, r, e)
}
func (s *Server) cycleAction(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.ActionInput
	if e := cycleDecode(c, &in, "operation_id", "expected_revision", "action", "evidence_reference", "observed_at", "reason"); e != nil {
		return e
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.Act(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	return cycleReply(c, r, e)
}
func (s *Server) cycleReceive(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.ReceiveInput
	if e := cycleDecode(c, &in, "operation_id", "expected_revision", "location_id", "delivered_milli", "accepted_milli", "reason"); e != nil {
		return e
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.Receive(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	return cycleReply(c, r, e)
}
func (s *Server) cycleFinance(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.FinanceInput
	if e := cycleDecode(c, &in, "operation_id", "expected_revision", "payable_id", "document_reference", "source_reference", "description", "issued_on", "currency", "total_cents", "installments"); e != nil {
		return e
	}
	var nested struct {
		Installments []json.RawMessage `json:"installments"`
	}
	if json.Unmarshal(c.Body(), &nested) != nil {
		return c.SendStatus(400)
	}
	for _, raw := range nested.Installments {
		var part payables.Installment
		if decodeCash(raw, &part, []string{"id", "due_on", "amount_cents"}) != nil {
			return c.SendStatus(400)
		}
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.CreatePayable(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	return cycleReply(c, r, e)
}
func (s *Server) cycleRead(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.GetCycle(c.UserContext(), s.DB, ss.Actor, ss.Device, id, limit, offset)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) cycleHistory(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.DeliveryHistory(c.UserContext(), s.DB, ss.Actor, ss.Device, id, limit, offset)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) cycleDelivery(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.GetDelivery(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) cycleOperation(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.Operation(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) cycleOrigin(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := supplycycle.GetOrigin(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(r)
}
