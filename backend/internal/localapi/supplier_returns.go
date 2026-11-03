package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"net/url"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/stock"
)

func (s *Server) mountSupplierReturns(r fiber.Router) {
	g := r.Group("", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Post("/purchase-receiving-cases/:id/returns", s.supplierReturnOpen)
	g.Get("/purchase-supplier-returns", s.supplierReturnList)
	g.Get("/purchase-supplier-returns/:id", s.supplierReturnGet)
	g.Get("/purchase-supplier-returns/:id/history", s.supplierReturnHistory)
	g.Post("/purchase-supplier-returns/:id/actions", s.supplierReturnAction)
	g.Post("/purchase-supplier-returns/:id/dispatch", s.supplierReturnStock)
	g.Post("/purchase-supplier-returns/:id/recover", s.supplierReturnStock)
	g.Get("/purchase-supplier-return-operations/:id", s.supplierReturnOperation)
	g.Get("/finance/receiving-return-review/:id", s.supplierReturnFinance)
}
func supplierReturnError(c *fiber.Ctx, e error) error {
	if errors.Is(e, purchases.ErrReturnLimit) {
		return c.Status(422).JSON(fiber.Map{"code": "supplier_return_source_limit", "max_returns": purchases.MaxSupplierReturns, "max_events": purchases.MaxReturnEvents})
	}
	if errors.Is(e, stock.ErrInsufficientStock) || errors.Is(e, stock.ErrOperationConflict) || errors.Is(e, stock.ErrStockOverflow) {
		return c.SendStatus(409)
	}
	if errors.Is(e, stock.ErrInvalidOperation) {
		return c.SendStatus(400)
	}
	var code interface{ Code() int }
	if errors.As(e, &code) && (code.Code() == 1555 || code.Code() == 2067) {
		return c.SendStatus(409)
	}
	return purchaseError(c, e)
}
func returnDecode(c *fiber.Ctx, in any, fields ...string) error {
	if !inventoryQuery(c) {
		return fiber.ErrBadRequest
	}
	if len(c.Body()) > 8192 {
		return fiber.ErrRequestEntityTooLarge
	}
	if decodeCash(c.Body(), in, fields) != nil {
		return fiber.ErrBadRequest
	}
	return nil
}
func (s *Server) supplierReturnOpen(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in purchases.ReturnOpenInput
	if e := returnDecode(c, &in, "operation_id", "return_id", "receipt_id", "quantity_milli", "reason"); e != nil {
		return e
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := purchases.OpenSupplierReturn(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	if e != nil {
		return supplierReturnError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) supplierReturnAction(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in purchases.ReturnActionInput
	if e := returnDecode(c, &in, "operation_id", "expected_revision", "action", "quantity_milli", "contact", "evidence_reference", "observed_at", "collection_mode", "paired_delivery_id", "route_reference", "deadline", "financial_instruction", "reason"); e != nil {
		return e
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := purchases.ActSupplierReturn(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	if e != nil {
		return supplierReturnError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) supplierReturnStock(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in purchases.ReturnStockInput
	if e := returnDecode(c, &in, "operation_id", "expected_revision", "location_id", "quantity_milli", "handover_reference", "observed_at", "reason"); e != nil {
		return e
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	action := "dispatch"
	if c.Route().Path == "/local/v1/purchase-supplier-returns/:id/recover" {
		action = "recover"
	}
	r, e := purchases.SupplierReturnStock(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, action, in)
	if e != nil {
		return supplierReturnError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) supplierReturnList(c *fiber.Ctx) error {
	if !inventoryQuery(c, "case_id", "order_id", "supplier_id", "state", "limit", "offset") {
		return c.SendStatus(400)
	}
	for _, k := range []string{"case_id", "order_id", "supplier_id", "state"} {
		if c.Context().QueryArgs().Has(k) && c.Query(k) == "" {
			return c.SendStatus(400)
		}
	}
	limit, offset, e := inventoryPage(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := purchases.ListSupplierReturns(c.UserContext(), s.DB, ss.Actor, ss.Device, c.Query("case_id"), c.Query("order_id"), c.Query("supplier_id"), c.Query("state"), limit, offset)
	if e != nil {
		return supplierReturnError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) supplierReturnGet(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := purchases.GetSupplierReturn(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return supplierReturnError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) supplierReturnHistory(c *fiber.Ctx) error {
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
	ss := c.Locals("session").(localauth.Session)
	r, e := purchases.SupplierReturnHistory(c.UserContext(), s.DB, ss.Actor, ss.Device, id, limit, offset)
	if e != nil {
		return supplierReturnError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) supplierReturnOperation(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := purchases.SupplierReturnOperation(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return supplierReturnError(c, e)
	}
	return c.JSON(r)
}
func (s *Server) supplierReturnFinance(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := url.PathUnescape(c.Params("id"))
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	r, e := payables.ReceivingReturns(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		if errors.Is(e, purchases.ErrReturnLimit) || errors.Is(e, purchases.ErrConflict) {
			return supplierReturnError(c, e)
		}
		return payableError(c, e)
	}
	return c.JSON(r)
}
