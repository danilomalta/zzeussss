package fiscalproduction

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestFiscalProviderConfigurationIsolationAndSecrets(t *testing.T) {
	c := TenantFiscalConfiguration{Binding: ProviderBinding{TenantID: "a", StoreID: "shop", DeviceID: "pc", ProfileID: "profile", ProfileRevision: 1}, Provider: "external", Environment: "homologation", CRT: "1"}
	a := RequestContext{Actor: identity.Scope{TenantID: "a", StoreID: "shop"}, Device: identity.DeviceContext{TenantID: "a", StoreID: "shop", DeviceID: "pc"}}
	r, err := NewProviderRegistry([]ProviderRegistration{{Configuration: c}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = r.Resolve(a); !errors.Is(err, ErrUnavailable) {
		t.Fatal("unimplemented external fell back to own")
	}
	if !errors.Is(r.ValidateCheckoutConfiguration(context.Background(), a, 1), ErrCheckoutProduction) {
		t.Fatal("production accepted")
	}
	if _, err = NewProviderRegistry([]ProviderRegistration{{Configuration: c}, {Configuration: c}}); err == nil {
		t.Fatal("duplicate tenant accepted")
	}
	path := filepath.Join(t.TempDir(), "token")
	if err = os.WriteFile(path, []byte("disposable-test-token"), 0644); err != nil {
		t.Fatal(err)
	}
	c.TokenReference = path
	if _, err = NewProviderRegistry([]ProviderRegistration{{Configuration: c}}); err == nil {
		t.Fatal("public token file accepted")
	}
	if err = os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "providers.json")
	raw, _ := json.Marshal(struct {
		Version int                         `json:"version"`
		Tenants []TenantFiscalConfiguration `json:"tenants"`
	}{1, []TenantFiscalConfiguration{c}})
	if err = os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadProviderRegistry(configPath); err != nil {
		t.Fatal(err)
	}
	// Inline secrets are rejected by strict existing configuration decoding.
	if err = os.WriteFile(configPath, []byte(`{"version":1,"tenants":[],"token":"disposable-test-token"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadProviderRegistry(configPath); err == nil {
		t.Fatal("inline secret accepted")
	}
	c.TokenReference = "relative-token"
	if _, err = NewProviderRegistry([]ProviderRegistration{{Configuration: c}}); err == nil {
		t.Fatal("relative secret reference accepted")
	}
}

type scopeTestProvider struct{ FiscalProvider }

func TestFiscalProviderScopeCannotCrossTenantStoreOrDevice(t *testing.T) {
	c := TenantFiscalConfiguration{Binding: ProviderBinding{TenantID: "a", StoreID: "shop", DeviceID: "pc", ProfileID: "profile", ProfileRevision: 1}, Provider: "external", Environment: "homologation", CRT: "1"}
	adapter := &scopeTestProvider{}
	r, err := NewProviderRegistry([]ProviderRegistration{{Configuration: c, Provider: adapter}})
	if err != nil {
		t.Fatal(err)
	}
	valid := RequestContext{Actor: identity.Scope{TenantID: "a", StoreID: "shop"}, Device: identity.DeviceContext{TenantID: "a", StoreID: "shop", DeviceID: "pc"}}
	if got, _, err := r.Resolve(valid); err != nil || got != adapter {
		t.Fatal("valid scope unavailable", err)
	}
	for _, a := range []RequestContext{
		{Actor: identity.Scope{TenantID: "b", StoreID: "shop"}, Device: identity.DeviceContext{TenantID: "b", StoreID: "shop", DeviceID: "pc"}},
		{Actor: identity.Scope{TenantID: "a", StoreID: "other"}, Device: identity.DeviceContext{TenantID: "a", StoreID: "other", DeviceID: "pc"}},
		{Actor: identity.Scope{TenantID: "a", StoreID: "shop"}, Device: identity.DeviceContext{TenantID: "a", StoreID: "shop", DeviceID: "other"}},
	} {
		if _, _, err = r.Resolve(a); !errors.Is(err, ErrUnavailable) {
			t.Fatal("foreign scope accepted")
		}
	}
}
