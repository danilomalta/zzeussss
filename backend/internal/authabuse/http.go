package authabuse

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"strings"
	"time"
)

// HTTP deliberately uses the socket peer, not Fiber's configurable ProxyHeader
// nor any Forwarded header. Native and browser requests consume the same DB
// budgets. Origin validation runs separately BEFORE this middleware.
func HTTP(store func() Store, scope, channel, accountField string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer cancel()
		s := store()
		if err := s.Take(ctx, scope, channel, c.Context().RemoteIP().String(), ""); err != nil {
			return failure(c, err)
		}
		// Keep only the identifier needed for the failed-account pseudonym; the
		// password/recovery secret is never decoded, persisted or logged here.
		account := ""
		if accountField != "" && len(c.Body()) <= 4096 {
			var raw map[string]json.RawMessage
			if json.Unmarshal(c.Body(), &raw) == nil {
				_ = json.Unmarshal(raw[accountField], &account)
			}
			if accountField == "email" {
				account = strings.ToLower(strings.TrimSpace(account))
			}
		}
		err := c.Next()
		if c.Response().StatusCode() == fiber.StatusUnauthorized && account != "" {
			if budgetErr := s.Take(ctx, scope, channel, "", account); budgetErr != nil {
				return failure(c, budgetErr)
			}
		}
		return err
	}
}
func failure(c *fiber.Ctx, err error) error {
	c.Response().ResetBody()
	c.Response().Header.Del(fiber.HeaderSetCookie)
	if errors.Is(err, ErrLimited) {
		c.Set(fiber.HeaderRetryAfter, "60")
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
	return c.SendStatus(fiber.StatusServiceUnavailable)
}
