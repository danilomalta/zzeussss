package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/auth"
	"titansystem-backend/internal/database"
	"titansystem-backend/internal/routes"
)

func main() {
	log.Println("Iniciando TitanSystem Backend (API)...")

	// Inicializa a conexão com o banco (SQLite).
	database.InitDB()

	app := fiber.New(fiber.Config{
		AppName: "TitanSystem Backend API",
	})

	// Proteção contra força bruta em tentativas de login.
	limitador := auth.NovoLimitadorTentativasLogin(auth.ConfiguracaoTentativasLoginPadrao())
	app.Use(limitador.Middleware())

	routes.Registrar(app)

	addr := ":3000"
	log.Printf("Servidor HTTP pronto. addr=%s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Erro crítico: falha ao iniciar servidor HTTP: %v", err)
	}
}
