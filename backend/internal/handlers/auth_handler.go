package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// AuthHandler gerencia as requisições HTTP relativas ao ciclo de vida de autenticação.
type AuthHandler struct {
	// A lógica futura injetará as interfaces dos Casos de Uso (Usecases) aqui:
	// loginUseCase    auth.LoginUseCase
	// refreshUseCase  auth.RefreshTokenUseCase
}

// NewAuthHandler instancia um novo controlador (Handler) de autenticação.
func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

// Login lida com a requisição HTTP de login do usuário.
//
// ROTAS: POST /api/v1/auth/login
//
// REGRAS DE NEGÓCIO E DE SEGURANÇA (SecOps):
// 1. Parsing da Requisição: Deve analisar o corpo HTTP da requisição contendo o e-mail e a senha em formato JSON,
//    mapeando-o para a estrutura de dados apropriada. Retorna HTTP 400 Bad Request em caso de payload corrompido.
// 2. Encaminhamento ao Caso de Uso: Passa a entrada sanitizada para o caso de uso correspondente de autenticação.
// 3. Emissão Segura de Refresh Token (Cookie HttpOnly):
//    - O Refresh Token gerado pelo caso de uso deve ser extraído.
//    - Deve criar e anexar um Cookie HTTP com o nome "refresh_token" ou "titan_session_rt".
//    - O Cookie deve conter as diretivas de segurança essenciais:
//        - HTTPOnly: true (bloqueia o acesso direto ao token a partir de código cliente Javascript).
//        - Secure: true (o cookie só será trafegado em requisições cifradas em HTTPS/SSL).
//        - SameSite: "Lax" (previne ataques Cross-Site Request Forgery em fluxos normais de navegação).
//        - MaxAge: Tempo correspondente à expiração do Refresh Token (ex: 7 dias em segundos).
//        - Path: "/api/v1/auth/refresh" (restringe o envio do cookie apenas a este endpoint específico, minimizando a superfície de ataque).
// 4. Retorno de Access Token (Body): O Access Token (JWT de curta duração) é enviado de volta ao cliente
//    no corpo da resposta HTTP no formato JSON seguro, juntamente com os dados resumidos do usuário autenticado.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	// [DIRETIVA DE SEGURANÇA E ARQUITETURA - SECOPS]
	// O "Access Token (JWT)" pode ser retornado livremente no corpo JSON da resposta (pois é de curta duração e será mantido em memória no frontend).
	// No entanto, o "Refresh Token" NUNCA deve ser enviado no JSON. Ele DEVE ser injetado exclusivamente através de Cookie seguro usando `c.Cookie()`
	// configurado obrigatoriamente com as flags:
	//   - HTTPOnly: true (impede roubo via ataques XSS no frontend)
	//   - Secure: true (garante tráfego estrito via HTTPS em produção)
	//   - SameSite: "Lax" (previne CSRF)
	//   - Path: "/api/v1/auth/refresh" (restringe o envio do cookie apenas a rota de renovação, blindando os outros endpoints)
	//
	// A lógica futura irá:
	// a) Fazer o binding do body (c.BodyParser) para uma struct LoginInput.
	// b) Chamar loginUseCase.Execute(input).
	// c) Se ocorrer erro de autenticação, responder com 401 Unauthorized e mensagem genérica.
	// d) Se sucesso, configurar o cookie seguro no cliente:
	//    c.Cookie(&fiber.Cookie{
	//        Name:     "titan_session_rt",
	//        Value:    output.RefreshToken,
	//        Expires:  time.Now().Add(7 * 24 * time.Hour),
	//        HTTPOnly: true,
	//        Secure:   true, // OBRIGATÓRIO EM PRODUÇÃO
	//        SameSite: "Lax",
	//        Path:     "/api/v1/auth/refresh",
	//    })
	// e) Retornar o Access Token no corpo da resposta com status 200 OK: c.JSON(output).
	return c.SendStatus(fiber.StatusNotImplemented)
}

// RefreshToken lida com a requisição HTTP de renovação silenciosa dos tokens de acesso.
//
// ROTAS: POST /api/v1/auth/refresh
//
// REGRAS DE NEGÓCIO E DE SEGURANÇA (SecOps):
// 1. Leitura Protegida do Cookie: Deve buscar o Refresh Token diretamente no cabeçalho de Cookie correspondente,
//    e nunca ler do corpo HTTP JSON nem das query strings. Retorna HTTP 401 se ausente.
// 2. Encaminhamento para Renovação: Passa o token obtido para o caso de uso de renovação de credenciais.
// 3. Rotação de Cookie:
//    - Caso o fluxo use rotação de tokens, invalida o cookie antigo.
//    - Configura e envia o novo Refresh Token em um cookie idêntico (HttpOnly, Secure, SameSite, Path restrito).
// 4. Emissão do Novo Access Token: Devolve o novo Access Token gerado no corpo JSON da resposta HTTP.
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	// A lógica futura irá:
	// a) Extrair o cookie de nome "titan_session_rt" usando c.Cookies("titan_session_rt").
	// b) Se vazio, retornar c.Status(fiber.StatusUnauthorized).JSON(...)
	// c) Chamar refreshUseCase.Execute(input).
	// d) Se falhar (token inválido/expirado/revogado), limpar o cookie definindo MaxAge: -1 e retornar 401.
	// e) Se sucesso, injetar o novo cookie do refresh token renovado.
	// f) Retornar o novo AccessToken no corpo com status 200 OK.
	return c.SendStatus(fiber.StatusNotImplemented)
}
