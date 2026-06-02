package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func AuthGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		
		// 1. Verifica se o header existe e tem o formato correto
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Acesso negado. Token ausente ou mal formatado.",
			})
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		jwtSecret := os.Getenv("JWT_SECRET")

		// 2. Faz o parse e a validação criptográfica do token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return []byte(jwtSecret), nil
		})

		// 3. Rejeita se expirado, corrompido ou assinatura falsa
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Sessão inválida ou expirada. Faça login novamente.",
			})
		}

		// 4. Extrai os dados do payload e injeta no contexto
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Falha ao ler dados do token.",
			})
		}

		c.Locals("userID", claims["sub"])
		c.Locals("role", claims["role"])
		c.Locals("tenant_id", claims["tenant_id"]) // Mantém tenant_id para isolamento multi-tenant

		return c.Next()
	}
}
