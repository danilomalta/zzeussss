package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/onlinesessions"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/core/routes"
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
	addr, err := onlineListenAddress(os.Getenv("TITAN_API_HOST"), os.Getenv("PORT"))
	if err != nil {
		log.Fatal("Endereço ou porta da API inválidos. Use TITAN_API_HOST como IP literal e PORT entre 1 e 65535.")
	}
	stopContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// 2. Inicializa a conexão com o banco (PostgreSQL + pgxpool + GORM)
	if err := database.InitDB(); err != nil {
		log.Fatal(err)
	}
	db, err := database.DB.DB()
	if err != nil {
		log.Fatal("base online indisponível")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = onlinesessions.CheckSchema(ctx, db)
	cancel()
	if err != nil {
		log.Fatal(err)
	}

	// [Fail Fast] Valida se os ponteiros de pool concorrente e ORM foram gerados com sucesso
	if database.DB == nil {
		log.Fatal("Erro crítico: falha ao inicializar o GORM (database.DB é nulo). Abortando.")
	}
	if database.Pool == nil {
		log.Fatal("Erro crítico: falha ao estabelecer o Pool de conexões pgxpool. Abortando.")
	}

	app := fiber.New(onlineServerConfig())

	app.Use(apicontract.Errors())

	// 4. Proteção contra força bruta em tentativas de login (SecOps)
	limitadorCfg := delivery.ConfiguracaoTentativasLoginPadrao()
	limitadorCfg.CaminhoLogin = "/api/v1/auth/login"
	limitador := delivery.NovoLimitadorTentativasLogin(limitadorCfg)
	app.Use(limitador.Middleware())

	// 5. Registro de Rotas
	routes.Registrar(app)

	// 6. Servidor ouvindo na rede local (LAN/Wi-Fi)
	log.Println("────────────────────────────────────────────────────────────────")
	log.Printf("API HTTP configurada em %s (somente rotas implementadas)", addr)
	log.Println("────────────────────────────────────────────────────────────────")

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Falha ao abrir o endereço HTTP configurado. Nenhum servidor iniciado.")
	}
	if err = serveOnline(stopContext, app, listener, onlineDrainTimeout, func() error {
		if closeErr := db.Close(); closeErr != nil {
			return closeErr
		}
		database.Pool.Close()
		return nil
	}); err != nil {
		// Fatal exits without deferred database cleanup under an active handler.
		// It never logs arbitrary listener/driver diagnostics.
		log.Fatal("Encerramento online não concluído normalmente. Resultado de operações sem resposta pode ser incerto; não repetir automaticamente.")
	}
	log.Println("API HTTP encerrada após concluir requisições em curso e fechar conexões de banco.")
}
