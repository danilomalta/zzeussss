package entitlements

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
)

// ReadTrustedKeys reads a locally provisioned JSON object mapping key IDs to
// standard base64 public keys. It never obtains trust from a contract request.
// Provisioning and permissions of this configuration are the operator's duty.
func ReadTrustedKeys(reader io.Reader) (*Verifier, error) {
	if reader == nil {
		return nil, ErrTrust
	}
	const maxBytes = 16 * 1024
	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil || len(body) > maxBytes {
		return nil, ErrTrust
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, ErrTrust
	}
	keys := make(map[string]ed25519.PublicKey)
	for decoder.More() {
		token, err := decoder.Token()
		id, ok := token.(string)
		if err != nil || !ok || !validKeyID(id) || len(keys) >= 64 {
			return nil, ErrTrust
		}
		if _, exists := keys[id]; exists {
			return nil, ErrTrust
		}
		var encoded string
		if err := decoder.Decode(&encoded); err != nil {
			return nil, ErrTrust
		}
		key, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, ErrTrust
		}
		keys[id] = ed25519.PublicKey(key)
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, ErrTrust
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrTrust
	}
	return NewVerifier(keys)
}
