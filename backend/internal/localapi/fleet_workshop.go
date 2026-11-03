package localapi

import (
	"encoding/csv"
	"github.com/gofiber/fiber/v2"
	"strings"
	"titansystem-backend/internal/localdb/fleet"
	"titansystem-backend/internal/localdb/localauth"
)

var workshopResources = []struct{ path, kind string }{{"templates", "template"}, {"requirements", "binding"}, {"inspections", "inspection"}, {"labor", "labor"}}

func (s *Server) mountFleetWorkshop(r fiber.Router) {
	g := r.Group("/fleet/workshop", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/operations/:id", s.workshopOperation)
	for _, v := range workshopResources {
		kind := v.kind
		path := "/" + v.path
		g.Get(path, func(c *fiber.Ctx) error { return s.workshopList(c, kind) })
		g.Post(path, func(c *fiber.Ctx) error { return s.workshopWrite(c, kind, false) })
		g.Get(path+"/:id", func(c *fiber.Ctx) error { return s.workshopGet(c, kind) })
		g.Get(path+"/:id/history", func(c *fiber.Ctx) error { return s.workshopHistory(c, kind) })
		g.Post(path+"/:id/actions", func(c *fiber.Ctx) error { return s.workshopWrite(c, kind, true) })
	}
	r.Get("/fleet/requests/:id/workshop", s.workshopReport)
	r.Get("/fleet/requests/:id/workshop.csv", s.workshopReport)
}
func (s *Server) workshopWrite(c *fiber.Ctx, kind string, action bool) error {
	in, e := fleetInput(c)
	if e != nil {
		return fleetError(c, e)
	}
	if action {
		id, e := fleetID(c)
		if e != nil || id != in.ID || in.ExpectedRevision < 1 || in.Action == "create" {
			return c.SendStatus(400)
		}
	} else if in.Action != "create" || in.ExpectedRevision != 0 {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.WorkshopMutate(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, kind, in)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) workshopGet(c *fiber.Ctx, kind string) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.WorkshopGet(c.UserContext(), s.DB, session.Actor, session.Device, kind, id)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) workshopList(c *fiber.Ctx, kind string) error {
	if !inventoryQuery(c, "request_id", "state", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	for _, k := range []string{"request_id", "state"} {
		if c.Context().QueryArgs().Has(k) && c.Query(k) == "" {
			return c.SendStatus(400)
		}
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.WorkshopList(c.UserContext(), s.DB, session.Actor, session.Device, kind, c.Query("request_id"), c.Query("state"), limit, offset)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) workshopOperation(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.WorkshopOperation(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) workshopHistory(c *fiber.Ctx, kind string) error {
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
	out, e := fleet.WorkshopRecordHistory(c.UserContext(), s.DB, session.Actor, session.Device, kind, id, limit, offset)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) workshopReport(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.WorkshopReport(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if e != nil {
		return fleetError(c, e)
	}
	if !strings.HasSuffix(c.Path(), ".csv") {
		return c.JSON(out)
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	for _, row := range [][]string{{"request_id", "declared_seconds_exact", "confirmed_seconds_exact", "rejected_seconds_exact", "voided_seconds_exact"}, {out.RequestID, out.DeclaredSecondsExact, out.ConfirmedSecondsExact, out.RejectedSecondsExact, out.VoidedSecondsExact}} {
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
	c.Set("Content-Disposition", `attachment; filename="fleet-workshop.csv"`)
	return c.SendString(b.String())
}
