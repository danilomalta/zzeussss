package localapi

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func (s *Server) mountCustomerReturns(r fiber.Router) {
	r.Get("/sales/:id/returns", s.customerReturnHistory)
	r.Post("/sales/:id/returns/quote", s.customerReturnQuote)
	r.Post("/sales/:id/returns/cash-session/quote", s.customerReturnSessionQuote)
	r.Post("/sales/:id/returns/cash-session", s.customerReturnSessionCreate)
	r.Post("/sales/:id/returns", s.customerReturnCreate)
	r.Get("/sale-return-operations/:id", s.customerReturnOperation)
}

func customerReturnError(c *fiber.Ctx, err error) error {
	var sqliteCode interface{ Code() int }
	if errors.As(err, &sqliteCode) && (sqliteCode.Code()&255 == 5 || sqliteCode.Code()&255 == 6) {
		return c.SendStatus(409)
	}
	if errors.Is(err, sale.ErrReturnRange) {
		return c.SendStatus(422)
	}
	if errors.Is(err, sale.ErrCancellationBalance) {
		return c.SendStatus(409)
	}
	return saleError(c, err)
}
func (s *Server) customerReturnHistory(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	a := c.Locals("session").(localauth.Session)
	v, err := sale.ReadCustomerReturns(c.UserContext(), s.DB, a.Actor, a.Device, c.Params("id"))
	if err != nil {
		return customerReturnError(c, err)
	}
	return c.JSON(v)
}
func (s *Server) customerReturnOperation(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	a := c.Locals("session").(localauth.Session)
	v, err := sale.ReadCustomerReturnOperation(c.UserContext(), s.DB, a.Actor, a.Device, c.Params("id"))
	if err != nil {
		return customerReturnError(c, err)
	}
	return c.JSON(v)
}
func decodeReturnItems(raw json.RawMessage) ([]sale.ReturnItemInput, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 || len(items) > 500 {
		return nil, sale.ErrInvalid
	}
	for _, i := range items {
		if _, err := exactSaleObject(i, []string{"sale_item_id", "quantity_milli", "disposition"}); err != nil {
			return nil, err
		}
	}
	var typed []sale.ReturnItemInput
	if err := json.Unmarshal(raw, &typed); err != nil {
		return nil, err
	}
	return typed, nil
}
func (s *Server) customerReturnQuote(c *fiber.Ctx) error {
	return s.customerReturnQuoteForSession(c, false)
}
func (s *Server) customerReturnSessionQuote(c *fiber.Ctx) error {
	return s.customerReturnQuoteForSession(c, true)
}
func (s *Server) customerReturnQuoteForSession(c *fiber.Ctx, explicit bool) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 128*1024 {
		return c.SendStatus(413)
	}
	fields := []string{"items"}
	if explicit {
		fields = append(fields, "refund_cash_session_id")
	}
	values, err := exactSaleObject(c.Body(), fields)
	if err != nil {
		return c.SendStatus(400)
	}
	items, err := decodeReturnItems(values["items"])
	if err != nil {
		return c.SendStatus(400)
	}
	a := c.Locals("session").(localauth.Session)
	var v sale.CustomerReturnQuote
	if explicit {
		var id string
		if json.Unmarshal(values["refund_cash_session_id"], &id) != nil {
			return c.SendStatus(400)
		}
		v, err = sale.QuoteCustomerReturnWithSession(c.UserContext(), s.DB, a.Actor, a.Device, c.Params("id"), items, id)
	} else {
		v, err = sale.QuoteCustomerReturn(c.UserContext(), s.DB, a.Actor, a.Device, c.Params("id"), items)
	}
	if err != nil {
		return customerReturnError(c, err)
	}
	return c.JSON(v)
}
func (s *Server) customerReturnCreate(c *fiber.Ctx) error {
	return s.customerReturnCreateForSession(c, false)
}
func (s *Server) customerReturnSessionCreate(c *fiber.Ctx) error {
	return s.customerReturnCreateForSession(c, true)
}
func (s *Server) customerReturnCreateForSession(c *fiber.Ctx, explicit bool) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 128*1024 {
		return c.SendStatus(413)
	}
	fields := []string{"operation_id", "reason", "expected_refund_cents", "items"}
	if explicit {
		fields = append(fields, "refund_cash_session_id")
	}
	values, err := exactSaleObject(c.Body(), fields)
	if err != nil {
		return c.SendStatus(400)
	}
	if _, err = decodeReturnItems(values["items"]); err != nil {
		return c.SendStatus(400)
	}
	var in sale.CustomerReturnInput
	if err = json.Unmarshal(c.Body(), &in); err != nil {
		return c.SendStatus(400)
	}
	if explicit && in.RefundCashSessionID == "" {
		return c.SendStatus(400)
	}
	in.SaleID = c.Params("id")
	a := c.Locals("session").(localauth.Session)
	v, err := sale.CreateCustomerReturn(c.UserContext(), s.DB, s.contracts, a.Actor, a.Device, in)
	if err != nil {
		return customerReturnError(c, err)
	}
	status := 201
	if v.Repeated {
		status = 200
	}
	return c.Status(status).JSON(v)
}
