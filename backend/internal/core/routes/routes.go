package routes

import (
	"github.com/gofiber/fiber/v2"
	authDelivery "titansystem-backend/internal/modules/auth/delivery"
	"titansystem-backend/internal/modules/auth/usecase"
	catalogDelivery "titansystem-backend/internal/modules/catalog/delivery"
	financialDelivery "titansystem-backend/internal/modules/financial/delivery"
	posDelivery "titansystem-backend/internal/modules/pos/delivery"
	tenantDelivery "titansystem-backend/internal/modules/tenant/delivery"
	"titansystem-backend/pkg/middleware"
	"titansystem-backend/pkg/websocket"
)

type RespostaSaude struct {
	Status string `json:"status"`
}

// Saude é o handler de verificação de integridade do backend (inline)
func Saude(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(RespostaSaude{Status: "ativo"})
}

// Registrar registra todas as rotas HTTP do backend sob a nova arquitetura DDD.
func Registrar(app *fiber.App) {
	// CORS geral aplicado a todas as rotas
	app.Use(middleware.CORS())

	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Limitador Geral de requisições aplicado globalmente na API v1
	v1.Use(middleware.RateLimitGeral())

	// Autenticação (SecOps e Bounded Context: Auth)
	authUseCase := usecase.NewLoginUseCase()
	authHandler := authDelivery.NewAuthHandler(authUseCase)
	authGroup := v1.Group("/auth")
	
	// Limitador estrito para rota de login (máximo 5 req/min por IP) para proteção contra Brute Force
	authGroup.Post("/login", middleware.RateLimitLogin(), authHandler.Login)
	authGroup.Post("/refresh", authHandler.RefreshToken)

	// Health Check (Rota pública sem proteção)
	v1.Get("/saude", Saude)

	// ── GRUPO DE NEGÓCIOS PROTEGIDO PELO AUTHGUARD ──────────────────────────────────
	negocios := v1.Group("")
	negocios.Use(middleware.AuthGuard())

	// Catálogo de Produtos (Bounded Context: Catalog)
	produtos := negocios.Group("/produtos")
	produtos.Get("/", catalogDelivery.ListarProdutos)
	produtos.Post("/", catalogDelivery.CriarProduto)

	// Análises com Inteligência Artificial (Bounded Context: Catalog)
	analises := negocios.Group("/analises")
	analises.Get("/produtos-parados", catalogDelivery.ListarProdutosParados)

	// Indicações e Recompensas SaaS (Bounded Context: Tenant)
	recompensas := negocios.Group("/recompensas")
	recompensas.Post("/indicacoes", tenantDelivery.CriarIndicacao)
	recompensas.Post("/indicacoes/recompensar", tenantDelivery.ConcederRecompensaIndicacao)
	recompensas.Get("/indicacoes/saldo", tenantDelivery.ConsultarSaldoIndicador)

	// Contábil e Fiscal - SPED (Bounded Context: Financial)
	contabil := negocios.Group("/accounting")
	contabil.Post("/sped/request", financialDelivery.RequestSpedGeneration)
	contabil.Get("/sped/status/:job_id", financialDelivery.GetSpedStatus)

	// Frente de Caixa e Motor de Descontos (Bounded Context: POS)
	descontos := negocios.Group("/discounts")
	descontos.Post("/suggest", posDelivery.SuggestDiscounts)
	descontos.Get("/suggestions", posDelivery.GetSuggestions)
	descontos.Post("/suggestions/:id/review", posDelivery.ReviewSuggestion)

	// Tempo real (WebSocket): chat e funil logístico transversal
	websocket.RegistrarRotas(app)
}
