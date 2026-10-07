package usecase

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
	"titansystem-backend/internal/onlinesessions"

	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/core/security"
	"titansystem-backend/internal/modules/auth/domain"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrSessionUnavailable = errors.New("sessão indisponível")

// LoginInput define os dados necessários para que um usuário tente se autenticar no sistema.
type LoginInput struct {
	// E-mail corporativo do usuário utilizado como identificador primário
	Email string `json:"email"`
	// Senha em texto plano que será validada criptograficamente contra o hash guardado
	Password string `json:"password"`
}

// UserResponse representa os dados públicos do usuário retornados após autenticação.
type UserResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
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
	RefreshToken   string    `json:"-"`
	RefreshExpires time.Time `json:"-"`
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

	if database.DB == nil {
		return nil, ErrSessionUnavailable
	}
	// 2. Busca do usuário pelo e-mail
	var user domain.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionUnavailable
		}
		// [SecOps] Proteção contra enumeração de usuários (retorna erro genérico)
		return nil, errors.New("e-mail ou senha inválidos")
	}

	// 3. Validação criptográfica da senha (Bcrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		// [SecOps] Retorna erro idêntico para evitar inferência de existência de usuário
		return nil, errors.New("e-mail ou senha inválidos")
	}
	active, lookupErr := security.ActiveSession(user.ID, user.ClientID, user.Role)
	if lookupErr != nil {
		return nil, ErrSessionUnavailable
	}
	if !active {
		return nil, errors.New("e-mail ou senha inválidos")
	}

	db, err := database.DB.DB()
	if err != nil {
		return nil, ErrSessionUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tokens, err := onlinesessions.New(db, os.Getenv("JWT_SECRET")).Create(ctx, onlinesessions.Scope{User: user.ID, Tenant: user.ClientID, Role: user.Role}, user.PasswordHash)
	if errors.Is(err, onlinesessions.ErrDenied) {
		return nil, errors.New("e-mail ou senha inválidos")
	}
	if err != nil {
		return nil, ErrSessionUnavailable
	}
	return &LoginOutput{
		AccessToken:    tokens.Access,
		ExpiresIn:      tokens.ExpiresIn,
		RefreshToken:   tokens.Refresh,
		RefreshExpires: tokens.RefreshExpires,
		User: UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Role:     user.Role,
			TenantID: user.ClientID,
		},
	}, nil
}
