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

	negocios.Get("/catalog/stock/summary", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.ResumoEstoqueOnline)
	negocios.Get("/catalog/stock/zero", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.RelatorioEstoqueOnline("zero_stock", false))
	negocios.Get("/catalog/stock/below-minimum", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.RelatorioEstoqueOnline("below_minimum", false))
	negocios.Get("/catalog/stock/coverage", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.RelatorioEstoqueOnline("all", false))
	negocios.Get("/catalog/stock/export.csv", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.RelatorioEstoqueOnline("all", true))
	negocios.Get("/catalog/cost-history/summary", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ResumoHistoricoCustos)
	negocios.Get("/catalog/cost-history/entries", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RelatorioHistoricoCustos(false, false))
	negocios.Get("/catalog/cost-history/entries/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DetalheHistoricoCustos)
	negocios.Get("/catalog/cost-history/products/:id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RelatorioHistoricoCustos(true, false))
	negocios.Get("/catalog/cost-history/export.csv", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RelatorioHistoricoCustos(false, true))
	negocios.Get("/catalog/margins/summary", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ResumoMargens)
	negocios.Get("/catalog/margins/missing-cost", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RelatorioMargens("missing_cost", false))
	negocios.Get("/catalog/margins/negative", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RelatorioMargens("negative", false))
	negocios.Get("/catalog/margins/below-target", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RelatorioMargens("below_target", false))
	negocios.Get("/catalog/margins/export.csv", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RelatorioMargens("all", true))

	produtos.Get("/:id/stock-settings", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ConsultarPlanejamentoEstoque)
	produtos.Post("/:id/stock-settings/preview", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.AlterarPlanejamentoEstoque(true))
	produtos.Post("/:id/stock-settings", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.AlterarPlanejamentoEstoque(false))
	produtos.Get("/:id/stock-settings/history", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.HistoricoPlanejamentoEstoque)
	negocios.Get("/catalog/stock-settings/operations/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ConsultarOperacaoPlanejamento)

	produtos.Get("/:id/cost", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ConsultarCusto)
	produtos.Post("/:id/cost", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.AlterarCusto)
	produtos.Get("/:id/cost/history", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.HistoricoCusto)
	produtos.Get("/:id/margin", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ConsultarMargem)
	negocios.Get("/catalog/costs/operations/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ConsultarOperacaoCusto)

	produtos.Get("/:id/barcodes", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock", "cashier"), catalogDelivery.ListarCodigosProduto(false))
	produtos.Post("/:id/barcodes", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.AlterarCodigoProduto("add"))
	produtos.Post("/:id/barcodes/:barcode_id/active", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.AlterarCodigoProduto("active"))
	produtos.Get("/:id/barcodes/history", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), catalogDelivery.ListarCodigosProduto(true))
	negocios.Get("/catalog/barcodes/lookup", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock", "cashier"), catalogDelivery.LocalizarCodigoProduto)
	negocios.Get("/catalog/barcodes/operations/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager", "stock"), catalogDelivery.ConsultarOperacaoCodigo)
	negocios.Post("/catalog/barcodes/batches/preview", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CadastrarLoteCodigos(false))
	negocios.Post("/catalog/barcodes/batches/apply", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CadastrarLoteCodigos(true))
	negocios.Get("/catalog/barcodes/batches/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.ConsultarLoteCodigos)
	negocios.Post("/catalog/barcodes/import/preview", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.ImportarCodigosCSV(false))
	negocios.Post("/catalog/barcodes/import/apply", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.ImportarCodigosCSV(true))
	negocios.Get("/catalog/barcodes/import/template", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.ModeloCodigosCSV)
	negocios.Get("/produtos/:id/barcodes/export.csv", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.ExportarCodigosCSV)
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

	negocios.Get("/catalog/quality/summary", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CatalogQualitySummary)
	negocios.Get("/catalog/quality/no-active-gtin", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CatalogQuality("no_active_gtin", false))
	negocios.Get("/catalog/quality/no-description", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CatalogQuality("no_description", false))
	negocios.Get("/catalog/quality/zero-stock", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CatalogQuality("zero_stock", false))
	negocios.Get("/catalog/quality/export.csv", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager", "stock"), catalogDelivery.CatalogQuality("any", true))

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
	descontos.Get("/checks/summary", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountCheckSummary)
	descontos.Get("/checks/price-changed", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountChecks("price_changed", false))
	descontos.Get("/checks/inactive", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountChecks("inactive", false))
	descontos.Get("/checks/suggestions/:id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountCheck)
	descontos.Get("/checks/export.csv", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountChecks("any", true))
	descontos.Get("/report/summary", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountReportSummary)
	descontos.Get("/report/pending", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountReport("PENDING", false))
	descontos.Get("/report/approved", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountReport("APPROVED", false))
	descontos.Get("/report/rejected", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountReport("REJECTED", false))
	descontos.Get("/report/export.csv", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.DiscountReport("all", true))
	descontos.Post("/suggest", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), posDelivery.SuggestDiscounts)
	descontos.Get("/suggestions", middleware.CurrentUser(), middleware.RequireRoles("admin", "owner", "manager"), posDelivery.GetSuggestions)
	descontos.Post("/suggestions/:id/review", Indisponivel)
	descontos.Get("/suggestions/:id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ConsultarSugestaoDesconto)
	descontos.Post("/suggestions/:id/approve", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RevisarSugestaoDesconto("approve"))
	descontos.Post("/suggestions/:id/reject", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.RevisarSugestaoDesconto("reject"))
	descontos.Get("/suggestions/:id/reviews", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.HistoricoDecisaoDesconto)
	descontos.Get("/reviews/:operation_id", middleware.CurrentUser(), middleware.RequireRoles("owner", "admin", "manager"), catalogDelivery.ConsultarDecisaoDesconto)

	// O chat de demonstração não recebe autenticação; não é exposto nesta fase.
}
