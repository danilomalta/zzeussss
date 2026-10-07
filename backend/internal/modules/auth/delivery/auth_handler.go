package delivery

import (
	"errors"
	"os"
	"strings"
	"time"

	"titansystem-backend/internal/core/security"
	"titansystem-backend/internal/modules/auth/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// AuthHandler gerencia as requisições HTTP relativas ao ciclo de vida de autenticação.
type AuthHandler struct {
	loginUseCase usecase.LoginUseCase
}

// NewAuthHandler instancia um novo controlador (Handler) do LoginUseCase.
func NewAuthHandler(loginUseCase usecase.LoginUseCase) *AuthHandler {
	return &AuthHandler{
		loginUseCase: loginUseCase,
	}
}

// Login lida com a requisição HTTP de login do usuário, validando credenciais e emitindo cookies de sessão.
//
// ROTA: POST /api/v1/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var input usecase.LoginInput

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "formato de requisição inválido",
		})
	}

	output, err := h.loginUseCase.Execute(input)
	if err != nil {
		if errors.Is(err, usecase.ErrSessionUnavailable) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "sessão indisponível"})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "e-mail ou senha inválidos"})
	}

	// O refresh token é armazenado somente em cookie HttpOnly.
	c.Cookie(cookieRenovacao(output.RefreshToken))

	return c.JSON(output)
}

// RefreshToken lida com a renovação silenciosa dos tokens de acesso.
//
// ROTA: POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	cookie := c.Cookies("titan_session_rt")
	if cookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "refresh token ausente",
		})
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "servidor sem chave de sessão configurada",
		})
	}

	claims, err := security.ParseSession(cookie, jwtSecret, "refresh")
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "refresh token inválido ou expirado"})
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "usuário ausente no token",
		})
	}

	role, ok := claims["role"].(string)
	if !ok || role == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "perfil ausente no token",
		})
	}

	name, ok := claims["name"].(string)
	if !ok || name == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "nome ausente no token",
		})
	}

	tenantID, ok := claims["tenant_id"].(string)
	if !ok || tenantID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "empresa ausente no token",
		})
	}
	active, lookupErr := security.ActiveSession(userID, tenantID, role)
	if lookupErr != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "sessão indisponível",
		})
	}
	if !active {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "sessão sem vínculo ativo",
		})
	}

	// Emite um novo Access Token com validade de 15 minutos.
	newAccessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       userID,
		"role":      role,
		"name":      name,
		"tenant_id": tenantID,
		"type":      "access",
		"exp":       time.Now().Add(15 * time.Minute).Unix(),
		"iat":       time.Now().Unix(),
	})

	newAccessToken, err := newAccessTokenObj.SignedString([]byte(jwtSecret))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "falha ao assinar novo token de acesso",
		})
	}

	// Emite um novo Refresh Token com validade de 7 dias.
	newRefreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       userID,
		"role":      role,
		"name":      name,
		"tenant_id": tenantID,
		"type":      "refresh",
		"exp":       time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	})

	newRefreshToken, err := newRefreshTokenObj.SignedString([]byte(jwtSecret))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "falha ao assinar novo token de renovação",
		})
	}

	c.Cookie(cookieRenovacao(newRefreshToken))

	return c.JSON(fiber.Map{
		"access_token": newAccessToken,
		"expires_in":   int64(15 * 60),
	})
}

// cookieRenovacao cria o cookie seguro utilizado pelo refresh token.
//
// Por padrão:
//   - HttpOnly: ativado;
//   - Secure: ativado;
//   - SameSite: Strict.
//
// Para desenvolvimento local usando HTTP, defina explicitamente:
//
//	COOKIE_SECURE=false
func cookieRenovacao(valor string) *fiber.Cookie {
	seguro := !strings.EqualFold(
		strings.TrimSpace(os.Getenv("COOKIE_SECURE")),
		"false",
	)

	mesmoSite := "Strict"
	if !seguro {
		mesmoSite = "Lax"
	}

	return &fiber.Cookie{
		Name:     "titan_session_rt",
		Value:    valor,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   seguro,
		SameSite: mesmoSite,
		Path:     "/api/v1/auth/refresh",
	}
}
