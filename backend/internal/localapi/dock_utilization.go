package localapi

import (
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountDockUtilization(r fiber.Router) {
	s.mountReceivingOverview(r)
	g := r.Group("/logistics/insights", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/utilization", s.dockUtilization)
	g.Get("/utilization.csv", s.dockUtilization)
}
func (s *Server) dockUtilization(c *fiber.Ctx) error {
	if !inventoryQuery(c, "dock_id", "from", "to", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := dockagenda.Utilization(c.UserContext(), s.DB, session.Actor, session.Device, dockagenda.UtilizationInput{DockID: c.Query("dock_id"), From: c.Query("from"), To: c.Query("to"), Limit: limit, Offset: offset})
	if e != nil {
		return dockInsightError(c, e)
	}
	if !strings.HasSuffix(c.Path(), ".csv") {
		return c.JSON(out)
	}
	exact := func(v dockagenda.ExactSeconds) string { return v.Numerator + "/" + v.Denominator }
	// A summary record is always exported, even for an empty detail page.
	rows := [][]string{{"record_type", "dock_id", "appointment_id", "order_id", "supplier_id", "status", "starts_at", "ends_at", "seconds_exact", "still_unloading", "period_seconds_exact", "planned_seconds_exact", "unloading_seconds_exact", "without_registered_unloading_seconds_exact", "planned_fraction_exact", "unloading_fraction_exact", "from_inclusive", "to_exclusive", "total_segments", "source_appointments", "limit", "offset", "measured_at", "basis"}}
	rows = append(rows, []string{"summary", reportCell(out.Dock.ID), "", "", "", "", "", "", "", "", exact(out.PeriodSeconds), exact(out.PlannedSeconds), exact(out.UnloadingSeconds), exact(out.WithoutRegisteredUnloadingSeconds), exact(out.PlannedFraction), exact(out.UnloadingFraction), out.From, out.To, strconv.Itoa(out.TotalSegments), strconv.Itoa(out.SourceAppointments), strconv.Itoa(out.Limit), strconv.Itoa(out.Offset), out.MeasuredAt, out.Basis})
	for _, v := range out.Days {
		rows = append(rows, []string{"day_utc", reportCell(out.Dock.ID), "", "", "", "", v.From, v.To, "", "", exact(v.PeriodSeconds), exact(v.PlannedSeconds), exact(v.UnloadingSeconds), "", "", exact(v.UnloadingFraction), v.From, v.To, "", "", "", "", out.MeasuredAt, out.Basis})
	}
	for _, v := range out.Items {
		rows = append(rows, []string{"segment", reportCell(out.Dock.ID), reportCell(v.AppointmentID), reportCell(v.OrderID), reportCell(v.SupplierID), v.Status, v.StartsAt, v.EndsAt, exact(v.Seconds), strconv.FormatBool(v.StillUnloading), "", "", "", "", "", "", out.From, out.To, "", "", "", "", out.MeasuredAt, out.Basis})
	}
	return dockInsightCSV(c, "utilizacao-docas-pagina.csv", rows)
}
