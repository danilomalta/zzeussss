package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"titansystem-backend/internal/auth"
	"titansystem-backend/internal/database"
	"titansystem-backend/internal/routes"
)

func main() {
	log.Println("Iniciando TitanSystem Backend (API) de Produção...")

	// 1. Carrega o arquivo .env se existir (silencioso em produção, logando local)
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: Arquivo .env não localizado. Usando variáveis de ambiente globais.")
	}

	// 2. Inicializa a conexão com o banco (PostgreSQL + pgxpool + GORM)
	database.InitDB()

	// [Fail Fast] Valida se os ponteiros de pool concorrente e ORM foram gerados com sucesso
	if database.DB == nil {
		log.Fatal("Erro crítico: falha ao inicializar o GORM (database.DB é nulo). Abortando.")
	}
	if database.Pool == nil {
		log.Fatal("Erro crítico: falha ao estabelecer o Pool de conexões pgxpool. Abortando.")
	}

	app := fiber.New(fiber.Config{
		AppName: "TitanSystem Backend API (PostgreSQL Cores)",
	})

	// 3. Proteção contra força bruta em tentativas de login (SecOps)
	limitadorCfg := auth.ConfiguracaoTentativasLoginPadrao()
	limitadorCfg.CaminhoLogin = "/api/v1/auth/login" // Ajustado ao padrão de rotas atual
	limitador := auth.NovoLimitadorTentativasLogin(limitadorCfg)
	app.Use(limitador.Middleware())

	// 4. Registro de Rotas
	routes.Registrar(app)

	// 5. Servidor ouvindo estritamente na porta 8080 (requisito corporativo)
	addr := ":8080"
	log.Printf("Servidor HTTP rodando sob PostgreSQL concorrente. addr=%s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Erro crítico: falha ao iniciar servidor HTTP: %v", err)
	}
}
