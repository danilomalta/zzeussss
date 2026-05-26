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

// REGRAS DE NEGÓCIO E DE SEGURANÇA (SecOps) - REFRESH TOKEN:
// 1. Extração Segura: O Refresh Token de entrada deve ser lido de maneira segura pelo handler a partir
//    do cookie HttpOnly de sessão e repassado para o caso de uso.
// 2. Validação de Existência e Expiração: O caso de uso deve validar se o Refresh Token fornecido
//    existe na base de persistência (Banco de Dados/Redis) e se ainda é válido (não expirado).
// 3. Rotação de Refresh Token (SecOps Avançado): Para máxima segurança, deve ser implementada a rotação
//    de Refresh Tokens. Cada vez que um Refresh Token é usado, ele é invalidado e um novo Refresh Token
//    de longo prazo é gerado e retornado.
// 4. Detecção de Reutilização de Refresh Token (Replay Attacks): Se um Refresh Token inválido ou já utilizado
//    for detectado, todo o histórico de tokens daquele usuário deve ser invalidado e todas as sessões ativas
//    do mesmo devem ser encerradas imediatamente (Revocação em Cascata), sob a premissa de que a chave possa ter sido vazada.
// 5. Renovação Silenciosa (Silent Refresh): A emissão do novo par de tokens (Access e Refresh) garante que o usuário
//    permaneça conectado de forma fluida e imperceptível, sem sofrer deslogamentos abruptos enquanto estiver ativo no sistema.
