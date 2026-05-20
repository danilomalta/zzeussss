package middleware

/*
CORS GUARD (Cross-Origin Resource Sharing)
==========================================
Proteção de Borda. O navegador usa esta política para bloquear requests não autorizados.

Regras de Negócio e Segurança (Nível Bancário):
1. **NUNCA** usar `AllowOrigins: []string{"*"}` em produção.
2. Definir explicitamente o painel web: `https://erp.titansystem.com` e o IP do Mobile.
3. [CRÍTICO] Exigir `AllowCredentials: true` para que o browser trafegue 
   os "HttpOnly Cookies" criados na autenticação.
*/

// CORSConfig dita os poderes que o frontend web possui sobre a API
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool // Obrigatoriamente TRUE em cenários corporativos com Cookies seguros
	MaxAgeSeconds    int  // Tempo de cache para requisições de preflight (OPTIONS)
}

// CorsHandler representa o construtor do middleware
type CorsHandler interface {
	Setup(config CORSConfig) func(ctx interface{}) error
}
