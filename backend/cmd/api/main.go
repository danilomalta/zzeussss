package main

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/core/routes"
	"titansystem-backend/internal/core/sync"
	"titansystem-backend/internal/modules/auth/delivery"
	"titansystem-backend/pkg/utils"
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

	// 3. Inicializa o SyncWorker de Resiliência Local-First (Passo 1)
	worker := sync.NovoSyncWorker()
	worker.Start(1 * time.Minute)

	app := fiber.New(fiber.Config{
		AppName: "TitanSystem Backend API (PostgreSQL Cores)",
	})

	// 4. Proteção contra força bruta em tentativas de login (SecOps)
	limitadorCfg := delivery.ConfiguracaoTentativasLoginPadrao()
	limitadorCfg.CaminhoLogin = "/api/v1/auth/login" // Ajustado ao padrão de rotas atual
	limitador := delivery.NovoLimitadorTentativasLogin(limitadorCfg)
	app.Use(limitador.Middleware())

	// 5. Registro de Rotas
	routes.Registrar(app)

	// 6. Servidor ouvindo na rede local (LAN/Wi-Fi) - escuta 0.0.0.0
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port
	ipLocal := utils.ObterIPLocal()

	log.Println("────────────────────────────────────────────────────────────────")
	log.Printf("🚀 TitanSystem rodando localmente em: http://localhost:%s", port)
	log.Printf("📱 Para acessar via celular no mesmo Wi-Fi, acesse: http://%s:%s", ipLocal, port)
	log.Println("────────────────────────────────────────────────────────────────")

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Erro crítico: falha ao iniciar servidor HTTP: %v", err)
	}
}
