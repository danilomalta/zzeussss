package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountReceivingCases(r fiber.Router) {
	g := r.Group("", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Post("/purchase-orders/:id/receiving-cases", s.receivingCaseOpen)
	g.Get("/purchase-receiving-cases", s.receivingCaseList)
	g.Get("/purchase-receiving-cases/:id", s.receivingCaseGet)
	g.Post("/purchase-receiving-cases/:id/actions", s.receivingCaseAct)
	g.Get("/purchase-receiving-cases/:id/history", s.receivingCaseHistory)
	g.Get("/purchase-receiving-case-operations/:id", s.receivingCaseOperation)
}
func receivingCaseError(c *fiber.Ctx, e error) error {
	if errors.Is(e, purchases.ErrOverviewLimit) || errors.Is(e, dockagenda.ErrInsightLimit) {
		return c.Status(422).JSON(fiber.Map{"code": "receiving_case_source_limit", "max_source_records": purchases.MaxOverviewRows})
	}
	if errors.Is(e, dockagenda.ErrConflict) {
		return c.SendStatus(409)
	}
	return purchaseError(c, e)
}
func (s *Server) receivingCaseOpen(c *fiber.Ctx) error {
	if !agendaWriteOK(c, 8192) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in purchases.CaseOpenInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "case_id", "kind", "requested_outcome", "reference", "assigned_to", "due_at", "reason"}) != nil {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := purchases.OpenReceivingCase(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, id, in)
	if e != nil {
		return receivingCaseError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) receivingCaseAct(c *fiber.Ctx) error {
	if !agendaWriteOK(c, 8192) {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in purchases.CaseActionInput
	if decodeCash(c.Body(), &in, []string{"operation_id", "expected_revision", "action", "reason", "assigned_to", "due_at"}) != nil {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := purchases.ActReceivingCase(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, id, in)
	if e != nil {
		return receivingCaseError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) receivingCaseGet(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := purchases.GetReceivingCase(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if e != nil {
		return receivingCaseError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) receivingCaseList(c *fiber.Ctx) error {
	if !inventoryQuery(c, "order_id", "status", "assigned_to", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	for _, k := range []string{"order_id", "status", "assigned_to"} {
		if c.Context().QueryArgs().Has(k) && c.Query(k) == "" {
			return c.SendStatus(400)
		}
	}
	session := c.Locals("session").(localauth.Session)
	out, e := purchases.ListReceivingCases(c.UserContext(), s.DB, session.Actor, session.Device, purchases.CaseListInput{OrderID: c.Query("order_id"), Status: c.Query("status"), AssignedTo: c.Query("assigned_to"), Limit: limit, Offset: offset})
	if e != nil {
		return receivingCaseError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) receivingCaseHistory(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := purchases.ReceivingCaseHistory(c.UserContext(), s.DB, session.Actor, session.Device, id, limit, offset)
	if e != nil {
		return receivingCaseError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) receivingCaseOperation(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := purchases.ReceivingCaseOperation(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if e != nil {
		return receivingCaseError(c, e)
	}
	return c.JSON(out)
}
