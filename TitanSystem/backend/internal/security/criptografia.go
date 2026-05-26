package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrChaveAESInvalida      = errors.New("chave AES inválida (precisa ter 32 bytes para AES-256)")
	ErrEntradaCriptografada  = errors.New("entrada criptografada inválida")
	ErrFalhaAutenticacaoGCM  = errors.New("falha na autenticação do conteúdo criptografado")
)

// CriptografarAES256GCM criptografa o texto claro usando AES-256-GCM.
//
// Formato de retorno:
// - Base64 de: nonce || cifra
//
// Regras:
// - nonce é gerado aleatoriamente a cada chamada (não reutilizar).
func CriptografarAES256GCM(chave []byte, textoClaro string) (string, error) {
	if len(chave) != 32 {
		return "", ErrChaveAESInvalida
	}

	bloco, err := aes.NewCipher(chave)
	if err != nil {
		return "", fmt.Errorf("falha ao criar cifra AES: %w", err)
	}

	gcm, err := cipher.NewGCM(bloco)
	if err != nil {
		return "", fmt.Errorf("falha ao criar modo GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("falha ao gerar nonce: %w", err)
	}

	cifra := gcm.Seal(nil, nonce, []byte(textoClaro), nil)
	empacotado := append(nonce, cifra...)
	return base64.StdEncoding.EncodeToString(empacotado), nil
}

// DescriptografarAES256GCM reverte o resultado de CriptografarAES256GCM.
func DescriptografarAES256GCM(chave []byte, base64NonceCifra string) (string, error) {
	if len(chave) != 32 {
		return "", ErrChaveAESInvalida
	}

	dados, err := base64.StdEncoding.DecodeString(base64NonceCifra)
	if err != nil {
		return "", ErrEntradaCriptografada
	}

	bloco, err := aes.NewCipher(chave)
	if err != nil {
		return "", fmt.Errorf("falha ao criar cifra AES: %w", err)
	}

	gcm, err := cipher.NewGCM(bloco)
	if err != nil {
		return "", fmt.Errorf("falha ao criar modo GCM: %w", err)
	}

	tamanhoNonce := gcm.NonceSize()
	if len(dados) < tamanhoNonce {
		return "", ErrEntradaCriptografada
	}

	nonce := dados[:tamanhoNonce]
	cifra := dados[tamanhoNonce:]

	textoClaro, err := gcm.Open(nil, nonce, cifra, nil)
	if err != nil {
		return "", ErrFalhaAutenticacaoGCM
	}

	return string(textoClaro), nil
}

