// Package secretconfig validates explicit credentials without rendering them or
// supplying fallbacks. Passwords and JWT signing keys have separate policies.
package secretconfig

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// ValidJWTKey requires 32 bytes in canonical lowercase hex (existing private lab
// provisioning) or unpadded base64url (existing private recovery-key format).
// Format/length cannot establish entropy or customer exclusivity: provisioners
// must generate fresh bytes with a CSPRNG, never encode a password or reuse a key.
// HS256 signs the canonical text bytes, consistently with the existing signer.
func ValidJWTKey(value string) bool {
	if Unsafe(value) || (len(value) != 43 && len(value) != 64) {
		return false
	}
	var key []byte
	var err error
	if len(value) == 64 {
		key, err = hex.DecodeString(value)
		if err == nil && hex.EncodeToString(key) != value {
			clear(key)
			return false
		}
	} else {
		key, err = base64.RawURLEncoding.Strict().DecodeString(value)
		if err == nil && base64.RawURLEncoding.EncodeToString(key) != value {
			clear(key)
			return false
		}
	}
	defer clear(key)
	if err != nil || len(key) != 32 {
		return false
	}
	// Reject elementary repeated patterns; this is NOT an entropy estimator.
	for period := 1; period <= len(key)/2; period++ {
		repeated := true
		for i := period; i < len(key); i++ {
			if key[i] != key[i%period] {
				repeated = false
				break
			}
		}
		if repeated {
			return false
		}
	}
	return true
}

// GenerateJWTKey is explicit provisioning, never a startup fallback. The caller
// owns private delivery/storage; the key must not be logged or sent over HTTP.
// Matches tools/titan_lab.py's existing secrets.token_hex(32) provisioning.
func GenerateJWTKey() (string, error) {
	var key [32]byte
	defer clear(key[:])
	if _, err := rand.Read(key[:]); err != nil {
		return "", errors.New("geração privada da chave JWT indisponível")
	}
	value := hex.EncodeToString(key[:])
	if !ValidJWTKey(value) {
		return "", errors.New("geração privada da chave JWT indisponível")
	}
	return value, nil
}

// Unsafe is the existing password/example denylist, not a JWT strength test.
func Unsafe(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	// Deny-list only: these public examples must never become runtime defaults.
	switch normalized {
	case "", "super_secret_key_change_in_prod", "titanpass":
		return true
	}
	return false
}
