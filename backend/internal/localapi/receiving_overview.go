package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"net/url"
	"strconv"
	"strings"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountReceivingOverview(r fiber.Router) {
	s.mountReceivingBoard(r)
	g := r.Group("/purchase-orders", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/:id/receiving-overview", s.receivingOverview)
	g.Get("/:id/receiving-overview.csv", s.receivingOverview)
}
func (s *Server) receivingOverview(c *fiber.Ctx) error {
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
	out, e := purchases.Overview(c.UserContext(), s.DB, session.Actor, session.Device, purchases.OverviewInput{OrderID: id, Limit: limit, Offset: offset})
	if e != nil {
		if errors.Is(e, purchases.ErrOverviewLimit) || errors.Is(e, dockagenda.ErrInsightLimit) {
			return c.Status(422).JSON(fiber.Map{"code": "receiving_overview_source_limit", "max_source_records": purchases.MaxOverviewRows})
		}
		if errors.Is(e, dockagenda.ErrConflict) {
			return c.SendStatus(409)
		}
		return purchaseError(c, e)
	}
	if !strings.HasSuffix(c.Path(), ".csv") {
		return c.JSON(out)
	}
	n := func(v int64) string { return strconv.FormatInt(v, 10) }
	rows := [][]string{{"record_type", "order_id", "supplier_id", "product_id", "unit", "planned_milli", "effective_delivered_milli", "effective_accepted_milli", "partial_rejected_milli", "remaining_milli", "fully_rejected_milli_exact", "voided_accepted_milli_exact", "commercial_status", "receiving_status", "review_flags", "appointment_id", "dock_id", "agenda_status", "starts_at", "ends_at", "total_appointments", "limit", "offset", "supplier_confirmation", "payment_evaluation", "measured_at"}}
	rows = append(rows, []string{"summary", reportCell(out.OrderID), reportCell(out.SupplierID), reportCell(out.ProductID), out.Unit, n(out.PlannedMilli), n(out.EffectiveDeliveredMilli), n(out.EffectiveAcceptedMilli), n(out.PartialRejectedMilli), n(out.RemainingMilli), out.FullyRejectedMilliExact, out.VoidedAcceptedMilliExact, out.CommercialStatus, out.ReceivingStatus, strings.Join(out.ReviewFlags, ";"), "", "", "", "", "", strconv.Itoa(out.TotalAppointments), strconv.Itoa(out.Limit), strconv.Itoa(out.Offset), out.SupplierConfirmation, out.PaymentEvaluation, out.MeasuredAt})
	for _, r := range out.Appointments {
		rows = append(rows, []string{"appointment", reportCell(out.OrderID), reportCell(out.SupplierID), "", "", "", "", "", "", "", "", "", "", "", "", reportCell(r.ID), reportCell(r.DockID), r.Status, r.StartsAt, r.EndsAt, "", "", "", out.SupplierConfirmation, out.PaymentEvaluation, out.MeasuredAt})
	}
	return dockInsightCSV(c, "conferencia-recebimento-pagina.csv", rows)
}
