package fiscal

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"titansystem-backend/internal/archivecrypto"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/identity"
)

const nativeJournalPrefix = "titan-fiscal-journal:v1:"

var ErrJournalKey = errors.New("customer fiscal journal encryption key unavailable")
var ErrJournalLegacy = errors.New("legacy fiscal journal requires explicit customer read compatibility")

type nativeJournalEnvelope struct {
	KeyID string `json:"key_id"`
	Data  []byte `json:"ciphertext"`
}

func nativeJournalKey(path string) (cipher.AEAD, string, error) {
	if !filepath.IsAbs(path) {
		return nil, "", ErrJournalKey
	}
	key, err := archivecrypto.ReadKey(path)
	if err != nil {
		return nil, "", ErrJournalKey
	}
	defer clear(key)
	sum := sha256.Sum256(key)
	aead, err := archivecrypto.NewCipher(key)
	if err != nil {
		return nil, "", ErrJournalKey
	}
	return aead, hex.EncodeToString(sum[:]), nil
}

func nativeJournalAAD(a identity.Scope, id, kind, keyID string) []byte {
	raw, _ := json.Marshal([]string{"titan-fiscal-journal-v1", a.TenantID, a.StoreID, id, kind, keyID})
	return raw
}

func nativeJournalPaths() ([]string, error) {
	paths := []string{os.Getenv("TITAN_FISCAL_JOURNAL_KEY_FILE")}
	if previous := os.Getenv("TITAN_FISCAL_JOURNAL_PREVIOUS_KEY_FILES"); previous != "" {
		var historical []string
		if len(previous) > 32768 || json.Unmarshal([]byte(previous), &historical) != nil || len(historical) > 16 {
			return nil, ErrJournalKey
		}
		paths = append(paths, historical...)
	}
	return paths, nil
}

// No key is generated or guessed here. The customer provisions a private
// 32-byte key using the existing backup key-init command and owns its backup.
func sealNativeJournal(a identity.Scope, id, kind, raw string) (string, error) {
	paths, err := nativeJournalPaths()
	if err != nil {
		return "", err
	}
	crypt, keyID, err := nativeJournalKey(paths[0])
	if err != nil {
		return "", err
	}
	nonce := make([]byte, crypt.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", ErrJournalKey
	}
	sealed := crypt.Seal(nonce, nonce, []byte(raw), nativeJournalAAD(a, id, kind, keyID))
	envelope, err := json.Marshal(nativeJournalEnvelope{KeyID: keyID, Data: sealed})
	if err != nil {
		return "", ErrJournalKey
	}
	return nativeJournalPrefix + string(envelope), nil
}

func openNativeJournal(a identity.Scope, id, kind, raw string) (string, error) {
	if !strings.HasPrefix(raw, nativeJournalPrefix) {
		// Never silently downgrade encrypted rows to plaintext. The customer may
		// explicitly allow historical reads; this never permits plaintext writes.
		// Existing immutable rows are not rewritten or deleted.
		if strings.HasPrefix(raw, "titan-fiscal-journal:") {
			return "", ErrIntegrity
		}
		if os.Getenv("TITAN_FISCAL_JOURNAL_LEGACY_READ") != "true" {
			return "", ErrJournalLegacy
		}
		return raw, nil
	}
	if len(raw) > 8*fiscalcore.MaxXML {
		return "", ErrIntegrity
	}
	var envelope nativeJournalEnvelope
	if fiscalcore.DecodeInput([]byte(strings.TrimPrefix(raw, nativeJournalPrefix)), &envelope) != nil || len(envelope.KeyID) != 64 {
		return "", ErrIntegrity
	}
	paths, err := nativeJournalPaths()
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		crypt, keyID, err := nativeJournalKey(path)
		if err != nil || keyID != envelope.KeyID {
			continue
		}
		n := crypt.NonceSize()
		if len(envelope.Data) < n+crypt.Overhead() {
			return "", ErrIntegrity
		}
		plain, err := crypt.Open(nil, envelope.Data[:n], envelope.Data[n:], nativeJournalAAD(a, id, kind, keyID))
		if err != nil {
			return "", ErrIntegrity
		}
		defer clear(plain)
		return string(plain), nil
	}
	return "", ErrJournalKey
}

func decodeNativeJournal(a identity.Scope, id, kind string, raw *string, into any) error {
	plain, err := openNativeJournal(a, id, kind, *raw)
	if err != nil {
		return err
	}
	if fiscalcore.DecodeInput([]byte(plain), into) != nil {
		return ErrIntegrity
	}
	*raw = plain
	return nil
}

func sameNativeJournal(a identity.Scope, id, kind, stored, canonical string) bool {
	plain, err := openNativeJournal(a, id, kind, stored)
	return err == nil && plain == canonical
}
