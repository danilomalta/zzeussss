package middleware

import "time"

/*
AUTH GUARD (JWT & Session Validation)
=====================================
A Espinha Dorsal da Segurança Zero-Trust do TitanSystem.

Regras de Negócio e Segurança:
1. Em vez de ler o `Authorization: Bearer <token>`, deve primariamente ler o token 
   de um **Cookie HttpOnly e Secure**. Isso imunda completamente ataques de roubo 
   de JWT via JavaScript malicioso (XSS).
2. Valida a assinatura do token com chave assimétrica (RS256) ou HS256 com chave forte.
3. Verifica se o "Role" do usuário bate com os níveis de acesso exigidos pela rota.
*/

// TokenPayload representa as claims embutidas no JWT gerado após o Login
type TokenPayload struct {
	UserID    string
	Role      string    // Ex: "admin", "cashier", "manager"
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// AuthGuard assina o contrato de validação das rotas trancadas
type AuthGuard interface {
	VerifySession() func(ctx interface{}) error
	RequireRole(roles ...string) func(ctx interface{}) error
	ExtractTokenFromCookie(ctx interface{}) (string, error)
}
