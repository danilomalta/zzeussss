package localapi

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func (s *Server) cancelSale(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if len(c.Body()) > 8*1024 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	if _, err := exactSaleObject(c.Body(), []string{"operation_id", "reason"}); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	var input struct {
		OperationID string `json:"operation_id"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := sale.CancelWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device,
		sale.CancelInput{OperationID: input.OperationID, SaleID: c.Params("id"), Reason: input.Reason})
	if errors.Is(err, sale.ErrCancellationBalance) {
		return c.SendStatus(fiber.StatusConflict)
	}
	if err != nil {
		return saleError(c, err)
	}
	return c.JSON(result)
}
