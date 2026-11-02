package main

import (
	"errors"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/secretconfig"
)

var errJWTConfiguration = errors.New("JWT_SECRET ausente ou inseguro; configure uma chave privada exclusiva antes de iniciar")

// Validate before connecting to a database or opening any HTTP listener.
func validateStartupSecrets(getenv func(string) string) error {
	if secretconfig.Unsafe(getenv("JWT_SECRET")) {
		return errJWTConfiguration
	}
	_, err := database.Configuration(getenv)
	return err
}
