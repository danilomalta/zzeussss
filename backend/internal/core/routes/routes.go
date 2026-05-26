package routes

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/modules/auth/usecase"
	authDelivery "titansystem-backend/internal/modules/auth/delivery"
	catalogDelivery "titansystem-backend/internal/modules/catalog/delivery"
	tenantDelivery "titansystem-backend/internal/modules/tenant/delivery"
	posDelivery "titansystem-backend/internal/modules/pos/delivery"
	financialDelivery "titansystem-backend/internal/modules/financial/delivery"
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
	api := app.Group("/api")
	v1 := api.Group("/v1")

	// Autenticação (SecOps e Bounded Context: Auth)
	authUseCase := usecase.NewLoginUseCase()
	authHandler := authDelivery.NewAuthHandler(authUseCase)
	authGroup := v1.Group("/auth")
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/refresh", authHandler.RefreshToken)

	// Health Check
	v1.Get("/saude", Saude)

	// Catálogo de Produtos (Bounded Context: Catalog)
	produtos := v1.Group("/produtos")
	produtos.Get("/", catalogDelivery.ListarProdutos)
	produtos.Post("/", catalogDelivery.CriarProduto)

	// Análises com Inteligência Artificial (Bounded Context: Catalog)
	analises := v1.Group("/analises")
	analises.Get("/produtos-parados", catalogDelivery.ListarProdutosParados)

	// Indicações e Recompensas SaaS (Bounded Context: Tenant)
	recompensas := v1.Group("/recompensas")
	recompensas.Post("/indicacoes", tenantDelivery.CriarIndicacao)
	recompensas.Post("/indicacoes/recompensar", tenantDelivery.ConcederRecompensaIndicacao)
	recompensas.Get("/indicacoes/saldo", tenantDelivery.ConsultarSaldoIndicador)

	// Contábil e Fiscal - SPED (Bounded Context: Financial)
	contabil := v1.Group("/accounting")
	contabil.Post("/sped/request", financialDelivery.RequestSpedGeneration)
	contabil.Get("/sped/status/:job_id", financialDelivery.GetSpedStatus)

	// Frente de Caixa e Motor de Descontos (Bounded Context: POS)
	descontos := v1.Group("/discounts")
	descontos.Post("/suggest", posDelivery.SuggestDiscounts)
	descontos.Get("/suggestions", posDelivery.GetSuggestions)
	descontos.Post("/suggestions/:id/review", posDelivery.ReviewSuggestion)

	// Tempo real (WebSocket): chat e funil logístico transversal
	websocket.RegistrarRotas(app)
}
