package websocket

import (
	"github.com/gofiber/fiber/v2"
	ws "github.com/gofiber/websocket/v2"
)

// RegistrarRotas registra os endpoints de tempo real e prepara o upgrade para WebSocket.
func RegistrarRotas(app *fiber.App) {
	grupo := app.Group("/api/v1/tempo-real")

	// Middleware obrigatório para upgrade seguro de HTTP para WebSocket.
	grupo.Use("/chat", func(c *fiber.Ctx) error {
		if ws.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.Status(fiber.StatusUpgradeRequired).JSON(fiber.Map{
			"erro": "upgrade para websocket é obrigatório neste endpoint",
		})
	})

	grupo.Get("/chat", ws.New(HandlerChat))
}

