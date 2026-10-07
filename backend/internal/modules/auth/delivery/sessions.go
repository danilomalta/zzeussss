package delivery

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/onlinesessions"
	"titansystem-backend/pkg/middleware"
)

func sessionStore() (*onlinesessions.Store, error) {
	if database.DB == nil || os.Getenv("JWT_SECRET") == "" {
		return nil, onlinesessions.ErrUnavailable
	}
	db, err := database.DB.DB()
	if err != nil {
		return nil, onlinesessions.ErrUnavailable
	}
	return onlinesessions.New(db, os.Getenv("JWT_SECRET")), nil
}
func sessionContext(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), 10*time.Second)
}
func sessionScope(c *fiber.Ctx) onlinesessions.Scope {
	tenant, _ := middleware.TenantID(c)
	role, _ := c.Locals("role").(string)
	id, _ := c.Locals("session_id").(string)
	return onlinesessions.Scope{Tenant: tenant, User: middleware.UserID(c), Role: role, Session: id}
}
func sessionFailure(c *fiber.Ctx, err error) error {
	if errors.Is(err, onlinesessions.ErrDenied) {
		return c.Status(401).JSON(fiber.Map{"error": "sessão inválida ou expirada"})
	}
	return c.Status(503).JSON(fiber.Map{"error": "sessão indisponível"})
}

// RefreshOrigin blocks cookie mutation from an unapproved browser origin.
// Requests without Origin are allowed for native clients; they still need the
// unpredictable refresh secret. CORS alone does not prevent a server-side write.
func RefreshOrigin(c *fiber.Ctx) error {
	if c.Get("Sec-Fetch-Site") == "cross-site" {
		return c.SendStatus(403)
	}
	origin := c.Get("Origin")
	if origin == "" {
		return c.Next()
	}
	allowed := os.Getenv("ALLOWED_ORIGINS")
	if allowed == "" {
		allowed = "http://localhost:3000"
	}
	for _, value := range strings.Split(allowed, ",") {
		if strings.TrimSpace(value) == origin && origin != "null" && origin != "*" {
			return c.Next()
		}
	}
	return c.SendStatus(403)
}
func (h *AuthHandler) Sessions(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	s, err := sessionStore()
	if err != nil {
		return sessionFailure(c, err)
	}
	ctx, cancel := sessionContext(c)
	defer cancel()
	items, err := s.List(ctx, sessionScope(c))
	if err != nil {
		return sessionFailure(c, err)
	}
	return c.JSON(fiber.Map{"sessions": items, "limit": 100})
}
func (h *AuthHandler) RevokeSession(c *fiber.Ctx) error { return h.revoke(c, c.Params("id"), false) }
func (h *AuthHandler) RevokeOthers(c *fiber.Ctx) error {
	return h.revoke(c, "00000000-0000-0000-0000-000000000000", true)
}
func (h *AuthHandler) Logout(c *fiber.Ctx) error { return h.revoke(c, sessionScope(c).Session, false) }
func (h *AuthHandler) revoke(c *fiber.Ctx, target string, others bool) error {
	c.Set("Cache-Control", "no-store")
	if len(c.Body()) != 0 {
		return c.SendStatus(400)
	}
	if !others && !onlinesessions.ValidID(target) {
		return c.SendStatus(400)
	}
	s, err := sessionStore()
	if err != nil {
		return sessionFailure(c, err)
	}
	ctx, cancel := sessionContext(c)
	defer cancel()
	p := sessionScope(c)
	if err = s.Revoke(ctx, p, target, others); err != nil {
		return sessionFailure(c, err)
	}
	if !others && target == p.Session {
		cookie := cookieRenovacao("", time.Unix(1, 0))
		cookie.MaxAge = -1
		c.Cookie(cookie)
	}
	return c.SendStatus(204)
}
