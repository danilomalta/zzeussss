package middleware

import (
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func AuthGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")

		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Acesso negado. Token ausente ou mal formatado.",
			})
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "servidor sem chave de sessão configurada",
			})
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Sessão inválida ou expirada. Faça login novamente.",
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Falha ao ler dados do token.",
			})
		}

		tenantID := valorClaim(claims["tenant_id"])
		if tenantID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token sem identificação de empresa.",
			})
		}

		c.Locals("userID", valorClaim(claims["sub"]))
		c.Locals("role", valorClaim(claims["role"]))
		c.Locals("tenant_id", tenantID)

		return c.Next()
	}
}

func valorClaim(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// TenantID devolve a empresa autenticada a partir das claims do token (nunca do corpo da requisição).
func TenantID(c *fiber.Ctx) (string, error) {
	id := strings.TrimSpace(fmt.Sprint(c.Locals("tenant_id")))
	if id == "" || id == "<nil>" {
		return "", fiber.NewError(fiber.StatusUnauthorized, "empresa ausente no token")
	}
	return id, nil
}

// UserID devolve o operador autenticado a partir das claims do token.
func UserID(c *fiber.Ctx) string {
	id := strings.TrimSpace(fmt.Sprint(c.Locals("userID")))
	if id == "<nil>" {
		return ""
	}
	return id
}
