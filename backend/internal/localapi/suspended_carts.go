package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func (s *Server) mountCarts(r fiber.Router) {
	r.Post("/pos/carts", s.saveCart)
	r.Get("/pos/carts", s.listCarts)
	r.Get("/pos/carts/report", s.cartReport)
	r.Get("/pos/carts/report.csv", s.cartReportCSV)
	r.Get("/pos/carts/:id/reconcile", s.reconcileCart)
	r.Get("/pos/carts/:id", s.getCart)
	r.Post("/pos/carts/:id/actions", s.actCart)
	r.Get("/pos/carts/:id/history", s.cartHistory)
	r.Get("/pos/cart-operations/:id", s.cartOperation)
}
func cartAPIError(c *fiber.Ctx, e error) error {
	if errors.Is(e, sale.ErrCartReportLimit) {
		return c.Status(422).JSON(fiber.Map{"code": "cart_report_source_limit", "max_carts": sale.MaxCartReportRows, "max_events": sale.MaxCartReportEvents})
	}
	if errors.Is(e, sale.ErrCartLimit) {
		return c.Status(422).JSON(fiber.Map{"code": "cart_revision_limit", "max_revisions": 4096})
	}
	var code interface{ Code() int }
	if errors.As(e, &code) && (code.Code() == 1555 || code.Code() == 2067) {
		return c.SendStatus(409)
	}
	return saleError(c, e)
}
func cartID(c *fiber.Ctx) (string, error) {
	c.Set("Cache-Control", "no-store")
	return url.PathUnescape(c.Params("id"))
}
func (s *Server) saveCart(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 128*1024 {
		return c.SendStatus(413)
	}
	fields, e := exactSaleObject(c.Body(), []string{"operation_id", "cart_id", "expected_revision", "label", "items"})
	if e != nil {
		return c.SendStatus(400)
	}
	var items []json.RawMessage
	if json.Unmarshal(fields["items"], &items) != nil || len(items) < 1 || len(items) > 500 {
		return c.SendStatus(400)
	}
	for _, item := range items {
		if _, e = exactSaleObject(item, []string{"product_id", "location_id", "quantity_milli"}); e != nil {
			return c.SendStatus(400)
		}
	}
	var in sale.CartSaveInput
	if json.Unmarshal(c.Body(), &in) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := sale.SaveCart(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in)
	if e != nil {
		return cartAPIError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) actCart(c *fiber.Ctx) error {
	id, e := cartID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	if _, e = exactSaleObject(c.Body(), []string{"operation_id", "expected_revision", "action", "cash_session_id", "received_cents", "reason"}); e != nil {
		return c.SendStatus(400)
	}
	var in sale.CartActionInput
	if json.Unmarshal(c.Body(), &in) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := sale.ActCart(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	if e != nil {
		return cartAPIError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) getCart(c *fiber.Ctx) error {
	id, e := cartID(c)
	if e != nil || !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := sale.GetCart(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return cartAPIError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) cartOperation(c *fiber.Ctx) error {
	id, e := cartID(c)
	if e != nil || !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := sale.CartOperation(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return cartAPIError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) listCarts(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if !inventoryQuery(c, "state", "limit", "offset") || c.Context().QueryArgs().Has("state") && c.Query("state") == "" {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := sale.ListCarts(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Query("state"), limit, offset)
	if e != nil {
		return cartAPIError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) cartHistory(c *fiber.Ctx) error {
	id, e := cartID(c)
	if e != nil || !inventoryQuery(c, "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := sale.CartEvents(c.UserContext(), s.DB, ss.Actor, ss.Device, id, limit, offset)
	if e != nil {
		return cartAPIError(c, e)
	}
	return c.JSON(out)
}

func (s *Server) reconcileCart(c *fiber.Ctx) error {
	id, e := cartID(c)
	if e != nil || !inventoryQuery(c, "cash_session_id") || c.Query("cash_session_id") == "" {
		return c.SendStatus(400)
	}
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := sale.ReconcileCart(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, c.Query("cash_session_id"))
	if e != nil {
		return cartAPIError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) cartReport(c *fiber.Ctx) error    { return s.cartReportReply(c, false) }
func (s *Server) cartReportCSV(c *fiber.Ctx) error { return s.cartReportReply(c, true) }
func (s *Server) cartReportReply(c *fiber.Ctx, csv bool) error {
	c.Set("Cache-Control", "no-store")
	if !inventoryQuery(c, "from", "to") {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := sale.ReadCartReport(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Query("from"), c.Query("to"))
	if e != nil {
		return cartAPIError(c, e)
	}
	if !csv {
		return c.JSON(out)
	}
	var b bytes.Buffer
	if e = sale.WriteCartReportCSV(&b, out); e != nil {
		return cartAPIError(c, e)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="carrinhos_backend.csv"`)
	return c.Send(b.Bytes())
}
