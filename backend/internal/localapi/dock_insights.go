package localapi

import (
	"bytes"
	"encoding/csv"
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountDockInsights(r fiber.Router) {
	s.mountDockUtilization(r)
	g := r.Group("/logistics/insights", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/availability", s.dockFreeWindows)
	g.Get("/suppliers/performance", s.dockPerformance)
	g.Get("/suppliers/performance.csv", s.dockPerformance)
	g.Get("/pending", s.dockPending)
	g.Get("/pending.csv", s.dockPending)
}
func dockInsightError(c *fiber.Ctx, e error) error {
	if errors.Is(e, dockagenda.ErrInsightLimit) {
		return c.Status(422).JSON(fiber.Map{"code": "agenda_report_source_limit", "max_appointments": dockagenda.MaxInsightRows})
	}
	return agendaError(c, e)
}
func (s *Server) dockFreeWindows(c *fiber.Ctx) error {
	if !inventoryQuery(c, "dock_id", "from", "to", "duration_minutes") {
		return c.SendStatus(400)
	}
	raw := c.Query("duration_minutes")
	if raw == "" {
		return c.SendStatus(400)
	}
	for _, v := range raw {
		if v < '0' || v > '9' {
			return c.SendStatus(400)
		}
	}
	n, e := strconv.ParseInt(raw, 10, 64)
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := dockagenda.AvailableWindows(c.UserContext(), s.DB, session.Actor, session.Device, dockagenda.AvailabilityInput{DockID: c.Query("dock_id"), From: c.Query("from"), To: c.Query("to"), DurationMinutes: n})
	if e != nil {
		return dockInsightError(c, e)
	}
	return c.JSON(out)
}
func dockInsightFilter(c *fiber.Ctx, period bool) (dockagenda.InsightFilter, error) {
	var in dockagenda.InsightFilter
	allowed := []string{"dock_id", "supplier_id", "limit", "offset"}
	if period {
		allowed = append(allowed, "from", "to")
	}
	if !inventoryQuery(c, allowed...) {
		return in, dockagenda.ErrInvalid
	}
	var e error
	in.Limit, in.Offset, e = inventoryPage(c)
	if e != nil {
		return in, dockagenda.ErrInvalid
	}
	for _, key := range []string{"dock_id", "supplier_id"} {
		if c.Context().QueryArgs().Has(key) && c.Query(key) == "" {
			return in, dockagenda.ErrInvalid
		}
	}
	in.DockID = c.Query("dock_id")
	in.SupplierID = c.Query("supplier_id")
	if period {
		in.From = c.Query("from")
		in.To = c.Query("to")
	}
	return in, nil
}
func dockInsightCSV(c *fiber.Ctx, name string, rows [][]string) error {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if e := w.WriteAll(rows); e != nil {
		return c.SendStatus(500)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="`+name+`"`)
	return c.Send(b.Bytes())
}
func (s *Server) dockPerformance(c *fiber.Ctx) error {
	in, e := dockInsightFilter(c, true)
	if e != nil {
		return dockInsightError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := dockagenda.SupplierPerformance(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if e != nil {
		return dockInsightError(c, e)
	}
	if !strings.HasSuffix(c.Path(), ".csv") {
		return c.JSON(out)
	}
	rows := [][]string{{"supplier_id", "appointments", "reserved", "arrived", "unloading", "completed", "cancelled", "no_show", "arrival_samples", "late_arrivals", "waiting_samples", "unloading_samples", "average_wait_seconds_exact", "average_unload_seconds_exact", "from_inclusive", "to_exclusive", "basis", "measured_at"}}
	ratio := func(r *dockagenda.ExactSeconds) string {
		if r == nil {
			return ""
		}
		return r.Numerator + "/" + r.Denominator
	}
	for _, m := range out.Items {
		rows = append(rows, []string{reportCell(m.SupplierID), strconv.FormatInt(m.Appointments, 10), strconv.FormatInt(m.Reserved, 10), strconv.FormatInt(m.Arrived, 10), strconv.FormatInt(m.Unloading, 10), strconv.FormatInt(m.Completed, 10), strconv.FormatInt(m.Cancelled, 10), strconv.FormatInt(m.NoShow, 10), strconv.FormatInt(m.ArrivalSamples, 10), strconv.FormatInt(m.LateArrivals, 10), strconv.FormatInt(m.WaitingSamples, 10), strconv.FormatInt(m.UnloadingSamples, 10), ratio(m.AverageWaitingSeconds), ratio(m.AverageUnloadingSeconds), out.From, out.To, out.Basis, out.MeasuredAt})
	}
	return dockInsightCSV(c, "desempenho-fornecedores-pagina.csv", rows)
}
func (s *Server) dockPending(c *fiber.Ctx) error {
	in, e := dockInsightFilter(c, false)
	if e != nil {
		return dockInsightError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := dockagenda.Pending(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if e != nil {
		return dockInsightError(c, e)
	}
	if !strings.HasSuffix(c.Path(), ".csv") {
		return c.JSON(out)
	}
	rows := [][]string{{"appointment_id", "dock_id", "order_id", "supplier_id", "status", "reason", "priority", "order_cancelled", "starts_at", "ends_at", "measured_at"}}
	for _, v := range out.Items {
		a := v.Appointment
		rows = append(rows, []string{reportCell(a.ID), reportCell(a.DockID), reportCell(a.OrderID), reportCell(a.SupplierID), a.Status, v.Reason, strconv.Itoa(v.Priority), strconv.FormatBool(v.OrderCancelled), a.StartsAt, a.EndsAt, out.MeasuredAt})
	}
	return dockInsightCSV(c, "pendencias-docas-pagina.csv", rows)
}
