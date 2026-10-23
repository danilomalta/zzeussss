package localapi

import (
	"encoding/csv"
	"errors"
	"github.com/gofiber/fiber/v2"
	"net/url"
	"strconv"
	"strings"
	"titansystem-backend/internal/localdb/fleet"
	"titansystem-backend/internal/localdb/localauth"
)

var fleetResources = []struct{ plural, kind string }{{"assets", "asset"}, {"requests", "request"}, {"trips", "trip"}, {"plans", "plan"}, {"providers", "provider"}, {"quotes", "quote"}, {"expenses", "expense"}, {"income", "income"}, {"impacts", "impact"}, {"profiles", "profile"}}

func (s *Server) mountFleet(r fiber.Router) {
	s.mountFleetWorkshop(r)
	g := r.Group("/fleet", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/operations/:id", s.fleetOperation)
	g.Get("/notices", s.fleetNotices)
	g.Post("/notices/:id/ack", s.fleetAck)
	g.Get("/assets/:id/availability", s.fleetAvailability)
	g.Get("/assets/:id/preventive", s.fleetPreventive)
	g.Get("/assets/:id/costs", s.fleetCosts)
	g.Get("/assets/:id/costs.csv", s.fleetCosts)
	for _, v := range fleetResources {
		kind := v.kind
		path := "/" + v.plural
		g.Get(path, func(c *fiber.Ctx) error { return s.fleetList(c, kind) })
		g.Post(path, func(c *fiber.Ctx) error { return s.fleetWrite(c, kind, false) })
		g.Get(path+"/:id", func(c *fiber.Ctx) error { return s.fleetGet(c, kind) })
		g.Get(path+"/:id/history", func(c *fiber.Ctx) error { return s.fleetHistory(c, kind) })
		g.Post(path+"/:id/actions", func(c *fiber.Ctx) error { return s.fleetWrite(c, kind, true) })
	}
}
func fleetError(c *fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, fleet.ErrWorkshopLimit):
		return c.Status(422).JSON(fiber.Map{"code": "fleet_workshop_source_limit", "max_source_records": fleet.MaxWorkshopSource})
	case errors.Is(e, fleet.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(e, fleet.ErrNotFound):
		return c.SendStatus(404)
	case errors.Is(e, fleet.ErrConflict):
		return c.SendStatus(409)
	case errors.Is(e, fleet.ErrLimit):
		return c.Status(422).JSON(fiber.Map{"code": "fleet_source_limit", "max_source_records": fleet.MaxSource})
	}
	return catalogError(c, e)
}
func fleetID(c *fiber.Ctx) (string, error) {
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil || !fleet.ID(id) {
		return "", fleet.ErrInvalid
	}
	return id, nil
}
func fleetInput(c *fiber.Ctx) (fleet.Input, error) {
	var in fleet.Input
	if !agendaWriteOK(c, 24576) || decodeCash(c.Body(), &in, []string{"operation_id", "id", "expected_revision", "action", "reason", "data"}) != nil {
		return in, fleet.ErrInvalid
	}
	return in, nil
}
func (s *Server) fleetWrite(c *fiber.Ctx, kind string, action bool) error {
	in, e := fleetInput(c)
	if e != nil {
		return fleetError(c, e)
	}
	if action {
		id, e := fleetID(c)
		if e != nil || in.ID != id || in.Action == "create" || in.ExpectedRevision < 1 {
			return c.SendStatus(400)
		}
	} else if in.Action != "create" || in.ExpectedRevision != 0 {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.Mutate(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, kind, in)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetGet(c *fiber.Ctx, kind string) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.Get(c.UserContext(), s.DB, session.Actor, session.Device, kind, id)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetList(c *fiber.Ctx, kind string) error {
	if !inventoryQuery(c, "asset_id", "state", "assigned_to", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	for _, k := range []string{"asset_id", "state", "assigned_to"} {
		if c.Context().QueryArgs().Has(k) && c.Query(k) == "" {
			return c.SendStatus(400)
		}
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.List(c.UserContext(), s.DB, session.Actor, session.Device, kind, fleet.Query{AssetID: c.Query("asset_id"), State: c.Query("state"), AssignedTo: c.Query("assigned_to"), Limit: limit, Offset: offset})
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetHistory(c *fiber.Ctx, kind string) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.RecordHistory(c.UserContext(), s.DB, session.Actor, session.Device, kind, id, limit, offset)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetOperation(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.Operation(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetAvailability(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.AssetAvailability(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetPreventive(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.Preventive(c.UserContext(), s.DB, session.Actor, session.Device, id, limit, offset)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetNotices(c *fiber.Ctx) error {
	if !inventoryQuery(c, "after", "limit") {
		return c.SendStatus(400)
	}
	limit, _, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	after := int64(0)
	if c.Context().QueryArgs().Has("after") {
		after, e = strconv.ParseInt(c.Query("after"), 10, 64)
		if e != nil || after < 0 {
			return c.SendStatus(400)
		}
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.Notices(c.UserContext(), s.DB, session.Actor, session.Device, after, limit)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetAck(c *fiber.Ctx) error {
	in, e := fleetInput(c)
	if e != nil {
		return fleetError(c, e)
	}
	id, e := strconv.ParseInt(c.Params("id"), 10, 64)
	if e != nil || id < 1 {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.AcknowledgeNotice(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, id, in)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) fleetCosts(c *fiber.Ctx) error {
	if !inventoryQuery(c, "from", "to") {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.Costs(c.UserContext(), s.DB, session.Actor, session.Device, id, c.Query("from"), c.Query("to"))
	if e != nil {
		return fleetError(c, e)
	}
	if !strings.HasSuffix(c.Path(), ".csv") {
		return c.JSON(out)
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	for _, row := range [][]string{{"asset_id", "from", "to_exclusive", "expense_cents_exact", "attributed_income_cents_exact", "estimated_impact_cents_exact", "period_depreciation_cents_exact", "partial_contribution_cents_exact", "meter_basis", "meter_milli_exact"}, {out.AssetID, out.From, out.ToExclusive, out.ExpenseCentsExact, out.AttributedIncomeCentsExact, out.EstimatedImpactCentsExact, out.PeriodDepreciationCentsExact, out.PartialContributionCentsExact, out.MeterBasis, out.CostPerMeterDenominator}} {
		for i := range row {
			v := strings.TrimLeft(row[i], " \t\r\n")
			if v != "" && strings.ContainsAny(v[:1], "=+-@") {
				row[i] = "'" + row[i]
			}
		}
		if e = w.Write(row); e != nil {
			return c.SendStatus(500)
		}
	}
	w.Flush()
	if w.Error() != nil {
		return c.SendStatus(500)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="fleet-costs.csv"`)
	return c.SendString(b.String())
}
