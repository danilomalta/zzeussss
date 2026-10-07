package usecase

// RefreshTokenInput define a entrada necessária para renovar a sessão do usuário.
type RefreshTokenInput struct {
	// Token de Renovação (Refresh Token) extraído de forma segura a partir do cookie HttpOnly da requisição
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenOutput define a saída contendo as novas credenciais de sessão do usuário.
type RefreshTokenOutput struct {
	// Novo Token de Acesso (JWT) de curto prazo gerado após a validação
	AccessToken string `json:"access_token"`
	// Tempo de expiração do novo AccessToken em segundos
	ExpiresIn int64 `json:"expires_in"`
	// Novo Refresh Token (no caso de rotação) para substituir o anterior
	RefreshToken string `json:"-"`
}

// RefreshTokenUseCase define a assinatura da interface do caso de uso de renovação de token.
type RefreshTokenUseCase interface {
	Execute(input RefreshTokenInput) (*RefreshTokenOutput, error)
}

// A implementação persistida desta entrega usa internal/onlinesessions.
// Refresh é segredo opaco de uso único, não JWT. O handler só emite tokens
// após confirmar sessão, consumo do hash, novo hash e auditoria na transação.
