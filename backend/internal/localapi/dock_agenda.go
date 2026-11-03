package localapi

import (
	"bytes"
	"encoding/csv"
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountDockAgenda(r fiber.Router) {
	s.mountDockInsights(r)
	g := r.Group("/logistics", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/docks", s.agendaDocks)
	g.Post("/docks", s.agendaSaveDock)
	g.Get("/docks/:id/history", s.agendaHistory)
	g.Get("/appointments", s.agendaList)
	g.Get("/appointments.csv", s.agendaList)
	g.Post("/appointments", s.agendaBook)
	g.Get("/appointments/:id", s.agendaGet)
	g.Post("/appointments/:id/actions", s.agendaAct)
	g.Get("/appointments/:id/history", s.agendaHistory)
	g.Get("/operations/:id", s.agendaReceipt)
	g.Get("/occupancy", s.agendaOccupancy)
}

func (s *Server) agendaOccupancy(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, e := dockagenda.Occupancies(c.UserContext(), s.DB, session.Actor, session.Device, limit, offset)
	if e != nil {
		return agendaError(c, e)
	}
	return c.JSON(r)
}
func agendaError(c *fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, dockagenda.ErrInvalid):
		return c.Status(400).JSON(fiber.Map{"code": "agenda_invalid"})
	case errors.Is(e, dockagenda.ErrNotFound):
		return c.Status(404).JSON(fiber.Map{"code": "agenda_not_found"})
	case errors.Is(e, dockagenda.ErrOverlap):
		return c.Status(409).JSON(fiber.Map{"code": "agenda_slot_occupied"})
	case errors.Is(e, dockagenda.ErrOrder):
		return c.Status(409).JSON(fiber.Map{"code": "agenda_order_unavailable"})
	case errors.Is(e, dockagenda.ErrConflict):
		return c.Status(409).JSON(fiber.Map{"code": "agenda_conflict"})
	default:
		return catalogError(c, e)
	}
}
func agendaWriteOK(c *fiber.Ctx, max int) bool { return inventoryQuery(c) && len(c.Body()) <= max }
func (s *Server) agendaSaveDock(c *fiber.Ctx) error {
	if !agendaWriteOK(c, 4096) {
		if len(c.Body()) > 4096 {
			return c.SendStatus(413)
		}
		return c.SendStatus(400)
	}
	var in dockagenda.DockInput
	if e := decodeCash(c.Body(), &in, []string{"operation_id", "dock_id", "name", "enabled", "expected_revision"}); e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, e := dockagenda.SaveDock(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if e != nil {
		return agendaError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) agendaBook(c *fiber.Ctx) error {
	if !agendaWriteOK(c, 8192) {
		if len(c.Body()) > 8192 {
			return c.SendStatus(413)
		}
		return c.SendStatus(400)
	}
	var in dockagenda.BookInput
	if e := decodeCash(c.Body(), &in, []string{"operation_id", "appointment_id", "dock_id", "order_id", "starts_at", "ends_at", "reason"}); e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, e := dockagenda.Book(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if e != nil {
		return agendaError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) agendaAct(c *fiber.Ctx) error {
	if !agendaWriteOK(c, 8192) {
		if len(c.Body()) > 8192 {
			return c.SendStatus(413)
		}
		return c.SendStatus(400)
	}
	var in dockagenda.ActionInput
	if e := decodeReportObject(c.Body(), &in, []string{"operation_id", "expected_revision", "action", "reason"}, []string{"dock_id", "starts_at", "ends_at"}); e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, e := dockagenda.Act(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, c.Params("id"), in)
	if e != nil {
		return agendaError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) agendaDocks(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, e := dockagenda.Docks(c.UserContext(), s.DB, session.Actor, session.Device, limit, offset)
	if e != nil {
		return agendaError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) agendaGet(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, e := dockagenda.Get(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if e != nil {
		return agendaError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) agendaReceipt(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, e := dockagenda.Receipt(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if e != nil {
		return agendaError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) agendaHistory(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	kind := "appointment"
	if bytes.Contains([]byte(c.Path()), []byte("/docks/")) {
		kind = "dock"
	}
	r, e := dockagenda.History(c.UserContext(), s.DB, session.Actor, session.Device, kind, c.Params("id"), limit, offset)
	if e != nil {
		return agendaError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) agendaList(c *fiber.Ctx) error {
	if !inventoryQuery(c, "from", "to", "dock_id", "status", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	if (c.Context().QueryArgs().Has("dock_id") && c.Query("dock_id") == "") || (c.Context().QueryArgs().Has("status") && c.Query("status") == "") {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, e := dockagenda.Appointments(c.UserContext(), s.DB, session.Actor, session.Device, dockagenda.Window{From: c.Query("from"), To: c.Query("to"), DockID: c.Query("dock_id"), Status: c.Query("status"), Limit: limit, Offset: offset})
	if e != nil {
		return agendaError(c, e)
	}
	if !bytes.HasSuffix([]byte(c.Path()), []byte(".csv")) {
		return c.JSON(r)
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if e = w.Write([]string{"appointment_id", "dock_id", "order_id", "supplier_id", "starts_at", "ends_at", "status", "revision", "arrived_at", "unloading_at", "completed_at", "supplier_confirmed", "stock_received"}); e != nil {
		return c.SendStatus(500)
	}
	optional := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	for _, a := range r.Items {
		if e = w.Write([]string{reportCell(a.ID), reportCell(a.DockID), reportCell(a.OrderID), reportCell(a.SupplierID), a.StartsAt, a.EndsAt, a.Status, strconv.FormatInt(a.Revision, 10), optional(a.ArrivedAt), optional(a.UnloadingAt), optional(a.CompletedAt), "false", "false"}); e != nil {
			return c.SendStatus(500)
		}
	}
	w.Flush()
	if w.Error() != nil {
		return c.SendStatus(500)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="agenda-docas-pagina.csv"`)
	return c.Send(b.Bytes())
}
