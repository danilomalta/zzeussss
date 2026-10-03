package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchases(r fiber.Router) {
	r.Get("/purchase-suppliers", s.purchaseSuppliers)
	r.Post("/purchase-suppliers", s.createPurchaseSupplier)
	r.Get("/purchase-approvals", s.purchaseApprovals)
	r.Get("/purchase-orders", s.purchaseOrders)
	r.Get("/purchase-orders/:id", s.purchaseOrder)
	r.Post("/purchase-orders", s.createPurchaseOrder)
}
func purchaseOffset(c *fiber.Ctx) (int, error) {
	v := c.Query("offset", "0")
	n, e := strconv.Atoi(v)
	if e != nil || n < 0 || len(v) > 10 {
		return 0, purchases.ErrInvalid
	}
	return n, nil
}
func purchaseError(c *fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, purchases.ErrInvalid):
		return c.SendStatus(400)
	case errors.Is(e, purchases.ErrConflict), errors.Is(e, purchases.ErrApproval):
		return c.SendStatus(409)
	case errors.Is(e, purchases.ErrNotFound):
		return c.SendStatus(404)
	default:
		return catalogError(c, e)
	}
}
func (s *Server) createPurchaseSupplier(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in purchases.SupplierInput
	if e := decodeCash(c.Body(), &in, []string{"operation_id", "id", "name", "status"}); e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, e := purchases.CreateSupplier(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if e != nil {
		return purchaseError(c, e)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}
func (s *Server) createPurchaseOrder(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in purchases.Input
	if e := decodeCash(c.Body(), &in, []string{"operation_id", "order_id", "supplier_id", "suggestion_id"}); e != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, e := purchases.Create(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in)
	if e != nil {
		return purchaseError(c, e)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}
func (s *Server) purchaseSuppliers(c *fiber.Ctx) error {
	n, e := purchaseOffset(c)
	if e != nil {
		return purchaseError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	v, e := purchases.Suppliers(c.UserContext(), s.DB, session.Actor, session.Device, n)
	if e != nil {
		return purchaseError(c, e)
	}
	return c.JSON(fiber.Map{"items": v})
}
func (s *Server) purchaseApprovals(c *fiber.Ctx) error {
	n, e := purchaseOffset(c)
	if e != nil {
		return purchaseError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	v, e := purchases.Approvals(c.UserContext(), s.DB, session.Actor, session.Device, n)
	if e != nil {
		return purchaseError(c, e)
	}
	return c.JSON(fiber.Map{"items": v})
}
func (s *Server) purchaseOrders(c *fiber.Ctx) error {
	n, e := purchaseOffset(c)
	if e != nil {
		return purchaseError(c, e)
	}
	session := c.Locals("session").(localauth.Session)
	v, e := purchases.Orders(c.UserContext(), s.DB, session.Actor, session.Device, n)
	if e != nil {
		return purchaseError(c, e)
	}
	return c.JSON(fiber.Map{"items": v})
}
func (s *Server) purchaseOrder(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	v, e := purchases.Get(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if e != nil {
		return purchaseError(c, e)
	}
	return c.JSON(v)
}
