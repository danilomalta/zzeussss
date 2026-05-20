package middleware

/*
RATE LIMITER MIDDLEWARE (DDoS & Brute Force Protection)
======================================================
Atua como a primeira barreira do sistema ("Escudo Frontal").
Regras de Negócio e Segurança:
1. Implementa o algoritmo de Token Bucket ou Leaky Bucket em memória ou via Redis.
2. Identifica o tráfego via `Real-IP` ou `X-Forwarded-For` de forma estrita.
3. [CRÍTICO] Rotas como `/login` ou `/checkout` devem possuir regras (limitadores) 
   muito mais restritos (ex: máximo de 5 tentativas de login por minuto) 
   do que rotas de leitura como `/products`.
*/

// RateLimiterConfig define os parâmetros de tolerância do servidor
type RateLimiterConfig struct {
	MaxRequestsPerSecond int
	BurstTolerance       int
	BlockDurationSeconds int
}

// RateLimiter é a assinatura base que será implementada pelo Fiber/Gin/Echo
type RateLimiter interface {
	Apply(config RateLimiterConfig) func(ctx interface{}) error
	BlacklistIP(ip string, duration string) error
}
