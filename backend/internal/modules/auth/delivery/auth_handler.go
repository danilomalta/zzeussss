package delivery

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
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
		SameSite: "Strict",                           // Mitigação total para Cross-Site Request Forgery (CSRF)
		Path:     "/api/v1/auth/refresh",             // Restringe o escopo de transmissão de cabeçalhos
	})

	// 4. Retorna o Access Token (JWT) e dados públicos no corpo da resposta
	return c.JSON(output)
}

// RefreshToken lida com a requisição HTTP de renovação silenciosa dos tokens de acesso.
//
// ROTAS: POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	// 1. Extrai o Refresh Token a partir do cookie seguro HttpOnly
	cookie := c.Cookies("titan_session_rt")
	if cookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "refresh token ausente",
		})
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("Erro crítico: JWT_SECRET não configurado no ambiente.")
	}

	// 2. Faz o parse e validação do token JWT do refresh token
	token, err := jwt.Parse(cookie, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "método de assinatura inválido")
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "refresh token inválido ou expirado",
		})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "refresh" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "tipo de token inválido",
		})
	}

	userID := claims["sub"].(string)
	role := claims["role"].(string)
	name := claims["name"].(string)
	tenantID := claims["tenant_id"].(string)

	// 3. Emite um novo Access Token (JWT - expira em 15 minutos)
	newAccessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       userID,
		"role":      role,
		"name":      name,
		"tenant_id": tenantID,
		"exp":       time.Now().Add(15 * time.Minute).Unix(),
		"iat":       time.Now().Unix(),
	})

	newAccessToken, err := newAccessTokenObj.SignedString([]byte(jwtSecret))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "falha ao assinar novo token de acesso",
		})
	}

	// 4. Emite um novo Refresh Token (JWT - expira em 7 dias para rotatividade)
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

	// 5. Injeta o novo Refresh Token na resposta utilizando c.Cookie()
	c.Cookie(&fiber.Cookie{
		Name:     "titan_session_rt",
		Value:    newRefreshToken,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
		Path:     "/api/v1/auth/refresh",
	})

	return c.JSON(fiber.Map{
		"access_token": newAccessToken,
		"expires_in":   int64(15 * 60),
	})
}
