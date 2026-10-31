package fiscalproduction

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"unicode/utf8"

	"software.sslmate.com/src/go-pkcs12"
)

// Signing sources are mutually exclusive. Password bytes are never returned,
// logged, accepted via HTTP, or stored in the durable fiscal journal.
func loadSigningMaterial(cfg FileConfiguration) ([][]byte, crypto.Signer, error) {
	if cfg.PKCS12Path != "" || cfg.PasswordPath != "" {
		if cfg.PKCS12Path == "" || cfg.PasswordPath == "" || cfg.CertificatePath != "" || cfg.PrivateKeyPath != "" {
			return nil, nil, ErrUnavailable
		}
		raw, err := privateFile(cfg.PKCS12Path)
		if err != nil {
			return nil, nil, ErrUnavailable
		}
		defer clear(raw)
		password, err := privateFile(cfg.PasswordPath)
		if err != nil {
			return nil, nil, ErrUnavailable
		}
		defer clear(password)
		if len(password) == 0 || len(password) > 1024 || !utf8.Valid(password) || strings.IndexByte(string(password), 0) >= 0 {
			return nil, nil, ErrUnavailable
		}
		// DecodeChain requires an authenticated, single-identity DER PFX. CAs
		// embedded by the issuer remain intermediates, never trust anchors.
		key, leaf, certificates, err := pkcs12.DecodeChain(raw, string(password))
		if err != nil || leaf == nil || leaf.IsCA || len(certificates) > 8 {
			return nil, nil, ErrUnavailable
		}
		signer, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, ErrUnavailable
		}
		chain := [][]byte{leaf.Raw}
		for _, certificate := range certificates {
			chain = append(chain, certificate.Raw)
		}
		return chain, signer, nil
	}
	if cfg.CertificatePath == "" || cfg.PrivateKeyPath == "" {
		return nil, nil, ErrUnavailable
	}
	certRaw, err := privateFile(cfg.CertificatePath)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	var chain [][]byte
	for len(certRaw) > 0 {
		block, rest := pem.Decode(certRaw)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, nil, ErrUnavailable
		}
		chain = append(chain, block.Bytes)
		certRaw = rest
	}
	if len(chain) == 0 || len(chain) > 9 {
		return nil, nil, ErrUnavailable
	}
	keyRaw, err := privateFile(cfg.PrivateKeyPath)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	defer clear(keyRaw)
	block, rest := pem.Decode(keyRaw)
	if block == nil || len(rest) != 0 {
		return nil, nil, ErrUnavailable
	}
	defer clear(block.Bytes)
	var key any
	switch block.Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, nil, ErrUnavailable
	}
	signer, ok := key.(*rsa.PrivateKey)
	if err != nil || !ok {
		return nil, nil, ErrUnavailable
	}
	return chain, signer, nil
}
