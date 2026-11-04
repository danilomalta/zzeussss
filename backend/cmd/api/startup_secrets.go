package main

import (
	"errors"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/secretconfig"
)

var errJWTConfiguration = errors.New("JWT_SECRET inválida; configure 32 bytes aleatórios exclusivos em hexadecimal minúsculo (64 caracteres) ou base64url sem padding (43 caracteres) antes de iniciar")

// Validate before connecting to a database or opening any HTTP listener.
func validateStartupSecrets(getenv func(string) string) error {
	if !secretconfig.ValidJWTKey(getenv("JWT_SECRET")) {
		return errJWTConfiguration
	}
	_, err := database.Configuration(getenv)
	return err
}
