package localapi

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"time"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/supplycycle"
)

func (s *Server) mountTrade(r fiber.Router) {
	g := r.Group("", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/company-trade/identity", s.tradeIdentity)
	g.Get("/company-trade/recovery", s.tradeRecovery)
	g.Post("/company-trade/recovery/review", s.tradeRecoveryReview)
	g.Get("/company-trade/messages", s.tradeMessages)
	g.Get("/company-trade/messages/:id", s.tradeMessage)
	g.Get("/company-trade/peers", s.tradePeers)
	g.Post("/company-trade/peers", limiter.New(limiter.Config{Max: 20, Expiration: time.Minute}), s.tradeTrust)
	g.Post("/company-trade/peers/:id/revoke", s.tradeRevoke)
	g.Post("/purchase-orders/:id/company-trade/export", s.tradeExport)
	g.Post("/company-trade/orders/import", s.tradeImport)
	g.Post("/company-trade/orders/:id/decision", s.tradeDecide)
	g.Post("/purchase-orders/:id/company-trade/decision", s.tradeApply)
	g.Get("/company-trade/operations/:id", s.tradeOperation)
}
func (s *Server) tradeIdentity(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(s.tradeSigner) != ed25519.PrivateKeySize {
		return c.SendStatus(503)
	}
	ss := c.Locals("session").(localauth.Session)
	if e := identity.CanOperate(c.UserContext(), s.DB, ss.Actor, ss.Device, identity.ViewOrders); e != nil {
		return cycleError(c, e)
	}
	public := s.tradeSigner.Public().(ed25519.PublicKey)
	return c.JSON(fiber.Map{"version": 1, "party": supplycycle.TradeParty{TenantID: ss.Device.TenantID, StoreID: ss.Device.StoreID, DeviceID: ss.Device.DeviceID}, "public_key": []byte(public), "fingerprint_sha256": supplycycle.TradeFingerprint(public), "business_identity_verified": false})
}
func (s *Server) tradeTrust(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.TradePeerInput
	if e := cycleDecode(c, &in, "operation_id", "id", "remote", "public_key", "fingerprint_sha256", "role", "supplier_id", "verification_reference"); e != nil {
		return e
	}
	if !tradeBytesField(c.Body(), "public_key") {
		return c.SendStatus(400)
	}
	var nested struct {
		Remote json.RawMessage `json:"remote"`
	}
	_ = json.Unmarshal(c.Body(), &nested)
	if decodeCash(nested.Remote, &in.Remote, []string{"tenant_id", "store_id", "device_id"}) != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.TrustTradePeer(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) tradeRevoke(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.TradeRevokeInput
	if e := cycleDecode(c, &in, "operation_id", "expected_revision", "reason"); e != nil {
		return e
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.RevokeTradePeer(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) tradeExport(c *fiber.Ctx) error {
	if s.contracts == nil || len(s.tradeSigner) != ed25519.PrivateKeySize {
		return c.SendStatus(503)
	}
	var in supplycycle.TradeExportInput
	if e := cycleDecode(c, &in, "operation_id", "peer_id", "reason"); e != nil {
		return e
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.ExportTradeOrder(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, s.tradeSigner, id, in)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func tradeImportDecode(c *fiber.Ctx, in *supplycycle.TradeImportInput) error {
	if e := cycleDecode(c, in, "operation_id", "peer_id", "envelope"); e != nil {
		return e
	}
	var nested struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	_ = json.Unmarshal(c.Body(), &nested)
	if !tradeBytesField(nested.Envelope, "payload") || !tradeBytesField(nested.Envelope, "signature") || decodeCash(nested.Envelope, &in.Envelope, []string{"payload", "signature"}) != nil {
		return fiber.ErrBadRequest
	}
	return nil
}
func (s *Server) tradeImport(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.TradeImportInput
	if e := tradeImportDecode(c, &in); e != nil {
		return e
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.ImportTradeOrder(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, in)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) tradeDecide(c *fiber.Ctx) error {
	if s.contracts == nil || len(s.tradeSigner) != ed25519.PrivateKeySize {
		return c.SendStatus(503)
	}
	var in supplycycle.TradeDecisionInput
	if e := cycleDecode(c, &in, "operation_id", "decision", "accepted_milli", "promised_at", "reason"); e != nil {
		return e
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.DecideTradeOrder(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, s.tradeSigner, id, in)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) tradeApply(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(503)
	}
	var in supplycycle.TradeImportInput
	if e := tradeImportDecode(c, &in); e != nil {
		return e
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.ApplyTradeDecision(c.UserContext(), s.DB, s.contracts, ss.Actor, ss.Device, id, in)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}
func (s *Server) tradeOperation(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	id, e := cycleID(c)
	if e != nil {
		return c.SendStatus(400)
	}
	ss := c.Locals("session").(localauth.Session)
	out, e := supplycycle.TradeOperation(c.UserContext(), s.DB, ss.Actor, ss.Device, id)
	if e != nil {
		return cycleError(c, e)
	}
	return c.JSON(out)
}

func tradeBytesField(raw []byte, name string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	value := bytes.TrimSpace(fields[name])
	return len(value) > 1 && value[0] == '"'
}
