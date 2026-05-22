package routes

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/handlers"
	handlers_ai "titansystem-backend/internal/handlers/ai_insights"
	handlers_recompensas "titansystem-backend/internal/handlers/rewards"
	"titansystem-backend/internal/usecase/auth"
	"titansystem-backend/internal/websocket"
)

// Registrar registra todas as rotas HTTP do backend.
func Registrar(app *fiber.App) {
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Autenticação (SecOps)
	authUseCase := auth.NewLoginUseCase()
	authHandler := handlers.NewAuthHandler(authUseCase)
	authGroup := v1.Group("/auth")
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/refresh", authHandler.RefreshToken)

	v1.Get("/saude", handlers.Saude)

	produtos := v1.Group("/produtos")
	produtos.Get("/", handlers.ListarProdutos)
	produtos.Post("/", handlers.CriarProduto)

	analises := v1.Group("/analises")
	analises.Get("/produtos-parados", handlers_ai.ListarProdutosParados)

	recompensas := v1.Group("/recompensas")
	recompensas.Post("/indicacoes", handlers_recompensas.CriarIndicacao)
	recompensas.Post("/indicacoes/recompensar", handlers_recompensas.ConcederRecompensaIndicacao)
	recompensas.Get("/indicacoes/saldo", handlers_recompensas.ConsultarSaldoIndicador)

	// Contábil (SPED)
	contabil := v1.Group("/accounting")
	contabil.Post("/sped/request", handlers.RequestSpedGeneration)
	contabil.Get("/sped/status/:job_id", handlers.GetSpedStatus)

	// Motor de Descontos
	descontos := v1.Group("/discounts")
	descontos.Post("/suggest", handlers.SuggestDiscounts)
	descontos.Get("/suggestions", handlers.GetSuggestions)
	descontos.Post("/suggestions/:id/review", handlers.ReviewSuggestion)

	// Tempo real (WebSocket): chat e, no futuro, atualizações do funil logístico.
	websocket.RegistrarRotas(app)
}

