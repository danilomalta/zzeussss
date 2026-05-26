package delivery

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/modules/auth/usecase"
)

// AuthHandler gerencia as requisições HTTP relativas ao ciclo de vida de autenticação.
type AuthHandler struct {
	loginUseCase usecase.LoginUseCase
}

// NewAuthHandler instancia um novo controlador (Handler) de autenticação injetando os casos de uso.
func NewAuthHandler(loginUseCase usecase.LoginUseCase) *AuthHandler {
	return &AuthHandler{
		loginUseCase: loginUseCase,
	}
}

// Login lida com a requisição HTTP de login do usuário, validando credenciais e emitindo cookies de sessão.
//
// ROTAS: POST /api/v1/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	// 1. Parsing da requisição JSON do corpo HTTP
	var input usecase.LoginInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "formato de requisição inválido",
		})
	}

	// 2. Executa a lógica de autenticação no Usecase core
	output, err := h.loginUseCase.Execute(input)
	if err != nil {
		// [SecOps] Retorna 401 Unauthorized para falhas de credencial
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// 3. Injeta de forma estrita o Refresh Token nas propriedades do Cookie
	//    [CRÍTICO] HTTPOnly e Secure ativados para blindagem total contra roubos de sessão.
	c.Cookie(&fiber.Cookie{
		Name:     "titan_session_rt",
		Value:    output.RefreshToken,
		Expires:  time.Now().Add(7 * 24 * time.Hour), // Expira em 7 dias (SecOps)
		HTTPOnly: true,                               // Bloqueia roubos de token via javascript do navegador (XSS)
		Secure:   true,                               // Garante que o cookie apenas trafegará sob SSL/HTTPS
		SameSite: "Lax",                              // Mitigação padrão para Cross-Site Request Forgery (CSRF)
		Path:     "/api/v1/auth/refresh",             // Restringe o escopo de transmissão de cabeçalhos
	})

	// 4. Retorna o Access Token (JWT) e dados públicos no corpo da resposta
	return c.JSON(output)
}

// RefreshToken lida com a requisição HTTP de renovação silenciosa dos tokens de acesso.
//
// ROTAS: POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	// A lógica futura de renovação lerá o cookie 'titan_session_rt'
	return c.SendStatus(fiber.StatusNotImplemented)
}
