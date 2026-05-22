package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// AuthGuard retorna um middleware do Fiber para controle de acesso baseado em autenticação JWT.
//
// REGRAS DE NEGÓCIO E DE SEGURANÇA (SecOps):
// 1. Extração do Token: Deve extrair o JSON Web Token (JWT) a partir do cabeçalho HTTP 'Authorization'
//    utilizando o esquema 'Bearer <token>'.
// 2. Validação Criptográfica: Deve decodificar e validar a assinatura do JWT contra o segredo definido
//    pela variável de ambiente JWT_SECRET, utilizando algoritmos seguros (ex: HS256/RS256).
// 3. Validação dos Claims Padrão: Verificar estritamente as claims de expiração (exp), data de emissão (iat)
//    e emissor (iss) para evitar tokens obsoletos ou forjados.
// 4. Injeção de Contexto (Claims): Se o token for válido, os dados do usuário autenticado (como UserID e Role)
//    devem ser injetados nas variáveis locais da requisição (c.Locals) para uso nos handlers posteriores.
// 5. Rejeição Segura (HTTP 401): Qualquer falha de parsing, token expirado, assinatura inválida ou ausência
//    do cabeçalho deve resultar no bloqueio imediato do fluxo e resposta segura HTTP 401 Unauthorized.
func AuthGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// A lógica futura irá:
		// a) Ler o cabeçalho "Authorization".
		// b) Validar se o formato inicia com "Bearer ".
		// c) Extrair a string do token.
		// d) Executar a validação criptográfica (jwt.ParseWithClaims) usando o JWT_SECRET.
		// e) Validar claims adicionais e injetar o "userID" e "role" em c.Locals("userID") e c.Locals("role").
		// f) Se qualquer passo falhar, retornar c.Status(fiber.StatusUnauthorized).JSON(...)
		// g) Se válido, prosseguir com c.Next().
		return c.Next()
	}
}
