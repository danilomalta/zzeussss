package routes

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
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
	app.Use(apicontract.Errors())
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
	authGroup.Post("/recovery/key", middleware.AuthGuard(), middleware.RateLimitLogin(), middleware.CurrentUser(), authHandler.IssueRecovery)
	authGroup.Post("/recovery/reset", authDelivery.RefreshOrigin, middleware.RateLimitLogin(), authHandler.Recover)
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
	produtos.Get("/search", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "cashier", "stock"), catalogDelivery.BuscarProdutos)
	produtos.Get("/:id/creation", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.AuditoriaCadastro)
	negocios.Get("/catalog/creations/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.ConsultarCadastro)
	produtos.Get("/:id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "cashier", "stock"), catalogDelivery.ConsultarProduto)
	produtos.Post("/:id/details", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.AlterarProduto("details"))
	produtos.Post("/:id/price", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.AlterarProduto("price"))
	produtos.Post("/:id/active", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.AlterarProduto("active"))

	produtos.Get("/:id/barcodes", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock", "cashier"), catalogDelivery.ListarCodigosProduto(false))
	produtos.Post("/:id/barcodes", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.AlterarCodigoProduto("add"))
	produtos.Post("/:id/barcodes/:barcode_id/active", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.AlterarCodigoProduto("active"))
	produtos.Get("/:id/barcodes/history", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ListarCodigosProduto(true))
	negocios.Get("/catalog/barcodes/lookup", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock", "cashier"), catalogDelivery.LocalizarCodigoProduto)
	negocios.Get("/catalog/barcodes/operations/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.ConsultarOperacaoCodigo)
	negocios.Post("/catalog/barcodes/batches/preview", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CadastrarLoteCodigos(false))
	negocios.Post("/catalog/barcodes/batches/apply", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CadastrarLoteCodigos(true))
	negocios.Get("/catalog/barcodes/batches/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.ConsultarLoteCodigos)
	produtos.Get("/:id/history", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.HistoricoProduto)
	negocios.Get("/catalog/operations/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.ConsultarOperacaoCatalogo)

	negocios.Post("/catalog/batches/preview", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.LoteCatalogo(false))
	negocios.Post("/catalog/batches/apply", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.LoteCatalogo(true))
	negocios.Get("/catalog/batches", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.HistoricoLotesCatalogo)
	negocios.Get("/catalog/batches/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ConsultarLoteCatalogo)

	negocios.Post("/catalog/undos/preview", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.DesfazerLoteCatalogo(false))
	negocios.Post("/catalog/imports/preview", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ImportarCatalogoCSV(false))
	negocios.Post("/catalog/adjustments/preview", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ReajustarCatalogo(false))
	negocios.Post("/catalog/adjustments/apply", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ReajustarCatalogo(true))
	negocios.Get("/catalog/adjustments", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.HistoricoReajustesCatalogo)
	negocios.Get("/catalog/adjustments/exports/preview", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ExportarReajustes(false))
	negocios.Get("/catalog/adjustments/exports/csv", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ExportarReajustes(true))
	negocios.Get("/catalog/adjustments/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ConsultarReajusteCatalogo)
	negocios.Get("/catalog/exports/preview", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ExportarCatalogo(false))
	negocios.Get("/catalog/exports/csv", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ExportarCatalogo(true))
	negocios.Post("/catalog/imports/apply", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ImportarCatalogoCSV(true))
	negocios.Get("/catalog/imports/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ConsultarImportacaoCatalogo(false))
	negocios.Get("/catalog/batches/:operation_id/import", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ConsultarImportacaoCatalogo(true))
	negocios.Post("/catalog/undos/apply", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.DesfazerLoteCatalogo(true))
	negocios.Get("/catalog/undos/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ConsultarReversaoCatalogo(false))
	negocios.Get("/catalog/batches/:operation_id/undo", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ConsultarReversaoCatalogo(true))

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
