package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func (s *Server) mountSales(router fiber.Router) {
	router.Post("/sales/:id/cancel", s.cancelSale)
	router.Post("/sales", s.completeSale)
	router.Get("/sales/:id", s.readSale)
}

func (s *Server) completeSale(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 128*1024 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	input, err := decodeSale(c.Body())
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := sale.CompleteWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, input)
	if err != nil {
		return saleError(c, err)
	}
	status := fiber.StatusCreated
	if result.Repeated {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(result)
}

func (s *Server) readSale(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	result, err := sale.Read(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return saleError(c, err)
	}
	return c.JSON(result)
}

func saleError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, sale.ErrInvalid):
		return c.SendStatus(fiber.StatusBadRequest)
	case errors.Is(err, sale.ErrNotFound):
		return c.SendStatus(fiber.StatusNotFound)
	case errors.Is(err, sale.ErrNoStock), errors.Is(err, sale.ErrConflict), errors.Is(err, sale.ErrCashClosed),
		errors.Is(err, sale.ErrCashOverflow), errors.Is(err, sale.ErrPaymentUnverified):
		return c.SendStatus(fiber.StatusConflict)
	default:
		return catalogError(c, err)
	}
}

// exactSaleObject rejects duplicate/extra/missing/null fields, including in
// nested items and payments. Numbers are subsequently decoded into int64.
func exactSaleObject(body []byte, fields []string) (map[string]json.RawMessage, error) {
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[field] = true
	}
	values := make(map[string]json.RawMessage, len(fields))
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, sale.ErrInvalid
	}
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || !allowed[name] {
			return nil, sale.ErrInvalid
		}
		if _, duplicate := values[name]; duplicate {
			return nil, sale.ErrInvalid
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, sale.ErrInvalid
		}
		values[name] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(values) != len(fields) {
		return nil, sale.ErrInvalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, sale.ErrInvalid
	}
	return values, nil
}

func decodeSale(body []byte) (sale.Input, error) {
	values, err := exactSaleObject(body, []string{"operation_id", "sale_id", "cash_session_id", "items", "payments"})
	if err != nil {
		return sale.Input{}, err
	}
	for _, check := range []struct {
		name   string
		fields []string
		max    int
	}{
		{"items", []string{"product_id", "location_id", "quantity_milli"}, 500},
		{"payments", []string{"method", "amount_cents"}, 8},
	} {
		var entries []json.RawMessage
		if err := json.Unmarshal(values[check.name], &entries); err != nil || len(entries) == 0 || len(entries) > check.max {
			return sale.Input{}, sale.ErrInvalid
		}
		for _, entry := range entries {
			if _, err := exactSaleObject(entry, check.fields); err != nil {
				return sale.Input{}, err
			}
		}
	}
	var input sale.Input
	if err := json.Unmarshal(body, &input); err != nil {
		return sale.Input{}, sale.ErrInvalid
	}
	return input, nil
}
