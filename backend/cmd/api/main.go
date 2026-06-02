package main

import (
	"log"
	"net"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/core/routes"
	"titansystem-backend/internal/core/sync"
	"titansystem-backend/internal/modules/auth/delivery"
)

// ObterIPLocal busca o primeiro endereço IPv4 local não-loopback (como 192.168.x.x).
func ObterIPLocal() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func main() {
	log.Println("Iniciando TitanSystem Backend (API) de Produção...")

	// 1. Carrega o arquivo .env se existir
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

	// 3. Inicializa o SyncWorker de Resiliência Local-First
	worker := sync.NovoSyncWorker()
	worker.Start(1 * time.Minute)

	app := fiber.New(fiber.Config{
		AppName: "TitanSystem Backend API (PostgreSQL Cores)",
	})

	// 4. Proteção contra força bruta em tentativas de login (SecOps)
	limitadorCfg := delivery.ConfiguracaoTentativasLoginPadrao()
	limitadorCfg.CaminhoLogin = "/api/v1/auth/login"
	limitador := delivery.NovoLimitadorTentativasLogin(limitadorCfg)
	app.Use(limitador.Middleware())

	// 5. Registro de Rotas
	routes.Registrar(app)

	// 6. Servidor ouvindo na rede local (LAN/Wi-Fi)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port
	ipLocal := ObterIPLocal()

	log.Println("────────────────────────────────────────────────────────────────")
	log.Printf("🚀 TitanSystem rodando localmente em: http://localhost:%s", port)
	log.Printf("Para acessar via smartphone no Wi-Fi, abra: http://%s:%s", ipLocal, port)
	log.Println("────────────────────────────────────────────────────────────────")

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Erro crítico: falha ao iniciar servidor HTTP: %v", err)
	}
}
