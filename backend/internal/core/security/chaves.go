package security

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

var ErrChaveAESAusente = errors.New("variável de ambiente TITAN_CHAVE_AES256_GCM_BASE64 não definida")

// CarregarChaveAES256GCM carrega uma chave de 32 bytes a partir de Base64.
//
// Regras:
// - A chave deve ter exatamente 32 bytes (AES-256).
// - A chave deve vir de ambiente/serviço de segredos, nunca ficar fixa no código.
func CarregarChaveAES256GCM() ([]byte, error) {
	valor := os.Getenv("TITAN_CHAVE_AES256_GCM_BASE64")
	if valor == "" {
		return nil, ErrChaveAESAusente
	}

	chave, err := base64.StdEncoding.DecodeString(valor)
	if err != nil {
		return nil, fmt.Errorf("chave Base64 inválida: %w", err)
	}
	if len(chave) != 32 {
		return nil, ErrChaveAESInvalida
	}
	return chave, nil
}

