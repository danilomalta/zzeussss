package fiscalproduction

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"software.sslmate.com/src/go-pkcs12"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/testsupport/fiscalcert"
)

func testVault(t *testing.T) (*FiscalSecretVault, string) {
	t.Helper()
	t.Setenv("FISCAL_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32)))
	path := filepath.Join(t.TempDir(), "vault.sqlite")
	v, e := OpenFiscalSecretVault(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { v.Close() })
	return v, path
}
func TestVaultEncryptedIsolationAndNoDisclosure(t *testing.T) {
	v, path := testVault(t)
	ctx := context.Background()
	secret := []byte("disposable-secret-never-in-database-or-output")
	var logs bytes.Buffer
	old := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(old)
	for _, kind := range []string{"certificate", "csc", "token"} {
		if e := v.Put(ctx, "a", kind, secret); e != nil {
			t.Fatal(e)
		}
		got, e := v.read(ctx, "a", kind)
		if e != nil || !bytes.Equal(got, secret) {
			t.Fatal("round trip")
		}
		if _, e = v.read(ctx, "b", kind); e == nil {
			t.Fatal("cross tenant read")
		}
	}
	var first, second []byte
	if e := v.db.QueryRow(`SELECT ciphertext FROM fiscal_secret_vault WHERE tenant='a' AND purpose='token'`).Scan(&first); e != nil {
		t.Fatal(e)
	}
	if e := v.Put(ctx, "a", "token", secret); e != nil {
		t.Fatal(e)
	}
	if e := v.db.QueryRow(`SELECT ciphertext FROM fiscal_secret_vault WHERE tenant='a' AND purpose='token'`).Scan(&second); e != nil {
		t.Fatal(e)
	}
	if bytes.Equal(first, second) {
		t.Fatal("nonce reused")
	}
	if _, e := v.db.Exec(`INSERT INTO fiscal_secret_vault VALUES ('b','token',?)`, second); e != nil {
		t.Fatal(e)
	}
	if _, e := v.read(ctx, "b", "token"); e == nil {
		t.Fatal("tenant substitution")
	}
	if _, e := v.db.Exec(`UPDATE fiscal_secret_vault SET ciphertext=? WHERE tenant='a' AND purpose='csc'`, second); e != nil {
		t.Fatal(e)
	}
	if _, e := v.read(ctx, "a", "csc"); e == nil {
		t.Fatal("purpose substitution")
	}
	response := httptest.NewRecorder()
	if e := json.NewEncoder(response).Encode(v); e != nil {
		t.Fatal(e)
	}
	outputs := [][]byte{logs.Bytes(), response.Body.Bytes(), []byte(fmt.Sprintf("%v %#v", v, v))}
	for _, p := range []string{path, path + "-wal", path + "-journal"} {
		data, e := os.ReadFile(p)
		if e == nil {
			outputs = append(outputs, data)
		}
	}
	for _, data := range outputs {
		if bytes.Contains(data, secret) {
			t.Fatal("plaintext disclosure")
		}
	}
}
func TestVaultOwnA1AndMissingKey(t *testing.T) {
	v, path := testVault(t)
	c, e := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if e != nil {
		t.Fatal(e)
	}
	pfx, e := pkcs12.Modern2026.Encode(c.LeafKey, c.Leaf, []*x509.Certificate{c.Root}, disposablePassword)
	if e != nil {
		t.Fatal(e)
	}
	cfg, _ := privatePFXConfiguration(t, c, pfx)
	cfg.PKCS12Path = ""
	cfg.PasswordPath = ""
	bundle, _ := json.Marshal(map[string]any{"configuration": cfg, "pkcs12": pfx, "password": []byte(disposablePassword)})
	if e = v.Put(context.Background(), cfg.Binding.TenantID, "certificate", bundle); e != nil {
		t.Fatal(e)
	}
	b := ProviderBinding{TenantID: cfg.Binding.TenantID, StoreID: cfg.Binding.StoreID, DeviceID: cfg.Binding.DeviceID, ProfileID: cfg.Binding.ProfileID, ProfileRevision: cfg.Binding.ProfileRevision}
	raw, _ := json.Marshal(map[string]any{"version": 1, "vault_path": path, "tenants": []TenantFiscalConfiguration{{Binding: b, Provider: "own", Environment: "production", CRT: "1"}}})
	configPath := filepath.Join(t.TempDir(), "providers.json")
	if e = os.WriteFile(configPath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	registry, e := LoadProviderRegistry(configPath)
	if e != nil {
		t.Fatal(e)
	}
	own := registry.entries[b.TenantID].Provider.(*OwnProvider)
	if own.runtime.credential.Fingerprint() != c.Credential.Fingerprint() {
		t.Fatal("A1 identity changed")
	}
	if registry.ValidateCheckoutConfiguration(context.Background(), RequestContext{}, 1) != ErrCheckoutProduction {
		t.Fatal("production guard")
	}
	unsigned, _, e := fiscalcore.BuildInvalidation(fiscalcore.Invalidation{UF: "SP", CNPJ: cfg.Binding.Issuer, Model: "55", Series: 1, Year: 2026, First: 30, Last: 31, Environment: 2, Reason: "Disposable cryptographic signature check only"})
	if e != nil {
		t.Fatal(e)
	}
	signed, e := own.runtime.credential.SignXML("nfe_invalidation", unsigned, time.Now())
	if e != nil || own.runtime.credential.VerifyXML("nfe_invalidation", signed, time.Now()) != nil {
		t.Fatal("vault signer failed")
	}
	revoked, e := c.CRL(true, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(cfg.CRLPaths[0], revoked, 0600); e != nil {
		t.Fatal(e)
	}
	if own.runtime.Validate() == nil {
		t.Fatal("live revocation lost")
	}
	for _, key := range []string{"", "invalid", base64.StdEncoding.EncodeToString(make([]byte, 16)), base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{11}, 32))} {
		t.Setenv("FISCAL_MASTER_KEY", key)
		if _, e := LoadProviderRegistry(configPath); e == nil {
			t.Fatal("missing/invalid/wrong key accepted")
		}
	}
}
