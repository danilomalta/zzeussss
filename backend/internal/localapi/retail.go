package localapi

import (
	"database/sql"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/retail"
	"titansystem-backend/internal/sqlitewrite"
)

func (s *Server) retailService(c *fiber.Ctx) retail.Service {
	ss := c.Locals("session").(localauth.Session)
	return retail.Service{DB: s.DB, Contracts: s.contracts, Actor: ss.Actor, Device: ss.Device, Token: ss.Token}
}
func (s *Server) mountRetail(r fiber.Router) {
	s.mountDeferredFiscal(r)
	r.Get("/retail/access", s.readRetailAccess)
	r.Post("/retail/operations/:id/reconcile", s.reconcileRetailOperation)
	r.Post("/retail/stores", s.createRetailStore)
	r.Get("/retail/stores/report", s.retailStoreReport)
	r.Post("/retail/stores/transfer", s.retailStoreTransfer)
	r.Post("/retail/sales/:id/refund", s.refundRetailSale)
	r.Get("/retail/customers/:id/loyalty", s.readRetailPoints)
	r.Post("/retail/customers/:id/loyalty/earn", s.earnRetailPoints)
	r.Post("/retail/customers/:id/loyalty/redeem", s.redeemRetailPoints)
	r.Get("/retail/config", s.readRetailConfig)
	r.Post("/retail/config", s.configureRetail)
	r.Post("/retail/customers", s.saveRetailCustomer)
	r.Get("/retail/customers/:id", s.readRetailCustomer)
	r.Post("/retail/customers/:id/privacy", s.retailPrivacy)
}
func (s *Server) readRetail(c *fiber.Ctx, work func(*sql.Tx) (any, error), policy ...bool) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	svc := s.retailService(c)
	var out any
	e := sqlitewrite.Run(c.UserContext(), s.DB, retail.ErrConflict, func(tx *sql.Tx) error {
		var auth error
		if len(policy) > 0 && policy[0] {
			auth = svc.AuthorizePolicy(c.UserContext(), tx)
		} else {
			auth = svc.Authorize(c.UserContext(), tx)
		}
		if e := auth; e != nil {
			return e
		}
		if e := retail.Owner(c.UserContext(), tx, svc.Actor); e != nil {
			return e
		}
		var e error
		out, e = work(tx)
		return e
	})
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (s *Server) readRetailConfig(c *fiber.Ctx) error {
	return s.readRetail(c, func(tx *sql.Tx) (any, error) {
		return retail.ConfigTx(c.UserContext(), tx, s.retailService(c).Actor.TenantID)
	}, true)
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
	if _, e := exactSaleObject(raw["config"], []string{"revision", "credit_timezone", "credit_enabled", "refund_enabled", "loyalty_enabled", "multistore_enabled", "credit_roles", "refund_roles", "points_roles", "earn_cents", "earn_points", "redeem_points", "redeem_cents", "valid_days"}); e != nil {
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

func (s *Server) readRetailPoints(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	out, e := s.retailService(c).Points(c.UserContext(), c.Params("id"))
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (s *Server) earnRetailPoints(c *fiber.Ctx) error {
	var in retail.PointsInput
	if !receivableBody(c, []string{"operation_id", "sale_id", "confirmed"}, &in) {
		return c.SendStatus(400)
	}
	if e := s.retailService(c).Award(c.UserContext(), c.Params("id"), in); e != nil {
		return receivableError(c, e)
	}
	return c.SendStatus(204)
}
func (s *Server) redeemRetailPoints(c *fiber.Ctx) error {
	var in retail.PointsInput
	if !receivableBody(c, []string{"operation_id", "points", "expected_balance", "confirmed"}, &in) {
		return c.SendStatus(400)
	}
	if e := s.retailService(c).Redeem(c.UserContext(), c.Params("id"), in); e != nil {
		return receivableError(c, e)
	}
	return c.SendStatus(204)
}

func (s *Server) retailStoreTransfer(c *fiber.Ctx) error {
	var in retail.TransferInput
	if !receivableBody(c, []string{"operation_id", "destination_store_id", "destination_custodian_id", "product_id", "from_location_id", "to_location_id", "quantity_milli", "expected_from_milli", "expected_to_milli", "confirmed"}, &in) {
		return c.SendStatus(400)
	}
	if e := s.retailService(c).Transfer(c.UserContext(), in); e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.SendStatus(204)
}
func (s *Server) retailStoreReport(c *fiber.Ctx) error {
	if len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	valid := true
	seen := map[string]bool{}
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] || (key != "store_id" && key != "from_on" && key != "to_on" && key != "all_stores") {
			valid = false
		}
		seen[key] = true
	})
	all := c.Query("all_stores")
	if !valid || (all != "" && all != "true" && all != "false") {
		return c.SendStatus(400)
	}
	svc := s.retailService(c)
	store := c.Query("store_id")
	if store == "" && all != "true" {
		store = svc.Actor.StoreID
	}
	out, e := svc.StoreReport(c.UserContext(), store, c.Query("from_on"), c.Query("to_on"), all == "true")
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}

func (s *Server) createRetailStore(c *fiber.Ctx) error {
	var in struct {
		OperationID string `json:"operation_id"`
		ID          string `json:"id"`
		Name        string `json:"name"`
	}
	if !receivableBody(c, []string{"operation_id", "id", "name"}, &in) {
		return c.SendStatus(400)
	}
	if e := s.retailService(c).CreateStore(c.UserContext(), in.OperationID, in.ID, in.Name); e != nil {
		return receivableError(c, e)
	}
	return c.SendStatus(204)
}

func (s *Server) reconcileRetailOperation(c *fiber.Ctx) error {
	var in struct {
		Confirmed bool `json:"confirmed"`
	}
	if !receivableBody(c, []string{"confirmed"}, &in) {
		return c.SendStatus(400)
	}
	out, e := s.retailService(c).Reconcile(c.UserContext(), c.Params("id"), in.Confirmed)
	if e != nil {
		return receivableError(c, e)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}

// Policy hints contain no customer/secret data and never grant a command.
// Each command still checks the current contract, role, store and session.
func (s *Server) readRetailAccess(c *fiber.Ctx) error {
	if !noBagQuery(c) || len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	svc := s.retailService(c)
	tx, e := s.DB.BeginTx(c.UserContext(), nil)
	if e != nil {
		return receivableError(c, e)
	}
	defer tx.Rollback()
	if e = fiscal.ValidateSaleSessionTx(c.UserContext(), tx, svc.Actor, svc.Device, svc.Token); e != nil {
		return receivableError(c, e)
	}
	cfg, e := retail.ConfigTx(c.UserContext(), tx, svc.Actor.TenantID)
	if e != nil {
		return receivableError(c, e)
	}
	owner := retail.Owner(c.UserContext(), tx, svc.Actor) == nil
	role := func(roles []string) bool { return retail.RoleTx(c.UserContext(), tx, svc.Actor, roles) == nil }
	stock := identity.CanOperateTx(c.UserContext(), tx, svc.Actor, svc.Device, identity.ManageStock) == nil
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"tenant_id": svc.Actor.TenantID, "store_id": svc.Actor.StoreID, "device_id": svc.Device.DeviceID, "identity_id": svc.Actor.IdentityID, "can_configure": owner, "can_customer": owner, "can_credit": cfg.CreditEnabled && role(cfg.CreditRoles), "can_refund": cfg.RefundEnabled && role(cfg.RefundRoles), "can_points": cfg.LoyaltyEnabled && role(cfg.PointsRoles), "can_stores": cfg.MultistoreEnabled && stock})
}
