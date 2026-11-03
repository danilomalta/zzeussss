package localapi

import (
	"database/sql"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/retail"
)

func (s *Server) retailService(c *fiber.Ctx) retail.Service {
	ss := c.Locals("session").(localauth.Session)
	return retail.Service{DB: s.DB, Contracts: s.contracts, Actor: ss.Actor, Device: ss.Device, Token: ss.Token}
}
func (s *Server) mountRetail(r fiber.Router) {
	r.Post("/retail/sales/:id/refund", s.refundRetailSale)
	r.Get("/retail/config", s.readRetailConfig)
	r.Post("/retail/config", s.configureRetail)
	r.Post("/retail/customers", s.saveRetailCustomer)
	r.Get("/retail/customers/:id", s.readRetailCustomer)
	r.Post("/retail/customers/:id/privacy", s.retailPrivacy)
}
func (s *Server) readRetail(c *fiber.Ctx, work func(*sql.Tx) (any, error)) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	svc := s.retailService(c)
	tx, e := s.DB.BeginTx(c.UserContext(), nil)
	if e != nil {
		return receivableError(c, e)
	}
	defer tx.Rollback()
	if e = svc.Authorize(c.UserContext(), tx); e != nil {
		return receivableError(c, e)
	}
	if e = retail.Owner(c.UserContext(), tx, svc.Actor); e != nil {
		return receivableError(c, e)
	}
	out, e := work(tx)
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (s *Server) readRetailConfig(c *fiber.Ctx) error {
	return s.readRetail(c, func(tx *sql.Tx) (any, error) {
		return retail.ConfigTx(c.UserContext(), tx, s.retailService(c).Actor.TenantID)
	})
}
func (s *Server) readRetailCustomer(c *fiber.Ctx) error {
	if !retail.ID(c.Params("id")) {
		return c.SendStatus(400)
	}
	return s.readRetail(c, func(tx *sql.Tx) (any, error) {
		return retail.CustomerTx(c.UserContext(), tx, s.retailService(c).Actor.TenantID, c.Params("id"))
	})
}
func (s *Server) configureRetail(c *fiber.Ctx) error {
	var body struct {
		OperationID string        `json:"operation_id"`
		Config      retail.Config `json:"config"`
	}
	if !receivableBody(c, []string{"operation_id", "config"}, &body) {
		return c.SendStatus(400)
	}
	var raw map[string]json.RawMessage
	json.Unmarshal(c.Body(), &raw)
	if _, e := exactSaleObject(raw["config"], []string{"revision", "credit_enabled", "refund_enabled", "loyalty_enabled", "multistore_enabled", "credit_roles", "refund_roles", "points_roles", "earn_cents", "earn_points", "redeem_points", "redeem_cents", "valid_days"}); e != nil {
		return c.SendStatus(400)
	}
	if e := s.retailService(c).Configure(c.UserContext(), body.OperationID, body.Config); e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(204)
}
func (s *Server) saveRetailCustomer(c *fiber.Ctx) error {
	var body struct {
		OperationID string          `json:"operation_id"`
		Customer    retail.Customer `json:"customer"`
	}
	if !receivableBody(c, []string{"operation_id", "customer"}, &body) {
		return c.SendStatus(400)
	}
	var raw map[string]json.RawMessage
	json.Unmarshal(c.Body(), &raw)
	if _, e := exactSaleObject(raw["customer"], []string{"id", "revision", "name", "cpf", "phone", "consent_reference", "credit_limit_cents", "block_overdue", "credit_enabled", "loyalty_consent"}); e != nil {
		return c.SendStatus(400)
	}
	if e := s.retailService(c).SaveCustomer(c.UserContext(), body.OperationID, body.Customer); e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(204)
}
func (s *Server) retailPrivacy(c *fiber.Ctx) error {
	var body struct {
		OperationID string `json:"operation_id"`
		Revision    int64  `json:"revision"`
		Anonymize   bool   `json:"anonymize"`
		Confirmed   bool   `json:"confirmed"`
	}
	if !receivableBody(c, []string{"operation_id", "revision", "anonymize", "confirmed"}, &body) || !body.Confirmed {
		return c.SendStatus(400)
	}
	out, e := s.retailService(c).Privacy(c.UserContext(), body.OperationID, c.Params("id"), body.Revision, body.Anonymize)
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}

func (s *Server) refundRetailSale(c *fiber.Ctx) error {
	var in retail.RefundInput
	if !receivableBody(c, []string{"operation_id", "item_id", "quantity_milli", "expected_quantity_milli", "expected_cents", "session_id", "reason"}, &in) {
		return c.SendStatus(400)
	}
	if e := s.retailService(c).RefundSale(c.UserContext(), c.Params("id"), in); e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(204)
}

// This opt-in credit flow has no inferred fiscal payment code. No emitter is called.
func (s *Server) rejectCreditFiscal(c *fiber.Ctx) (bool, error) {
	ss := c.Locals("session").(localauth.Session)
	var n int
	e := s.DB.QueryRowContext(c.UserContext(), `SELECT count(*) FROM receivable_credit_sales c JOIN sales s ON s.tenant_id=c.tenant_id AND s.id=c.sale_id WHERE c.tenant_id=? AND c.store_id=? AND s.device_id=? AND c.sale_id=?`, ss.Actor.TenantID, ss.Actor.StoreID, ss.Device.DeviceID, c.Params("id")).Scan(&n)
	if e != nil {
		return true, receivableError(c, e)
	}
	if n != 0 {
		c.Set("Cache-Control", "no-store")
		return true, c.Status(409).JSON(fiber.Map{"code": "credit_fiscal_payment_not_mapped"})
	}
	return false, nil
}
