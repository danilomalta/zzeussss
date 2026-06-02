package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimitGeral limita requisições gerais na API (máximo de 100 requisições por minuto por IP).
func RateLimitGeral() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Limite de requisições excedido. Tente novamente mais tarde.",
			})
		},
	})
}

// RateLimitLogin limita tentativas de força bruta na rota de login (máximo de 5 requisições por minuto por IP).
func RateLimitLogin() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Limite de tentativas de login excedido. Tente novamente em 1 minuto.",
			})
		},
	})
}
