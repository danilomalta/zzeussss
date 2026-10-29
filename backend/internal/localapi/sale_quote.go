package localapi

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func (s *Server) quoteSale(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 128*1024 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	values, err := exactSaleObject(c.Body(), []string{"cash_session_id", "items"}, "tax_quote_operations")
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if raw, ok := values["tax_quote_operations"]; ok {
		var refs []string
		if json.Unmarshal(raw, &refs) != nil || len(refs) == 0 || len(refs) > 100 {
			return c.SendStatus(400)
		}
	}
	var items []json.RawMessage
	if err = json.Unmarshal(values["items"], &items); err != nil || len(items) == 0 || len(items) > 500 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	for _, item := range items {
		if _, err = exactSaleObject(item, []string{"product_id", "location_id", "quantity_milli"}); err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}
	}
	var in sale.QuoteInput
	if err = json.Unmarshal(c.Body(), &in); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := sale.QuoteWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, in, session.Token)
	if err != nil {
		return saleError(c, err)
	}
	return c.JSON(result)
}
