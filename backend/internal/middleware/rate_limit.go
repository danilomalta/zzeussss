package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// RateLimiter retorna um middleware do Fiber para controle de taxa (Rate Limiting).
//
// REGRAS DE NEGÓCIO E DE SEGURANÇA (SecOps):
// 1. Prevenção de Ataques de Força Bruta (Brute-Force): Deve limitar tentativas consecutivas
//    de login (ex: máximo de 5 tentativas por minuto por IP).
// 2. Proteção de DDoS e Abuso de API: Limitar requisições gerais por IP em endpoints sensíveis.
// 3. Persistência Distribuída (Redis): Deve utilizar um cliente Redis para armazenar e incrementar
//    os contadores de requisições de forma atômica e com tempo de expiração (TTL) dinâmico.
// 4. Cabeçalhos de Resposta (Rate Limit Headers): Deve incluir cabeçalhos HTTP padrão na resposta
//    (ex: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset) informando o estado da cota do cliente.
// 5. Tratamento de Erro (HTTP 429): Caso o limite seja excedido, deve bloquear imediatamente o fluxo e retornar
//    status HTTP 429 (Too Many Requests) com uma resposta JSON segura.
func RateLimiter() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// A lógica futura irá:
		// a) Extrair o IP do cliente (c.IP()) ou cabeçalho de proxy confiável (X-Forwarded-For).
		// b) Identificar se o endpoint atual é sensível (ex: /api/v1/auth/login).
		// c) Consultar o Redis incrementando a chave "rate_limit:<IP>:<endpoint>".
		// d) Se o contador exceder o limite (ex: 5 para login, 100 para rotas normais), retornar c.Status(fiber.StatusTooManyRequests).JSON(...)
		// e) Caso contrário, prosseguir com c.Next().
		return c.Next()
	}
}
