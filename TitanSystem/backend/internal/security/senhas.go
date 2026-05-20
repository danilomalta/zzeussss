package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrHashSenhaInvalido = errors.New("hash de senha inválido")
)

type ParametrosArgon2id struct {
	MemoriaKB uint32
	Iteracoes uint32
	Paralelo  uint8
	TamanhoSal int
	TamanhoChave int
}

func ParametrosArgon2idPadrao() ParametrosArgon2id {
	return ParametrosArgon2id{
		MemoriaKB:   64 * 1024,
		Iteracoes:   3,
		Paralelo:    2,
		TamanhoSal:  16,
		TamanhoChave: 32,
	}
}

// GerarHashSenhaArgon2id gera um hash no formato:
// argon2id$v=19$m=<memoria>,t=<iteracoes>,p=<paralelo>$<sal_base64>$<chave_base64>
func GerarHashSenhaArgon2id(senha string, p ParametrosArgon2id) (string, error) {
	sal := make([]byte, p.TamanhoSal)
	if _, err := rand.Read(sal); err != nil {
		return "", fmt.Errorf("falha ao gerar sal: %w", err)
	}

	chave := argon2.IDKey([]byte(senha), sal, p.Iteracoes, p.MemoriaKB, p.Paralelo, uint32(p.TamanhoChave))

	salB64 := base64.RawStdEncoding.EncodeToString(sal)
	chaveB64 := base64.RawStdEncoding.EncodeToString(chave)

	return fmt.Sprintf(
		"argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		p.MemoriaKB, p.Iteracoes, p.Paralelo, salB64, chaveB64,
	), nil
}

func VerificarSenhaArgon2id(senha string, hash string) (bool, error) {
	partes := strings.Split(hash, "$")
	if len(partes) != 5 {
		return false, ErrHashSenhaInvalido
	}
	if partes[0] != "argon2id" {
		return false, ErrHashSenhaInvalido
	}

	var versao int
	if _, err := fmt.Sscanf(partes[1], "v=%d", &versao); err != nil || versao != 19 {
		return false, ErrHashSenhaInvalido
	}

	var memoria uint32
	var iteracoes uint32
	var paralelo uint8
	if _, err := fmt.Sscanf(partes[2], "m=%d,t=%d,p=%d", &memoria, &iteracoes, &paralelo); err != nil {
		return false, ErrHashSenhaInvalido
	}

	sal, err := base64.RawStdEncoding.DecodeString(partes[3])
	if err != nil {
		return false, ErrHashSenhaInvalido
	}
	chaveEsperada, err := base64.RawStdEncoding.DecodeString(partes[4])
	if err != nil {
		return false, ErrHashSenhaInvalido
	}

	chave := argon2.IDKey([]byte(senha), sal, iteracoes, memoria, paralelo, uint32(len(chaveEsperada)))
	if subtle.ConstantTimeCompare(chave, chaveEsperada) == 1 {
		return true, nil
	}
	return false, nil
}

// GerarHashSenhaBcrypt gera hash com custo configurável.
func GerarHashSenhaBcrypt(senha string, custo int) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(senha), custo)
	if err != nil {
		return "", fmt.Errorf("falha ao gerar hash bcrypt: %w", err)
	}
	return string(b), nil
}

func VerificarSenhaBcrypt(senha string, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(senha))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	return false, fmt.Errorf("falha ao verificar bcrypt: %w", err)
}

