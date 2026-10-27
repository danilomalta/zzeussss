package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountReceivingBoard(r fiber.Router) {
	s.mountReceivingCases(r)
	g := r.Group("/management/purchases", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/receiving-board", s.receivingBoard)
	g.Get("/receiving-board.csv", s.receivingBoard)
}
func (s *Server) receivingBoard(c *fiber.Ctx) error {
	if !inventoryQuery(c, "from", "to", "supplier_id", "product_id", "state", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	for _, k := range []string{"supplier_id", "product_id", "state"} {
		if c.Context().QueryArgs().Has(k) && c.Query(k) == "" {
			return c.SendStatus(400)
		}
	}
	state := c.Query("state", "pending")
	session := c.Locals("session").(localauth.Session)
	out, e := purchases.Board(c.UserContext(), s.DB, session.Actor, session.Device, purchases.BoardInput{From: c.Query("from"), To: c.Query("to"), SupplierID: c.Query("supplier_id"), ProductID: c.Query("product_id"), State: state, Limit: limit, Offset: offset})
	if e != nil {
		if errors.Is(e, purchases.ErrBoardLimit) || errors.Is(e, purchases.ErrOverviewLimit) || errors.Is(e, dockagenda.ErrInsightLimit) {
			return c.Status(422).JSON(fiber.Map{"code": "receiving_board_source_limit", "max_orders": purchases.MaxBoardOrders, "max_source_evidence": purchases.MaxBoardEvidence, "max_per_order": purchases.MaxOverviewRows})
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
	rows := [][]string{{"record_type", "supplier_id", "product_id", "unit", "order_id", "planned_milli_exact", "accepted_milli_exact", "remaining_milli_exact", "pending_milli_exact", "orders", "pending_orders", "received_orders", "cancelled_orders", "commercial_status", "receiving_status", "review_flags", "from_inclusive", "to_exclusive", "cohort_orders", "matched_orders", "source_evidence", "limit", "offset", "basis", "measured_at"}}
	rows = append(rows, []string{"summary", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", out.From, out.To, strconv.Itoa(out.CohortOrders), strconv.Itoa(out.MatchedOrders), n(out.SourceEvidence), strconv.Itoa(out.Limit), strconv.Itoa(out.Offset), out.Basis, out.MeasuredAt})
	for _, v := range out.Groups {
		rows = append(rows, []string{"group", reportCell(v.SupplierID), reportCell(v.ProductID), v.Unit, "", v.PlannedMilliExact, v.AcceptedMilliExact, v.RemainingMilliExact, v.PendingMilliExact, strconv.Itoa(v.Orders), strconv.Itoa(v.PendingOrders), strconv.Itoa(v.ReceivedOrders), strconv.Itoa(v.CancelledOrders), "", "", "", out.From, out.To, "", "", "", "", "", out.Basis, out.MeasuredAt})
	}
	for _, v := range out.Items {
		pending := "0"
		if v.CommercialStatus != "cancelled" {
			pending = n(v.RemainingMilli)
		}
		rows = append(rows, []string{"order", reportCell(v.SupplierID), reportCell(v.ProductID), v.Unit, reportCell(v.OrderID), n(v.PlannedMilli), n(v.AcceptedMilli), n(v.RemainingMilli), pending, "1", "", "", "", v.CommercialStatus, v.ReceivingStatus, strings.Join(v.ReviewFlags, ";"), out.From, out.To, "", "", "", "", "", out.Basis, out.MeasuredAt})
	}
	return dockInsightCSV(c, "painel-recebimentos-pagina.csv", rows)
}
