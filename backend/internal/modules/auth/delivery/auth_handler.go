package delivery

import (
	"errors"
	"os"
	"strings"
	"time"

	"titansystem-backend/internal/modules/auth/usecase"
	"titansystem-backend/internal/onlinesessions"

	"github.com/gofiber/fiber/v2"
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
	c.Set("Cache-Control", "no-store")
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
	c.Cookie(cookieRenovacao(output.RefreshToken, output.RefreshExpires))

	return c.JSON(output)
}

// RefreshToken lida com a renovação silenciosa dos tokens de acesso.
//
// ROTA: POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	cookie := c.Cookies("titan_session_rt")
	if cookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "refresh token ausente",
		})
	}

	if _, err := onlinesessions.ParseRefresh(cookie); err != nil {
		return sessionFailure(c, err)
	}
	store, err := sessionStore()
	if err != nil {
		return sessionFailure(c, err)
	}
	ctx, cancel := sessionContext(c)
	defer cancel()
	tokens, err := store.Rotate(ctx, cookie)
	if err != nil {
		return sessionFailure(c, err)
	}
	c.Cookie(cookieRenovacao(tokens.Refresh, tokens.RefreshExpires))

	return c.JSON(fiber.Map{
		"access_token": tokens.Access,
		"expires_in":   tokens.ExpiresIn,
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
func cookieRenovacao(valor string, expires time.Time) *fiber.Cookie {
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
		Expires:  expires,
		HTTPOnly: true,
		Secure:   seguro,
		SameSite: mesmoSite,
		Path:     "/api/v1/auth/refresh",
	}
}
