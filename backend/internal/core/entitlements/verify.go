// Package entitlements verifica declaracoes de modulos assinadas pelo emissor.
// Nao autentica usuarios e ainda nao persiste ou aplica contratos nas rotas.
package entitlements

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
)

const maxPayload = 16 * 1024

var (
	ErrTrust     = errors.New("emissor nao confiavel")
	ErrSignature = errors.New("assinatura de modulos invalida")
	ErrClaims    = errors.New("declaracao de modulos invalida")
	ErrTenant    = errors.New("contrato de outra empresa")
	ErrValidity  = errors.New("contrato fora da vigencia")
)

// Envelope usa base64 para os campos []byte quando serializado por encoding/json.
// KeyID apenas seleciona uma chave ja confiavel; nunca transporta a chave emissora.
type Envelope struct {
	KeyID     string `json:"key_id"`
	Payload   []byte `json:"payload"`
	Signature []byte `json:"signature"`
}

type Claims struct {
	Version   int          `json:"version"`
	TenantID  string       `json:"tenant_id"`
	Revision  int64        `json:"revision"`
	IssuedAt  int64        `json:"issued_at"`
	NotBefore int64        `json:"not_before"`
	ExpiresAt int64        `json:"expires_at"`
	Modules   []modules.ID `json:"modules"`
}

type Verifier struct {
	keys map[string]ed25519.PublicKey
}

func validKeyID(id string) bool {
	return id != "" && len(id) <= 128 && strings.TrimSpace(id) == id && !strings.ContainsRune(id, 0)
}

// NewVerifier recebe chaves publicas instaladas por um canal confiavel.
// Faz copias para impedir que mutacoes do mapa original alterem a confianca.
func NewVerifier(keys map[string]ed25519.PublicKey) (*Verifier, error) {
	if len(keys) == 0 {
		return nil, ErrTrust
	}
	v := &Verifier{keys: make(map[string]ed25519.PublicKey, len(keys))}
	for id, key := range keys {
		if !validKeyID(id) || len(key) != ed25519.PublicKeySize {
			return nil, ErrTrust
		}
		v.keys[id] = append(ed25519.PublicKey(nil), key...)
	}
	return v, nil
}

// SigningMessage separa este protocolo de outras assinaturas Titan.
// A assinatura cobre o ID da chave e os bytes exatos do payload.
func SigningMessage(keyID string, payload []byte) []byte {
	message := []byte("TitanSystem.modules/v1\x00")
	message = append(message, keyID...)
	message = append(message, 0)
	return append(message, payload...)
}

func decodeClaims(payload []byte) (Claims, error) {
	// Rejeita chaves JSON duplicadas antes de decodificar o contrato.
	scan := json.NewDecoder(bytes.NewReader(payload))
	opening, err := scan.Token()
	if err != nil || opening != json.Delim('{') {
		return Claims{}, ErrClaims
	}
	seen := make(map[string]bool)
	for scan.More() {
		token, err := scan.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return Claims{}, ErrClaims
		}
		seen[key] = true
		var value json.RawMessage
		if err := scan.Decode(&value); err != nil {
			return Claims{}, ErrClaims
		}
	}
	if _, err := scan.Token(); err != nil {
		return Claims{}, ErrClaims
	}
	if _, err := scan.Token(); err != io.EOF {
		return Claims{}, ErrClaims
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var claims Claims
	if err := decoder.Decode(&claims); err != nil {
		return Claims{}, ErrClaims
	}
	return claims, nil
}

// Verify exige a empresa esperada e horario UTC obtidos pelo backend.
// Nao verifica revisao anterior, revogacao remota ou retrocesso do relogio:
// esses controles dependem do armazenamento e da politica offline posteriores.
func (v *Verifier) Verify(envelope Envelope, expectedTenant string, now time.Time) (Claims, error) {
	if v == nil || !validKeyID(envelope.KeyID) {
		return Claims{}, ErrTrust
	}
	key, ok := v.keys[envelope.KeyID]
	if !ok {
		return Claims{}, ErrTrust
	}
	if len(envelope.Payload) == 0 || len(envelope.Payload) > maxPayload || len(envelope.Signature) != ed25519.SignatureSize {
		return Claims{}, ErrSignature
	}
	if !ed25519.Verify(key, SigningMessage(envelope.KeyID, envelope.Payload), envelope.Signature) {
		return Claims{}, ErrSignature
	}
	claims, err := decodeClaims(envelope.Payload)
	if err != nil {
		return Claims{}, err
	}
	if claims.Version != 1 || claims.Revision < 1 || claims.TenantID == "" || strings.TrimSpace(claims.TenantID) != claims.TenantID {
		return Claims{}, ErrClaims
	}
	if expectedTenant == "" || claims.TenantID != expectedTenant {
		return Claims{}, ErrTenant
	}
	if claims.IssuedAt <= 0 || claims.NotBefore < claims.IssuedAt || claims.ExpiresAt <= claims.NotBefore {
		return Claims{}, ErrClaims
	}
	if now.Unix() < claims.NotBefore || now.Unix() >= claims.ExpiresAt {
		return Claims{}, ErrValidity
	}
	seenModules := make(map[modules.ID]bool)
	for _, id := range claims.Modules {
		if seenModules[id] {
			return Claims{}, ErrClaims
		}
		seenModules[id] = true
	}
	if err := modules.Validate(claims.Modules); err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrClaims, err)
	}
	return claims, nil
}
