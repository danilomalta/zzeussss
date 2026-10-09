package localapi

import (
	"github.com/gofiber/fiber/v2"
	"net/url"
	"strconv"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseReceivingHistory(r fiber.Router) {
	r.Get("/purchase-orders/:id/receiving-history", s.purchaseReceivingHistory)
}
func (s *Server) purchaseReceivingHistory(c *fiber.Ctx) error {
	args := c.Context().QueryArgs()
	valid := true
	count := 0
	var offset int64
	args.VisitAll(func(k, v []byte) {
		count++
		if string(k) != "offset" || len(v) == 0 || len(v) > 16 {
			valid = false
			return
		}
		for _, b := range v {
			if b < '0' || b > '9' {
				valid = false
				return
			}
		}
		n, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil || n > purchases.MaxQuantity {
			valid = false
			return
		}
		offset = n
	})
	if !valid || count > 1 {
		return c.SendStatus(400)
	}
	id, err := url.PathUnescape(c.Params("id"))
	if err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.ReceivingHistory(c.UserContext(), s.DB, session.Actor, session.Device, id, offset)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
