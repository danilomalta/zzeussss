package localapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stock"
)

func (s *Server) mountStock(router fiber.Router) {
	router.Get("/stock/balance", s.stockBalance)
	router.Post("/stock/operations", s.recordStock)
}

func (s *Server) stockBalance(c *fiber.Ctx) error {
	product, location := c.Query("product_id"), c.Query("location_id")
	if product == "" || location == "" || len(product) > 128 || len(location) > 128 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	balance, err := stock.Balance(c.UserContext(), s.DB, session.Actor, session.Device, product, location)
	if err != nil {
		return stockError(c, err)
	}
	return c.JSON(fiber.Map{"product_id": product, "location_id": location, "quantity_milli": balance})
}

func (s *Server) recordStock(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 16*1024 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	input, err := decodeStockInput(c.Body())
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := stock.RecordWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, input)
	if err != nil {
		return stockError(c, err)
	}
	status := fiber.StatusCreated
	if result.Repeated {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(result)
}

func stockError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, stock.ErrInvalidOperation):
		return c.SendStatus(fiber.StatusBadRequest)
	case errors.Is(err, stock.ErrOperationConflict), errors.Is(err, stock.ErrInsufficientStock), errors.Is(err, stock.ErrStockOverflow):
		return c.SendStatus(fiber.StatusConflict)
	default:
		return catalogError(c, err)
	}
}

func decodeStockInput(body []byte) (stock.Input, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return stock.Input{}, stock.ErrInvalidOperation
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return stock.Input{}, stock.ErrInvalidOperation
		}
		switch name {
		case "operation_id", "kind", "product_id", "from_location_id", "to_location_id", "quantity_milli", "reason":
		default:
			return stock.Input{}, stock.ErrInvalidOperation
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return stock.Input{}, stock.ErrInvalidOperation
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return stock.Input{}, stock.ErrInvalidOperation
	}
	if _, err := decoder.Token(); err != io.EOF {
		return stock.Input{}, stock.ErrInvalidOperation
	}
	var input stock.Input
	if err := json.Unmarshal(body, &input); err != nil {
		return stock.Input{}, stock.ErrInvalidOperation
	}
	return input, nil
}
