package entitlements_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
)

type fixture struct {
	verifier *entitlements.Verifier
	private  ed25519.PrivateKey
	claims   entitlements.Claims
	now      time.Time
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"issuer-1": public})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000000000, 0).UTC()
	return fixture{verifier, private, entitlements.Claims{
		Version: 1, TenantID: "market-a", Revision: 1,
		IssuedAt: now.Unix() - 60, NotBefore: now.Unix() - 60, ExpiresAt: now.Unix() + 60,
		Modules: []modules.ID{modules.Core, modules.Inventory, modules.POS},
	}, now}
}

func signed(t *testing.T, f fixture) entitlements.Envelope {
	t.Helper()
	payload, err := json.Marshal(f.claims)
	if err != nil {
		t.Fatal(err)
	}
	return rawSigned(f, payload)
}

func rawSigned(f fixture, payload []byte) entitlements.Envelope {
	return entitlements.Envelope{KeyID: "issuer-1", Payload: payload,
		Signature: ed25519.Sign(f.private, entitlements.SigningMessage("issuer-1", payload))}
}

func TestValidContractIsBoundToCompany(t *testing.T) {
	f := newFixture(t)
	envelope := signed(t, f)
	claims, err := f.verifier.Verify(envelope, "market-a", f.now)
	if err != nil || claims.Revision != 1 {
		t.Fatalf("contrato valido: %+v %v", claims, err)
	}
	if err := modules.Require(modules.POS, claims.Modules); err != nil {
		t.Fatal(err)
	}
	if _, err := f.verifier.Verify(envelope, "market-b", f.now); !errors.Is(err, entitlements.ErrTenant) {
		t.Fatalf("contrato cruzou empresas: %v", err)
	}
}

func TestTamperingCannotEnableAnotherModule(t *testing.T) {
	f := newFixture(t)
	envelope := signed(t, f)
	f.claims.Modules = append(f.claims.Modules, modules.Production)
	modified, err := json.Marshal(f.claims)
	if err != nil {
		t.Fatal(err)
	}
	envelope.Payload = modified
	if _, err := f.verifier.Verify(envelope, "market-a", f.now); !errors.Is(err, entitlements.ErrSignature) {
		t.Fatalf("payload adulterado aceito: %v", err)
	}
}

func TestUntrustedKeyAndForeignSignatureAreRejected(t *testing.T) {
	f := newFixture(t)
	envelope := signed(t, f)
	envelope.KeyID = "untrusted"
	if _, err := f.verifier.Verify(envelope, "market-a", f.now); !errors.Is(err, entitlements.ErrTrust) {
		t.Fatalf("emissor nao confiavel aceito: %v", err)
	}
	envelope = signed(t, f)
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	envelope.Signature = ed25519.Sign(private, entitlements.SigningMessage(envelope.KeyID, envelope.Payload))
	if _, err := f.verifier.Verify(envelope, "market-a", f.now); !errors.Is(err, entitlements.ErrSignature) {
		t.Fatalf("chave de outra pessoa aceita: %v", err)
	}
}

func TestValidityBoundaries(t *testing.T) {
	f := newFixture(t)
	envelope := signed(t, f)
	for _, stamp := range []int64{f.claims.NotBefore - 1, f.claims.ExpiresAt, f.claims.ExpiresAt + 1} {
		if _, err := f.verifier.Verify(envelope, "market-a", time.Unix(stamp, 0)); !errors.Is(err, entitlements.ErrValidity) {
			t.Fatalf("vigencia indevida em %d: %v", stamp, err)
		}
	}
	for _, stamp := range []int64{f.claims.NotBefore, f.claims.ExpiresAt - 1} {
		if _, err := f.verifier.Verify(envelope, "market-a", time.Unix(stamp, 0)); err != nil {
			t.Fatalf("vigencia valida rejeitada: %v", err)
		}
	}
}

func TestSignedButInvalidModuleConfigurationIsRejected(t *testing.T) {
	for _, ids := range [][]modules.ID{
		{modules.Core, modules.POS},
		{modules.Core, "unknown"},
		{modules.Core, modules.Core},
	} {
		f := newFixture(t)
		f.claims.Modules = ids
		if _, err := f.verifier.Verify(signed(t, f), "market-a", f.now); !errors.Is(err, entitlements.ErrClaims) {
			t.Fatalf("configuracao assinada invalida aceita: %v %v", ids, err)
		}
	}
}

func TestUnknownVersionInvalidRevisionAndMalformedJSONAreRejected(t *testing.T) {
	f := newFixture(t)
	f.claims.Version = 2
	if _, err := f.verifier.Verify(signed(t, f), "market-a", f.now); !errors.Is(err, entitlements.ErrClaims) {
		t.Fatalf("versao desconhecida aceita: %v", err)
	}
	f = newFixture(t)
	f.claims.Revision = 0
	if _, err := f.verifier.Verify(signed(t, f), "market-a", f.now); !errors.Is(err, entitlements.ErrClaims) {
		t.Fatalf("revisao invalida aceita: %v", err)
	}
	for _, payload := range []string{
		`{"tenant_id":"market-a","tenant_id":"market-b"}`,
		`{} {}`,
		`[]`,
		`{"unexpected":true}`,
	} {
		if _, err := f.verifier.Verify(rawSigned(f, []byte(payload)), "market-a", f.now); !errors.Is(err, entitlements.ErrClaims) {
			t.Fatalf("JSON invalido aceito: %s %v", payload, err)
		}
	}
}

func TestVerifierCopiesTrustedPublicKeys(t *testing.T) {
	f := newFixture(t)
	public := f.private.Public().(ed25519.PublicKey)
	keys := map[string]ed25519.PublicKey{"issuer-1": public}
	verifier, err := entitlements.NewVerifier(keys)
	if err != nil {
		t.Fatal(err)
	}
	public[0] ^= 1
	delete(keys, "issuer-1")
	if _, err := verifier.Verify(signed(t, f), "market-a", f.now); err != nil {
		t.Fatalf("configuracao externa alterou confianca: %v", err)
	}
	if _, err := entitlements.NewVerifier(nil); !errors.Is(err, entitlements.ErrTrust) {
		t.Fatalf("verificador sem chave aceito: %v", err)
	}
}
