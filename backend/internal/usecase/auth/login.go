package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"titansystem-backend/internal/database"
	"titansystem-backend/internal/models"
)

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

// loginUseCaseImpl é a implementação real da lógica de negócio e segurança de autenticação.
type loginUseCaseImpl struct{}

// NewLoginUseCase instancia uma implementação real do LoginUseCase.
func NewLoginUseCase() LoginUseCase {
	return &loginUseCaseImpl{}
}

// Execute valida as credenciais, verifica a criptografia da senha e gera os tokens de sessão.
func (u *loginUseCaseImpl) Execute(input LoginInput) (*LoginOutput, error) {
	// 1. Sanitização e Validação básica
	email := strings.TrimSpace(strings.ToLower(input.Email))
	if email == "" || input.Password == "" {
		return nil, errors.New("e-mail ou senha inválidos")
	}

	// 2. Busca do usuário pelo e-mail
	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// [SecOps] Proteção contra enumeração de usuários (retorna erro genérico)
		return nil, errors.New("e-mail ou senha inválidos")
	}

	// 3. Validação criptográfica da senha (Bcrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		// [SecOps] Retorna erro idêntico para evitar inferência de existência de usuário
		return nil, errors.New("e-mail ou senha inválidos")
	}

	// 4. Emissão do Access Token (JWT - expira em 15 minutos)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "super_secret_key_change_in_prod"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role,
		"name": user.Name,
		"exp":  time.Now().Add(15 * time.Minute).Unix(),
		"iat":  time.Now().Unix(),
	})

	accessToken, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return nil, fmtError("falha ao assinar token de acesso: %w", err)
	}

	// 5. Emissão do Refresh Token (String criptograficamente segura - 32 bytes)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmtError("falha ao gerar refresh token: %w", err)
	}
	refreshToken := hex.EncodeToString(b)

	// Duração de expiração em segundos (15 minutos = 900 segundos)
	expiresIn := int64(15 * 60)

	return &LoginOutput{
		AccessToken:  accessToken,
		ExpiresIn:    expiresIn,
		RefreshToken: refreshToken,
		User: UserResponse{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
			Role:  user.Role,
		},
	}, nil
}

// fmtError é um pequeno utilitário local para evitar quebra com imports e manter legibilidade
func fmtError(format string, err error) error {
	return errors.New(strings.Replace(format, "%w", err.Error(), 1))
}
