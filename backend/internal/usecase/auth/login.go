package auth

// LoginInput define os dados necessários para que um usuário tente se autenticar no sistema.
type LoginInput struct {
	// E-mail corporativo do usuário utilizado como identificador primário
	Email string `json:"email"`
	// Senha em texto plano que será validada criptograficamente contra o hash guardado
	Password string `json:"password"`
}

// UserResponse representa os dados públicos do usuário retornados após autenticação.
type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// LoginOutput define os tokens e dados de sessão gerados após sucesso na autenticação.
type LoginOutput struct {
	// Token de Acesso de curto prazo para chamadas autenticadas de API
	AccessToken string `json:"access_token"`
	// Tempo de expiração do AccessToken em segundos
	ExpiresIn int64 `json:"expires_in"`
	// Dados resumidos do usuário autenticado para consumo imediato do frontend
	User UserResponse `json:"user"`
	// Refresh Token de longo prazo (será injetado em um Cookie HttpOnly para segurança extra)
	RefreshToken string `json:"-"`
}

// LoginUseCase define a assinatura da interface do caso de uso de Login.
type LoginUseCase interface {
	Execute(input LoginInput) (*LoginOutput, error)
}

// REGRAS DE NEGÓCIO E DE SEGURANÇA (SecOps) - LOGIN:
// 1. Sanitização e Validação: Os campos de entrada (Email e Password) devem ser limpos (trim)
//    e validados sintaticamente (verificar formato de e-mail, comprimento de senha) antes de prosseguir.
// 2. Busca e Verificação de Hash: O usuário deve ser buscado no banco de dados. A senha fornecida
//    deve ser validada utilizando um algoritmo de hashing robusto (como bcrypt.CompareHashAndPassword ou Argon2id)
//    contra o hash salvo. Em caso de credencial inválida, deve-se responder com uma mensagem genérica
//    (ex: "usuário ou senha inválidos") para evitar Enumeração de Usuários (User Enumeration).
// 3. Geração de Access Token (JWT): Gerar um token JWT de curto prazo (expiração máxima recomendada: 15 minutos).
//    Esse token deve conter claims básicas (sub/userID, role, exp, iat) assinadas criptograficamente com o JWT_SECRET.
//    O AccessToken é devolvido no corpo da resposta JSON para armazenamento seguro na memória da aplicação do cliente.
// 4. Geração de Refresh Token: Criar uma string criptograficamente segura e aleatória (ex: UUIDv4 de alta entropia)
//    com expiração de longo prazo (ex: 7 dias), persistida no banco ou cache como ativa associada ao UserID.
//    O Refresh Token NUNCA deve ser retornado no corpo da resposta JSON do endpoint público principal.
// 5. Armazenamento Seguro (HttpOnly Cookie): O Refresh Token gerado deve ser enviado de volta ao cliente via
//    cabeçalho HTTP 'Set-Cookie' utilizando flags estritas de proteção:
//    - HttpOnly (impede leitura por scripts maliciosos, anulando ataques de roubo via XSS).
//    - Secure (obriga tráfego apenas sobre HTTPS).
//    - SameSite=Lax ou SameSite=Strict (mitiga ataques de CSRF).
