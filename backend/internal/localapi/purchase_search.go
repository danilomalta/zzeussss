package localapi

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

func (s *Server) mountPurchaseSearch(r fiber.Router) {
	r.Get("/purchase-order-search", s.purchaseSearch)
}
func (s *Server) purchaseSearch(c *fiber.Ctx) error {
	filter := purchases.SearchFilter{}
	offset := int64(0)
	valid := true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key, value := string(k), string(v)
		if seen[key] || value == "" {
			valid = false
			return
		}
		seen[key] = true
		switch key {
		case "supplier_id":
			filter.SupplierID = value
		case "product_id":
			filter.ProductID = value
		case "offset":
			for _, ch := range value {
				if ch < '0' || ch > '9' {
					valid = false
				}
			}
			var err error
			offset, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				valid = false
			}
		default:
			valid = false
		}
	})
	if !valid {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := purchases.Search(c.UserContext(), s.DB, session.Actor, session.Device, filter, offset)
	if err != nil {
		return purchaseError(c, err)
	}
	return c.JSON(out)
}
