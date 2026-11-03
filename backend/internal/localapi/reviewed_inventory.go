package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stocktaking"
)

func (s *Server) mountReviewedInventory(r fiber.Router) {
	s.mountManagementReports(r)
	s.mountMaterialCost(r)
	g := r.Group("/stock/inventories", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Post("", s.openReviewedInventory)
	g.Get("", s.listReviewedInventory)
	g.Get("/:id/history", s.historyReviewedInventory)
	g.Get("/:id/report.csv", s.exportReviewedInventory)
	g.Get("/:id", s.getReviewedInventory)
	g.Post("/:id/counts", s.countReviewedInventory)
	g.Post("/:id/state", s.stateReviewedInventory)
	r.Get("/stock/inventory-operations/:id", s.receiptReviewedInventory)
	r.Get("/system/info", s.localSystemInfo)
}
func inventoryError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, stocktaking.ErrInvalid):
		return c.Status(400).JSON(fiber.Map{"code": "inventory_invalid"})
	case errors.Is(err, stocktaking.ErrNotFound):
		return c.Status(404).JSON(fiber.Map{"code": "inventory_not_found"})
	case errors.Is(err, stocktaking.ErrStale):
		return c.Status(409).JSON(fiber.Map{"code": "inventory_recount_required"})
	case errors.Is(err, stocktaking.ErrReserved):
		return c.Status(409).JSON(fiber.Map{"code": "inventory_below_reserved"})
	case errors.Is(err, stocktaking.ErrConflict):
		return c.Status(409).JSON(fiber.Map{"code": "inventory_conflict"})
	default:
		return catalogError(c, err)
	}
}
func inventoryQuery(c *fiber.Ctx, allowed ...string) bool {
	if c.Method() == "GET" && len(c.Body()) != 0 {
		return false
	}
	keys := map[string]bool{}
	for _, k := range allowed {
		keys[k] = true
	}
	seen := map[string]bool{}
	ok := true
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		s := string(k)
		if !keys[s] || seen[s] {
			ok = false
		}
		seen[s] = true
	})
	return ok
}
func inventoryPage(c *fiber.Ctx) (int, int, error) {
	values := []int{50, 0}
	for i, key := range []string{"limit", "offset"} {
		raw := c.Query(key)
		if raw == "" {
			if c.Context().QueryArgs().Has(key) {
				return 0, 0, stocktaking.ErrInvalid
			}
			continue
		}
		for _, r := range raw {
			if r < '0' || r > '9' {
				return 0, 0, stocktaking.ErrInvalid
			}
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, stocktaking.ErrInvalid
		}
		values[i] = n
	}
	if values[0] < 1 || values[0] > 100 || values[1] > 1000000 {
		return 0, 0, stocktaking.ErrInvalid
	}
	return values[0], values[1], nil
}
func inventoryBody(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 65536 {
		return c.SendStatus(413)
	}
	return nil
}
func decodeInventoryOpen(body []byte) (stocktaking.OpenInput, error) {
	var in stocktaking.OpenInput
	if err := decodeCash(body, &in, []string{"operation_id", "inventory_id", "location_id", "title", "items"}); err != nil {
		return in, err
	}
	if err := decodeInventoryItems(body, []string{"product_id", "unit"}); err != nil {
		return in, err
	}
	return in, nil
}
func decodeInventoryCount(body []byte) (stocktaking.CountInput, error) {
	var in stocktaking.CountInput
	if err := decodeCash(body, &in, []string{"operation_id", "expected_revision", "items"}); err != nil {
		return in, err
	}
	if err := decodeInventoryItems(body, []string{"product_id", "counted_milli"}); err != nil {
		return in, err
	}
	return in, nil
}
func decodeInventoryItems(body []byte, fields []string) error {
	var e struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return err
	}
	raw := bytes.TrimSpace(e.Items)
	if len(raw) == 0 || raw[0] != '[' {
		return stocktaking.ErrInvalid
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return err
	}
	if len(rows) < 1 || len(rows) > stocktaking.MaxLines {
		return stocktaking.ErrInvalid
	}
	for _, row := range rows {
		var item map[string]any
		if err := decodeCash(row, &item, fields); err != nil {
			return err
		}
	}
	return nil
}
func (s *Server) openReviewedInventory(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 65536 {
		return c.SendStatus(413)
	}
	in, err := decodeInventoryOpen(c.Body())
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.Open(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if err != nil {
		return inventoryError(c, err)
	}
	status := 201
	if r.Repeated {
		status = 200
	}
	return c.Status(status).JSON(r)
}
func (s *Server) countReviewedInventory(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 65536 {
		return c.SendStatus(413)
	}
	in, err := decodeInventoryCount(c.Body())
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.Count(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, c.Params("id"), in)
	if err != nil {
		return inventoryError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) stateReviewedInventory(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in stocktaking.StateInput
	if err := decodeCash(c.Body(), &in, []string{"operation_id", "expected_revision", "action", "reason"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.ChangeState(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, c.Params("id"), in)
	if err != nil {
		return inventoryError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) getReviewedInventory(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.Get(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return inventoryError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) receiptReviewedInventory(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.Receipt(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return inventoryError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) listReviewedInventory(c *fiber.Ctx) error {
	if !inventoryQuery(c, "status", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, err := inventoryPage(c)
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.List(c.UserContext(), s.DB, session.Actor, session.Device, c.Query("status"), limit, offset)
	if err != nil {
		return inventoryError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) historyReviewedInventory(c *fiber.Ctx) error {
	if !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, err := inventoryPage(c)
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.History(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), limit, offset)
	if err != nil {
		return inventoryError(c, err)
	}
	return c.JSON(r)
}
func (s *Server) exportReviewedInventory(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.Get(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return inventoryError(c, err)
	}
	var buf bytes.Buffer
	if err = stocktaking.Export(&buf, r); err != nil {
		return inventoryError(c, err)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="inventario.csv"`)
	return c.Send(buf.Bytes())
}

// SystemBuildRevision is supplied by -ldflags at packaging time. An unknown
// build remains explicitly unknown; the endpoint never claims database identity.
var SystemBuildRevision = "unknown"

func (s *Server) localSystemInfo(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	if err := identity.CanOperate(c.UserContext(), s.DB, session.Actor, session.Device, identity.ViewCatalog); err != nil {
		return catalogError(c, err)
	}
	var version int
	if err := s.DB.QueryRowContext(c.UserContext(), `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version); err != nil {
		return c.SendStatus(500)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"service": "titan-local", "build_revision": SystemBuildRevision, "schema_version": version, "reviewed_inventory_api": 1, "tenant_id": session.Actor.TenantID, "store_id": session.Actor.StoreID, "device_id": session.Device.DeviceID})
}
