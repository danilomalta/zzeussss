package localapi

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/stock"
)

func stockReadError(c *fiber.Ctx, err error) error {
	if errors.Is(err, stock.ErrOperationMissing) {
		return c.SendStatus(404)
	}
	if errors.Is(err, stock.ErrAvailability) {
		return c.SendStatus(409)
	}
	return stockError(c, err)
}
func (s *Server) stockOperationReceipt(c *fiber.Ctx) error {
	if c.Context().QueryArgs().Len() != 0 {
		return c.SendStatus(400)
	}
	op, err := supplierPath(c)
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := stock.Receipt(c.UserContext(), s.DB, session.Actor, session.Device, op)
	if err != nil {
		return stockReadError(c, err)
	}
	return c.JSON(v)
}
func (s *Server) stockOperationHistory(c *fiber.Ctx) error {
	values := map[string]string{}
	ok := true
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if _, seen := values[key]; seen || len(v) == 0 {
			ok = false
		}
		switch key {
		case "product_id", "location_id", "offset":
		default:
			ok = false
		}
		values[key] = string(v)
	})
	offset := int64(0)
	if value, exists := values["offset"]; exists {
		for _, c := range value {
			if c < '0' || c > '9' {
				ok = false
			}
		}
		var err error
		offset, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			ok = false
		}
	}
	if !ok {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	v, err := stock.History(c.UserContext(), s.DB, session.Actor, session.Device, values["product_id"], values["location_id"], offset)
	if err != nil {
		return stockReadError(c, err)
	}
	return c.JSON(v)
}
