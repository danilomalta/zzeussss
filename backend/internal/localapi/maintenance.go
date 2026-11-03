package localapi

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/fleet"
	"titansystem-backend/internal/localdb/localauth"
)

var maintenanceResources = []struct{ path, kind string }{{"specialties", "specialty"}, {"technicians", "technician"}, {"assets", "asset"}, {"classifications", "request"}}

func (s *Server) mountMaintenance(r fiber.Router) {
	g := r.Group("/maintenance", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/operations/:id", s.maintenanceOperation)
	g.Get("/candidates", s.maintenanceCandidates)
	g.Get("/queue", s.maintenanceQueue)
	for _, v := range maintenanceResources {
		path := "/" + v.path
		kind := v.kind
		g.Get(path, func(c *fiber.Ctx) error { return s.maintenanceList(c, kind) })
		g.Get(path+"/:id", func(c *fiber.Ctx) error { return s.maintenanceGet(c, kind) })
		g.Get(path+"/:id/history", func(c *fiber.Ctx) error { return s.maintenanceHistory(c, kind) })
		g.Post(path, func(c *fiber.Ctx) error { return s.maintenanceWrite(c, kind, false) })
		g.Post(path+"/:id/actions", func(c *fiber.Ctx) error { return s.maintenanceWrite(c, kind, true) })
	}
}
func (s *Server) maintenanceWrite(c *fiber.Ctx, kind string, action bool) error {
	in, e := fleetInput(c)
	if e != nil {
		return fleetError(c, e)
	}
	if action {
		id, e := fleetID(c)
		if e != nil || id != in.ID || in.Action == "create" || in.ExpectedRevision < 1 {
			return c.SendStatus(400)
		}
	} else if in.Action != "create" || in.ExpectedRevision != 0 {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.MaintenanceWrite(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, kind, in)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) maintenanceGet(c *fiber.Ctx, kind string) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.MaintenanceGet(c.UserContext(), s.DB, session.Actor, session.Device, kind, id)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) maintenanceList(c *fiber.Ctx, kind string) error {
	if !inventoryQuery(c, "state", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil || c.Context().QueryArgs().Has("state") && c.Query("state") == "" {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.MaintenanceList(c.UserContext(), s.DB, session.Actor, session.Device, kind, c.Query("state"), limit, offset)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) maintenanceHistory(c *fiber.Ctx, kind string) error {
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
	out, e := fleet.MaintenanceRecordHistory(c.UserContext(), s.DB, session.Actor, session.Device, kind, id, limit, offset)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) maintenanceOperation(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := fleetID(c)
	if e != nil {
		return fleetError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.MaintenanceOperation(c.UserContext(), s.DB, session.Actor, session.Device, id)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) maintenanceCandidates(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.MaintenanceCandidates(c.UserContext(), s.DB, session.Actor, session.Device)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(fiber.Map{"items": out})
}
func (s *Server) maintenanceQueue(c *fiber.Ctx) error {
	if !inventoryQuery(c, "specialty_id", "technician_id", "state", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	for _, k := range []string{"specialty_id", "technician_id", "state"} {
		if c.Context().QueryArgs().Has(k) && c.Query(k) == "" {
			return c.SendStatus(400)
		}
	}
	session := c.Locals("session").(localauth.Session)
	out, e := fleet.MaintenanceDispatch(c.UserContext(), s.DB, session.Actor, session.Device, c.Query("specialty_id"), c.Query("technician_id"), c.Query("state"), limit, offset)
	if e != nil {
		return fleetError(c, e)
	}
	return c.JSON(out)
}
