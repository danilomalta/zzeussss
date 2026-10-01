package entitlements

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/core/modules"
)

func TestReadTrustedKeysVerifiesConfiguredIssuer(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(public)
	verifier, err := ReadTrustedKeys(strings.NewReader(`{"issuer":"` + encoded + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(Claims{Version: 1, TenantID: "company", Revision: 1, IssuedAt: 100, NotBefore: 100, ExpiresAt: 200, Modules: []modules.ID{modules.Core}})
	if err != nil {
		t.Fatal(err)
	}
	envelope := Envelope{KeyID: "issuer", Payload: payload, Signature: ed25519.Sign(private, SigningMessage("issuer", payload))}
	if _, err := verifier.Verify(envelope, "company", time.Unix(150, 0)); err != nil {
		t.Fatal(err)
	}
}

func TestReadTrustedKeysRejectsAmbiguousOrMalformedConfiguration(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(make([]byte, ed25519.PublicKeySize))
	for _, body := range []string{
		`{}`, `null`, `[]`, `{"issuer":null}`, `{"issuer":{}}`, `{"issuer":"bad"}`,
		`{"issuer":"` + encoded + `","issuer":"` + encoded + `"}`,
		`{" issuer":"` + encoded + `"}`, `{"issuer":"` + encoded + `"} {}`,
		`{"issuer":"` + base64.StdEncoding.EncodeToString(make([]byte, ed25519.PrivateKeySize)) + `"}`,
		strings.Repeat(" ", 16385),
	} {
		if _, err := ReadTrustedKeys(strings.NewReader(body)); !errors.Is(err, ErrTrust) {
			t.Fatalf("configuracao aceita: erro=%v", err)
		}
	}
	if _, err := ReadTrustedKeys(nil); !errors.Is(err, ErrTrust) {
		t.Fatalf("reader nil: %v", err)
	}
}
