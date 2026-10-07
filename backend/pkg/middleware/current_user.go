package middleware

import (
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/security"
)

// CurrentUser revalida a sessão na base PostgreSQL que atende a API legada.
// O token sozinho não prova que o usuário ainda pertence à empresa ativa.
func CurrentUser() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenantID, err := TenantID(c)
		if err != nil {
			return err
		}
		role, _ := c.Locals("role").(string)
		ok, err := security.ActiveOnlineSession(UserID(c), tenantID, role, valorClaim(c.Locals("session_id")))
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "sessão indisponível"})
		}
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "sessão sem vínculo ativo"})
		}
		return c.Next()
	}
}

// RequireRoles protege operações que exigem papel específico no servidor.
// Usar apenas depois de AuthGuard e CurrentUser.
func RequireRoles(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		for _, allowed := range roles {
			if role == allowed {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "função sem permissão"})
	}
}
