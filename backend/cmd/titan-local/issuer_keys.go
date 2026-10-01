package main

import (
	"errors"
	"os"

	"titansystem-backend/internal/core/entitlements"
)

func readIssuerVerifier(path string) (*entitlements.Verifier, error) {
	if path == "" {
		return nil, nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("nao foi possivel abrir configuracao de chaves emissoras")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("configuracao de chaves emissoras deve ser arquivo regular")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("nao foi possivel abrir configuracao de chaves emissoras")
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("configuracao de chaves emissoras deve ser arquivo regular")
	}
	verifier, err := entitlements.ReadTrustedKeys(file)
	if err != nil {
		return nil, errors.New("configuracao de chaves emissoras invalida")
	}
	return verifier, nil
}
