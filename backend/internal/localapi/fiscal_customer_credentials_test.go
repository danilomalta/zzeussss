package localapi

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/fiscalproduction"
)

func installCustomerProviderApp(t *testing.T, f *stockHTTPFixture, path string) {
	t.Helper()
	registry, err := fiscalproduction.LoadProviderRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithFiscalProvidersAndGate(f.db, f.device, verifier, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
}
func TestHTTPFiscalCustomerCredentialsOwnerRecoveryAndNoDisclosure(t *testing.T) {
	f := fiscalFixture(t)
	var logs bytes.Buffer
	priorLogger := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(priorLogger)
	t.Setenv("FISCAL_MASTER_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{47}, 32)))
	dir := t.TempDir()
	vault := filepath.Join(dir, "private.sqlite")
	config := filepath.Join(dir, "providers.json")
	raw, _ := json.Marshal(map[string]any{"version": 1, "vault_path": vault, "tenants": []fiscalproduction.TenantFiscalConfiguration{{CustomerManaged: true, Provider: "external", Environment: "homologation", CRT: "1", Binding: fiscalproduction.ProviderBinding{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, DeviceID: f.owner.DeviceID, ProfileID: "issuer-one", ProfileRevision: 1}}}})
	if err := os.WriteFile(config, raw, 0600); err != nil {
		t.Fatal(err)
	}
	installCustomerProviderApp(t, f, config)
	path := "/local/v1/fiscal/onboarding/issuer-one/provider"
	code, body := request(t, f.app, "GET", path, "", f.token)
	if code != 200 || !bytes.Contains(body, []byte(`"revision":0`)) {
		t.Fatal("empty status", code)
	}
	input := `{"operation_id":"customer-first","expected_revision":0,"profile_revision":1,"token":"PrivateFocusToken123","csc":"PrivateCSC123"}`
	code, body = request(t, f.app, "POST", path, input, f.token)
	if code != 200 {
		t.Fatal("save status", code, string(body))
	}
	var out fiscalproduction.CustomerCredentialStatus
	if json.Unmarshal(body, &out) != nil || out.Revision != 1 || !out.TokenPresent || !out.CSCPresent || !out.ProductionBlocked {
		t.Fatal("unsafe status")
	}
	installCustomerProviderApp(t, f, config)
	code, recovered := request(t, f.app, "GET", path, "", f.token)
	if code != 200 || !bytes.Equal(body, recovered) {
		t.Fatal("restart recovery", code)
	}
	code, replayed := request(t, f.app, "POST", path, input, f.token)
	if code != 200 || !bytes.Equal(body, replayed) {
		t.Fatal("replay", code)
	}
	file, _ := os.ReadFile(vault)
	for _, data := range [][]byte{body, recovered, replayed, file, logs.Bytes()} {
		if bytes.Contains(data, []byte("PrivateFocusToken123")) || bytes.Contains(data, []byte("PrivateCSC123")) {
			t.Fatal("credential disclosure")
		}
	}
	for _, bad := range []string{
		strings.Replace(input, `"customer-first"`, `"different-operation"`, 1),
		strings.Replace(input, `"profile_revision":1`, `"profile_revision":2`, 1),
	} {
		if code, _ := request(t, f.app, "POST", path, bad, f.token); code != 409 {
			t.Fatal("conflict status", code)
		}
	}
	for _, bad := range []string{
		strings.Replace(input, `"expected_revision":0`, `"expected_revision":null`, 1),
		strings.Replace(input, `"expected_revision":0`, `"expected_revision":0,"expected_revision":1`, 1),
		strings.Replace(input, `"token":`, `"tenant_id":"other","token":`, 1),
		strings.Replace(input, `"token":`, `"vault_path":"/tmp/foreign","token":`, 1),
	} {
		if code, body := request(t, f.app, "POST", path, bad, f.token); code != 400 || bytes.Contains(body, []byte("Private")) {
			t.Fatal("unsafe input", code)
		}
	}
	if code, _ := request(t, f.app, "GET", path+"?token=PrivateFocusToken123", "", f.token); code != 400 {
		t.Fatal("query admission", code)
	}
	if code, _ := request(t, f.app, "POST", path, input, ""); code != 401 {
		t.Fatal("unauthenticated", code)
	}
	for _, role := range []string{"manager", "accountant", "cashier"} {
		code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"credential-`+role+`","identity_id":"`+role+`","name":"Synthetic","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
		if code != 201 {
			t.Fatal("staff", code)
		}
		code, body := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"`+role+`","password":"test-password-strong-1"}`, "")
		if code != 200 {
			t.Fatal("login", code)
		}
		var auth struct {
			Token string `json:"token"`
		}
		if json.Unmarshal(body, &auth) != nil {
			t.Fatal("login format")
		}
		for _, method := range []string{"GET", "POST"} {
			data := ""
			if method == "POST" {
				data = input
			}
			if code, _ := request(t, f.app, method, path, data, auth.Token); code != 403 {
				t.Fatal("non-owner credential access", role, method, code)
			}
		}
	}
	code, _ = request(t, f.app, "POST", "/local/v1/logout", `{}`, f.token)
	if code != 204 {
		t.Fatal("logout", code)
	}
	if code, _ := request(t, f.app, "POST", path, input, f.token); code != 401 {
		t.Fatal("revoked session accepted", code)
	}
}
