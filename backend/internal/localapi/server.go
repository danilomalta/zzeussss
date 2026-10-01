// Package localapi expõe o banco do aparelho somente no endereço loopback.
// New recebe um DeviceContext já provado pelo processo de inicialização.
package localapi

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

type Server struct {
	DB        *sql.DB
	Device    identity.DeviceContext
	contracts *entitlementstore.Store
}

// New preserves authentication and authorized reads without issuer keys.
// Catalog mutations are unavailable until a trusted verifier is configured.
func New(db *sql.DB, device identity.DeviceContext) (*fiber.App, error) {
	return NewWithVerifier(db, device, nil)
}

// NewWithVerifier receives trust from process configuration, never HTTP input.
// The device must already be proven by startup; user sessions are checked per request.
func NewWithVerifier(db *sql.DB, device identity.DeviceContext, verifier *entitlements.Verifier) (*fiber.App, error) {
	if db == nil || device.TenantID == "" || device.StoreID == "" || device.DeviceID == "" {
		return nil, errors.New("servidor local sem aparelho")
	}
	s := &Server{DB: db, Device: device}
	if verifier != nil {
		contracts, err := entitlementstore.New(db, verifier, nil)
		if err != nil {
			return nil, err
		}
		s.contracts = contracts
	}
	app := fiber.New(fiber.Config{AppName: "Titan Local", DisableStartupMessage: true, BodyLimit: 1 << 20})
	v1 := app.Group("/local/v1")
	v1.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "local"}) })
	v1.Post("/login", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.login)
	protected := v1.Group("", s.requireSession)
	protected.Get("/me", func(c *fiber.Ctx) error {
		session := c.Locals("session").(localauth.Session)
		return c.JSON(fiber.Map{"tenant_id": session.Actor.TenantID, "store_id": session.Actor.StoreID, "identity_id": session.Actor.IdentityID, "device_id": session.Device.DeviceID, "expires_unix": session.ExpiresUnix})
	})
	protected.Post("/logout", s.logout)
	protected.Post("/module-contracts", s.installContract)
	s.mountCatalog(protected)
	s.mountStock(protected)
	return app, nil
}

func (s *Server) login(c *fiber.Ctx) error {
	var request struct {
		IdentityID string `json:"identity_id"`
		Password   string `json:"password"`
	}
	if err := c.BodyParser(&request); err != nil || request.IdentityID == "" || request.Password == "" {
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
