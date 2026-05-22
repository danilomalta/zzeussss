package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// CORS retorna um middleware do Fiber para configuração da política de compartilhamento de recursos (CORS).
//
// REGRAS DE NEGÓCIO E DE SEGURANÇA (SecOps):
// 1. Restrição Estrita de Origem (Origin Restriction): Não deve utilizar o curinga "*" em produção.
//    As origens permitidas devem ser lidas dinamicamente a partir das variáveis de ambiente (ex: VITE_API_URL, etc.).
// 2. Suporte a Credenciais (AllowCredentials): Deve explicitamente permitir credenciais (true) para viabilizar
//    o tráfego seguro de tokens de sessão via Cookies seguros (HttpOnly, Secure, SameSite=Lax/Strict).
// 3. Métodos e Cabeçalhos Permitidos (Allowed Methods & Headers): Restringir estritamente aos verbos HTTP necessários
//    (GET, POST, PUT, DELETE, OPTIONS) e aos cabeçalhos de requisição autorizados (Content-Type, Authorization).
// 4. Cache de Preflight (MaxAge): Configurar o cabeçalho 'Access-Control-Max-Age' para cachear requisições OPTIONS
//    prévias, otimizando o tráfego e a latência de rede.
func CORS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// A lógica futura irá:
		// a) Ler a lista de origens permitidas definidas em variáveis de ambiente.
		// b) Validar a origem da requisição (c.Get("Origin")) contra a lista de permitidas.
		// c) Se permitida, definir os cabeçalhos:
		//    - Access-Control-Allow-Origin: <origem_da_requisicao>
		//    - Access-Control-Allow-Credentials: true
		//    - Access-Control-Allow-Headers: Content-Type, Authorization, X-Requested-With
		//    - Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
		// d) Responder com status 204 (No Content) se for uma requisição do tipo OPTIONS (Preflight).
		// e) Prosseguir com c.Next().
		return c.Next()
	}
}
