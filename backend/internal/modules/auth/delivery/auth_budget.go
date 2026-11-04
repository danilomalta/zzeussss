package delivery

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/authabuse"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/onlinesessions"
)

func DurableAuthBudget(channel, accountField string) fiber.Handler {
	budget := authabuse.HTTP(func() authabuse.Store {
		if database.DB == nil {
			return authabuse.Store{}
		}
		db, err := database.DB.DB()
		if err != nil {
			return authabuse.Store{}
		}
		return authabuse.Store{DB: db, Postgres: true}
	}, "online-installation", channel, accountField)
	return func(c *fiber.Ctx) error {
		// Preserve cheap syntax rejection before any DB access. The existing IP
		// limiter precedes this wrapper; malformed recovery tokens never reach an
		// account lookup or a credential change in Store.Recover.
		if channel == "recovery" && len(c.Body()) <= 2048 {
			var input map[string]json.RawMessage
			var key string
			if json.Unmarshal(c.Body(), &input) == nil && json.Unmarshal(input["recovery_key"], &key) == nil {
				if _, err := onlinesessions.ParseRecovery(key); err != nil {
					return c.Next()
				}
			}
		}
		return budget(c)
	}
}
