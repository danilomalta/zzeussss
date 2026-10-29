// Package localapi expõe o núcleo SQLite com sessão e contexto de estação.
// O processo configura loopback ou servidor de loja com TLS e restrição de rede.
// New recebe um DeviceContext já provado pelo processo de inicialização.
package localapi

import (
	"bytes"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"io"
	"log"
	"strings"
	"time"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/fiscalproduction"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

type Server struct {
	DB          *sql.DB
	Device      identity.DeviceContext
	contracts   *entitlementstore.Store
	tradeSigner ed25519.PrivateKey
	production  *fiscalproduction.Runtime
}

// New preserves authentication and authorized reads without issuer keys.
// Catalog mutations are unavailable until a trusted verifier is configured.
func New(db *sql.DB, device identity.DeviceContext) (*fiber.App, error) {
	return NewWithVerifier(db, device, nil)
}

// NewWithVerifier receives trust from process configuration, never HTTP input.
// The device must already be proven by startup; user sessions are checked per request.
func NewWithVerifier(db *sql.DB, device identity.DeviceContext, verifier *entitlements.Verifier) (*fiber.App, error) {
	return NewWithVerifierAndGate(db, device, verifier, nil)
}

// NewWithVerifierAndGate installs transport admission BEFORE all routes.
// Session, device, role, contract and record checks still apply afterwards.
func NewWithVerifierAndGate(db *sql.DB, device identity.DeviceContext, verifier *entitlements.Verifier, gate fiber.Handler) (*fiber.App, error) {
	return NewWithTradeSignerAndGate(db, device, verifier, gate, nil)
}

// Signing key comes only from verified process configuration, never HTTP.
func NewWithTradeSignerAndGate(db *sql.DB, device identity.DeviceContext, verifier *entitlements.Verifier, gate fiber.Handler, signer ed25519.PrivateKey) (*fiber.App, error) {
	return NewWithProductionAndGate(db, device, verifier, gate, signer, nil)
}

func NewWithProductionAndGate(db *sql.DB, device identity.DeviceContext, verifier *entitlements.Verifier, gate fiber.Handler, signer ed25519.PrivateKey, production *fiscalproduction.Runtime) (*fiber.App, error) {
	if db == nil || device.TenantID == "" || device.StoreID == "" || device.DeviceID == "" {
		return nil, errors.New("servidor local sem aparelho")
	}
	if production != nil {
		if verifier == nil {
			return nil, fiscalproduction.ErrUnavailable
		}
		b := production.Binding()
		if production.Validate() != nil || b.TenantID != device.TenantID || b.StoreID != device.StoreID || b.DeviceID != device.DeviceID {
			return nil, fiscalproduction.ErrUnavailable
		}
	}
	s := &Server{DB: db, Device: device, production: production}
	if signer != nil {
		if len(signer) != ed25519.PrivateKeySize {
			return nil, errors.New("trade signing identity unavailable")
		}
		var public []byte
		if err := db.QueryRow(`SELECT public_key FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, device.TenantID, device.StoreID, device.DeviceID).Scan(&public); err != nil || !bytes.Equal(public, signer.Public().(ed25519.PublicKey)) {
			return nil, errors.New("trade signing identity unavailable")
		}
		s.tradeSigner = append(ed25519.PrivateKey(nil), signer...)
	}
	if verifier != nil {
		contracts, err := entitlementstore.New(db, verifier, nil)
		if err != nil {
			return nil, err
		}
		s.contracts = contracts
	}
	config := fiber.Config{AppName: "Titan Local", DisableStartupMessage: true, BodyLimit: 1 << 20}
	if gate != nil {
		config.ReadTimeout = 15 * time.Second
		config.WriteTimeout = 15 * time.Second
		config.IdleTimeout = time.Minute
		config.Concurrency = 128
	}
	if production != nil {
		config.WriteTimeout = 45 * time.Second
	}
	app := fiber.New(config)
	app.Use(apicontract.Errors())
	if gate != nil {
		// fasthttp's parser/connection diagnostics may include raw request
		// fragments. Structured, redacted operational telemetry is separate.
		app.Server().Logger = log.New(io.Discard, "", 0)
	}
	if gate != nil {
		app.Use(gate)
	}
	// Includes denied/expired fiscal requests: authentication errors must not
	// become cacheable merely because they never reached the route handler.
	app.Use("/local/v1/fiscal", func(c *fiber.Ctx) error { c.Set(fiber.HeaderCacheControl, "no-store"); return c.Next() })
	v1 := app.Group("/local/v1")
	v1.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "local"}) })
	v1.Post("/login", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.login)
	v1.Post("/account/recover", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.recoverOwner)
	protected := v1.Group("", s.requireSession)
	protected.Get("/me", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		session := c.Locals("session").(localauth.Session)
		return c.JSON(fiber.Map{"tenant_id": session.Actor.TenantID, "store_id": session.Actor.StoreID, "identity_id": session.Actor.IdentityID, "device_id": session.Device.DeviceID, "expires_unix": session.ExpiresUnix})
	})
	protected.Get("/capabilities", s.capabilities)
	protected.Get("/access/policy", s.readPolicy)
	protected.Post("/access/policy", limiter.New(limiter.Config{Max: 20, Expiration: time.Minute}), s.mutatePolicy)
	protected.Get("/access/audit", s.auditAccess)
	protected.Get("/staff", s.listStaff)
	protected.Post("/staff", s.createStaff)
	protected.Post("/logout", s.logout)
	protected.Get("/account/sessions", s.ownSessions)
	protected.Post("/account/password", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.changeOwnPassword)
	protected.Post("/account/sessions/revoke-others", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.revokeOtherSessions)
	protected.Post("/staff/:id/password-reset", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.resetStaffPassword)
	protected.Post("/staff/:id/sessions/revoke", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.revokeStaffSessions)
	protected.Post("/account/recovery/revoke", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.revokeOwnRecovery)
	protected.Post("/module-contracts", s.installContract)
	s.mountCatalog(protected)
	s.mountCatalogEdit(protected)
	s.mountProductState(protected)
	s.mountProductStateHistory(protected)
	s.mountComparison(protected)
	s.mountPurchases(protected)
	s.mountTrade(protected)
	s.mountSupplierEdit(protected)
	s.mountSupplierHistory(protected)
	s.mountSupplierOperation(protected)
	s.mountPurchaseSearch(protected)
	s.mountPurchaseTrace(protected)
	s.mountPurchaseCancel(protected)
	s.mountPurchaseCancelOperation(protected)
	s.mountPurchaseReceivingAuthorization(protected)
	s.mountPurchaseReceipts(protected)
	s.mountPurchaseReceivingTrace(protected)
	s.mountPurchaseRejections(protected)
	s.mountPurchaseReceiptVoids(protected)
	s.mountPurchaseReceivingHistory(protected)
	s.mountPurchaseReceivingOperations(protected)
	s.mountProduction(protected)
	s.mountProductionOperations(protected)
	s.mountRecipeState(protected)
	s.mountProductionCapacity(protected)
	s.mountProductionOrders(protected)
	s.mountCheckoutPlans(protected)
	s.mountCheckoutProofs(protected)
	s.mountCashAccounting(protected)
	s.mountPayables(protected)
	s.mountPayableCash(protected)
	s.mountPayableReadiness(protected)
	s.mountSupplyCycle(protected)
	s.mountSupplierReturns(protected)
	s.mountCarts(protected)
	s.mountCustomerReturns(protected)
	s.mountCashMovementReceipt(protected)
	s.mountProductionMaterials(protected)
	s.mountProductionResults(protected)
	s.mountProductionLosses(protected)
	s.mountProductionStages(protected)
	s.mountProductionLots(protected)
	s.mountProductionQuality(protected)
	s.mountProductionTrace(protected)
	s.mountProductionSearch(protected)
	s.mountProductionLotSearch(protected)
	s.mountProductionProductUsage(protected)
	s.mountStockAvailability(protected)
	s.mountStockReservations(protected)
	s.mountReplenishment(protected)
	s.mountStock(protected)
	s.mountCash(protected)
	s.mountCashBags(protected)
	s.mountVaultCounts(protected)
	s.mountCashVaults(protected)
	s.mountSales(protected)
	s.mountFiscal(protected)
	return app, nil
}

func (s *Server) login(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if len(c.Body()) > 4096 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	var request struct {
		IdentityID string `json:"identity_id"`
		Password   string `json:"password"`
	}
	if err := decodeCash(c.Body(), &request, []string{"identity_id", "password"}); err != nil || request.IdentityID == "" || request.Password == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session, err := localauth.Login(c.UserContext(), s.DB, s.Device, request.IdentityID, request.Password)
	if err != nil {
		if errors.Is(err, localauth.ErrDenied) {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(fiber.Map{"token": session.Token, "expires_unix": session.ExpiresUnix})
}

func (s *Server) requireSession(c *fiber.Ctx) error {
	header := c.Get(fiber.HeaderAuthorization)
	if !strings.HasPrefix(header, "Bearer ") {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	token := strings.TrimPrefix(header, "Bearer ")
	session, err := localauth.Resolve(c.UserContext(), s.DB, token)
	if err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	if session.Device != s.Device {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	c.Locals("session", session)
	return c.Next()
}

func (s *Server) logout(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	if err := localauth.Logout(c.UserContext(), s.DB, session.Token); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
