package localapi

import (
	"github.com/gofiber/fiber/v2"
	"net/url"
	"os"
	"strings"
)

// Missing Origin is for native clients, not authorization. Cross-site browser
// requests are rejected before touching budgets or credentials. A same-origin
// reverse proxy is allowed without trusting its forwarded identity headers.
func localAuthOrigin(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if c.Get("Sec-Fetch-Site") == "cross-site" {
		return c.SendStatus(403)
	}
	origin := c.Get("Origin")
	if origin == "" {
		return c.Next()
	}
	if origin == "null" || origin == "*" {
		return c.SendStatus(403)
	}
	allowed := os.Getenv("ALLOWED_ORIGINS")
	if allowed == "" {
		allowed = "http://localhost:3000,titan://app"
	}
	for _, value := range strings.Split(allowed, ",") {
		if strings.TrimSpace(value) == origin {
			return c.Next()
		}
	}
	u, err := url.Parse(origin)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "" && u.Host == string(c.Context().Host()) {
		return c.Next()
	}
	return c.SendStatus(403)
}
