package service

import (
	"errors"

	"github.com/titan/backend/internal/auth/domain"
	"github.com/titan/backend/internal/auth/repository"
)

type AuthService interface {
	Login(email, password string) (*domain.User, error)
}

type authService struct {
	repo repository.UserRepository
}

func NewAuthService(repo repository.UserRepository) AuthService {
	return &authService{repo: repo}
}

func (s *authService) Login(email, password string) (*domain.User, error) {
	user, err := s.repo.FindByEmail(email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if err := user.ValidatePassword(password); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}
