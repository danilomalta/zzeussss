package middleware

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS retorna o middleware de CORS configurado de forma estanque para produção.
func CORS() fiber.Handler {
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		// Fallback seguro de desenvolvimento local e rede local
		allowedOrigins = "http://localhost:5173,http://localhost:3000,http://127.0.0.1:5173"
	}

	return cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowCredentials: true,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Requested-With",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		MaxAge:           86400, // 24 horas de cache preflight para otimização
	})
}
