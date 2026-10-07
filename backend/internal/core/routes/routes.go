package routes

import (
	"github.com/gofiber/fiber/v2"
	authDelivery "titansystem-backend/internal/modules/auth/delivery"
	"titansystem-backend/internal/modules/auth/usecase"
	catalogDelivery "titansystem-backend/internal/modules/catalog/delivery"
	posDelivery "titansystem-backend/internal/modules/pos/delivery"
	"titansystem-backend/pkg/middleware"
)

type RespostaSaude struct {
	Status string `json:"status"`
}

// Saude é o handler de verificação de integridade do backend (inline)
func Saude(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(RespostaSaude{Status: "ativo"})
}

// Indisponivel impede a execução de fluxos ainda sem isolamento ou autorização completa.
func Indisponivel(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"status": "indisponivel",
	})
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
	authGroup.Post("/refresh", authDelivery.RefreshOrigin, authHandler.RefreshToken)
	authGroup.Post("/password", middleware.AuthGuard(), middleware.RateLimitLogin(), middleware.CurrentUser(), authHandler.ChangePassword)
	authGroup.Post("/logout", middleware.AuthGuard(), middleware.CurrentUser(), authHandler.Logout)
	authGroup.Get("/sessions", middleware.AuthGuard(), middleware.CurrentUser(), authHandler.Sessions)
	authGroup.Post("/sessions/revoke-others", middleware.AuthGuard(), middleware.CurrentUser(), authHandler.RevokeOthers)
	authGroup.Post("/sessions/:id/revoke", middleware.AuthGuard(), middleware.CurrentUser(), authHandler.RevokeSession)

	// Health Check (Rota pública sem proteção)
	v1.Get("/saude", Saude)

	// ── GRUPO DE NEGÓCIOS PROTEGIDO PELO AUTHGUARD ──────────────────────────────────
	negocios := v1.Group("")
	negocios.Use(middleware.AuthGuard())

	// Catálogo de Produtos (Bounded Context: Catalog)
	produtos := negocios.Group("/produtos")
	produtos.Get("/", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "cashier", "stock"), catalogDelivery.ListarProdutos)
	produtos.Post("/", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.CriarProduto)

	// Análises com Inteligência Artificial (Bounded Context: Catalog)
	analises := negocios.Group("/analises")
	analises.Get("/produtos-parados", Indisponivel)

	// Indicações e Recompensas SaaS (Bounded Context: Tenant)
	recompensas := negocios.Group("/recompensas")
	recompensas.Post("/indicacoes", Indisponivel)
	recompensas.Post("/indicacoes/recompensar", Indisponivel)
	recompensas.Get("/indicacoes/saldo", Indisponivel)

	// Contábil e Fiscal - SPED (Bounded Context: Financial)
	contabil := negocios.Group("/accounting")
	contabil.Post("/sped/request", Indisponivel)
	contabil.Get("/sped/status/:job_id", Indisponivel)

	// Frente de Caixa e Motor de Descontos (Bounded Context: POS)
	descontos := negocios.Group("/discounts")
	descontos.Post("/suggest", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), posDelivery.SuggestDiscounts)
	descontos.Get("/suggestions", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), posDelivery.GetSuggestions)
	descontos.Post("/suggestions/:id/review", Indisponivel)

	// O chat de demonstração não recebe autenticação; não é exposto nesta fase.
}
