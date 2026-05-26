package middleware

import (
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// AuthGuard retorna um middleware do Fiber para controle de acesso baseado em autenticação JWT.
func AuthGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extração do cabeçalho de Autorização
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "cabeçalho Authorization ausente",
			})
		}

		// 2. Validação do prefixo Bearer
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "esquema de autenticação inválido, utilize Bearer",
			})
		}

		tokenString := strings.TrimPrefix(authHeader, prefix)

		// 3. Carrega o segredo criptográfico
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			log.Fatal("Erro crítico: JWT_SECRET não configurado no ambiente.")
		}

		// 4. Decodificação e validação criptográfica da assinatura
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "método de assinatura inválido")
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "token de acesso inválido ou expirado",
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "claims do token inválidos",
			})
		}

		// 5. Injeta as informações extraídas no Locals da requisição
		userID, _ := claims["sub"].(string)
		role, _ := claims["role"].(string)
		tenantID, _ := claims["tenant_id"].(string)

		c.Locals("user_id", userID)
		c.Locals("role", role)
		c.Locals("tenant_id", tenantID)

		return c.Next()
	}
}
