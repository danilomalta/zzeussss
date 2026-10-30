package fiscalproduction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/fiscal"
)

func customerFixture(t *testing.T) (*ProviderRegistry, RequestContext, string) {
	t.Helper()
	a := focusContext(t)
	_, path := testVault(t)
	p, err := fiscal.Configure(context.Background(), a.DB, a.License, a.Actor, a.Device, fiscal.ProfileInput{OperationID: "customer-profile", ProfileID: "profile", CNPJ: "11222333000181", LegalName: "Synthetic client", State: "SP", Municipality: "3550308", StateRegistration: "123456789", CRT: "1", Model: "65", Environment: "homologation", Series: 1}, a.Token)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewProviderRegistry([]ProviderRegistration{{Configuration: TenantFiscalConfiguration{CustomerManaged: true, Binding: ProviderBinding{TenantID: a.Actor.TenantID, StoreID: a.Actor.StoreID, DeviceID: a.Device.DeviceID, ProfileID: p.Input.ProfileID, ProfileRevision: 1}, Provider: "external", Environment: "homologation", CRT: "1"}}})
	if err != nil {
		t.Fatal(err)
	}
	registry.vaultPath = path
	return registry, a, path
}
func TestCustomerCredentialsEncryptedRestartRecoveryAndProductionBlocked(t *testing.T) {
	ctx := context.Background()
	r, a, path := customerFixture(t)
	in := CustomerCredentialInput{OperationID: "client-first", ProfileRevision: 1, Token: []byte("PrivateCustomerFocusToken123"), CSC: []byte("PrivateCustomerCSC123")}
	initial, err := r.ReadCustomerCredentials(ctx, a, "profile")
	if err != nil || initial.Revision != 0 || !initial.ProductionBlocked {
		t.Fatal("initial status", err)
	}
	saved, err := r.SaveCustomerCredentials(ctx, a, "profile", in)
	if err != nil || saved.Revision != 1 || !saved.TokenPresent || !saved.CSCPresent || !saved.ProductionBlocked {
		t.Fatal("save", err)
	}
	restarted, err := NewProviderRegistry([]ProviderRegistration{r.entries[a.Actor.TenantID]})
	if err != nil {
		t.Fatal(err)
	}
	restarted.vaultPath = path
	recovered, err := restarted.ReadCustomerCredentials(ctx, a, "profile")
	if err != nil || recovered != saved {
		t.Fatal("restart recovery", err)
	}
	provider, _, err := restarted.Resolve(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := provider.(*FocusProvider); !ok {
		t.Fatal("not Focus")
	}
	replay, err := restarted.SaveCustomerCredentials(ctx, a, "profile", in)
	if err != nil || replay != saved {
		t.Fatal("idempotency", err)
	}
	for _, patch := range []func(*CustomerCredentialInput){func(i *CustomerCredentialInput) { i.Token = []byte("ChangedSecret") }, func(i *CustomerCredentialInput) { i.OperationID = "different-attempt" }, func(i *CustomerCredentialInput) { i.ProfileRevision = 2 }} {
		changed := in
		patch(&changed)
		if _, err := restarted.SaveCustomerCredentials(ctx, a, "profile", changed); !errors.Is(err, fiscal.ErrConflict) {
			t.Fatal("conflict not rejected", err)
		}
	}
	foreign := a
	foreign.Actor.TenantID = "other"
	if _, err := r.ReadCustomerCredentials(ctx, foreign, "profile"); err == nil {
		t.Fatal("cross-tenant access")
	}
	expired := a
	expired.Token = "expired"
	if _, err := r.SaveCustomerCredentials(ctx, expired, "profile", in); err == nil {
		t.Fatal("expired session accepted")
	}
	raw, _ := json.Marshal(saved)
	dbRaw, _ := os.ReadFile(path)
	for _, secret := range [][]byte{in.Token, in.CSC} {
		if bytes.Contains(raw, secret) || bytes.Contains(dbRaw, secret) {
			t.Fatal("plaintext leaked")
		}
	}
	profile, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, "profile", 0)
	if err != nil {
		t.Fatal(err)
	}
	profile.Input.OperationID = "production-profile"
	profile.Input.ExpectedRevision = 1
	profile.Input.Environment = "production"
	p, err := fiscal.Configure(ctx, a.DB, a.License, a.Actor, a.Device, profile.Input, a.Token)
	if err != nil || p.Revision != 2 {
		t.Fatal("production profile", err)
	}
	in.OperationID = "save-production"
	in.ExpectedRevision = 1
	in.ProfileRevision = 2
	if _, err = r.SaveCustomerCredentials(ctx, a, "profile", in); err != nil {
		t.Fatal("cannot save production draft", err)
	}
	if _, _, err = r.Resolve(a); !errors.Is(err, ErrCheckoutProduction) {
		t.Fatal("production guard weakened", err)
	}
	t.Setenv("FISCAL_MASTER_KEY", "")
	if _, err = r.ReadCustomerCredentials(ctx, a, "profile"); err == nil {
		t.Fatal("missing master key accepted")
	}
}
func TestCustomerCredentialsCannotOptInOrSelectForeignVault(t *testing.T) {
	r, a, _ := customerFixture(t)
	in := CustomerCredentialInput{OperationID: "first", ProfileRevision: 1, Token: []byte("Synthetic123")}
	e := r.entries[a.Actor.TenantID]
	e.Configuration.CustomerManaged = false
	r.entries[a.Actor.TenantID] = e
	if _, err := r.SaveCustomerCredentials(context.Background(), a, "profile", in); err == nil {
		t.Fatal("trusted opt-in bypassed")
	}
	for _, token := range []string{"bad:token", "with space", "with\nnewline", strings.Repeat("x", 513)} {
		in.Token = []byte(token)
		if _, err := r.SaveCustomerCredentials(context.Background(), a, "profile", in); !errors.Is(err, fiscal.ErrInvalid) {
			t.Fatal("invalid token accepted")
		}
	}
}

func TestCustomerCredentialsTenantIsolationAndConcurrentRotation(t *testing.T) {
	ctx := context.Background()
	r, a, path := customerFixture(t)
	other, b, _ := customerFixture(t)
	other.vaultPath = path
	first := CustomerCredentialInput{OperationID: "tenant-first", ProfileRevision: 1, Token: []byte("FirstTenantPrivateToken"), CSC: []byte("FirstTenantCSC")}
	second := CustomerCredentialInput{OperationID: "tenant-second", ProfileRevision: 1, Token: []byte("SecondTenantPrivateToken")}
	if _, err := r.SaveCustomerCredentials(ctx, a, "profile", first); err != nil {
		t.Fatal(err)
	}
	if _, err := other.SaveCustomerCredentials(ctx, b, "profile", second); err != nil {
		t.Fatal(err)
	}
	provider, _, err := other.Resolve(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(provider.(*FocusProvider).token) != string(second.Token) {
		t.Fatal("wrong tenant credential")
	}
	// Two callers race the same revision; only one can replace its ciphertext.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, op := range []string{"rotation-one", "rotation-two"} {
		wg.Add(1)
		go func(op string) {
			defer wg.Done()
			in := first
			in.ExpectedRevision = 1
			in.OperationID = op
			_, err := r.SaveCustomerCredentials(ctx, a, "profile", in)
			results <- err
		}(op)
	}
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, fiscal.ErrConflict) && !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("expected one rotation winner, got %d", winners)
	}
	latest, err := r.ReadCustomerCredentials(ctx, a, "profile")
	if err != nil || latest.Revision != 2 {
		t.Fatal("rotation lost", err)
	}
	isolated, err := other.ReadCustomerCredentials(ctx, b, "profile")
	if err != nil || isolated.Revision != 1 || isolated.OperationID != "tenant-second" {
		t.Fatal("rotation crossed tenants", err)
	}
	text := fmt.Sprintf("%v %#v", first, first)
	if strings.Contains(text, string(first.Token)) || strings.Contains(text, string(first.CSC)) {
		t.Fatal("input formatting disclosed credentials")
	}
	// Swapping encrypted rows must fail authenticated decryption.
	vault, err := OpenFiscalSecretVault(path)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	if _, err = vault.db.Exec(`UPDATE fiscal_secret_vault SET ciphertext=(SELECT ciphertext FROM fiscal_secret_vault WHERE tenant=? AND purpose='customer_provider') WHERE tenant=? AND purpose='customer_provider'`, a.Actor.TenantID, b.Actor.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err = other.ReadCustomerCredentials(ctx, b, "profile"); err == nil {
		t.Fatal("encrypted tenant substitution accepted")
	}
}
func TestCustomerCredentialsTrustedActivationRejectsA3AndProductionURL(t *testing.T) {
	r, a, _ := customerFixture(t)
	base := r.entries[a.Actor.TenantID].Configuration
	for _, change := range []func(*TenantFiscalConfiguration){func(c *TenantFiscalConfiguration) { c.CertificateType = "A3" }, func(c *TenantFiscalConfiguration) { c.FocusBaseURL = "https://api.focusnfe.com.br" }, func(c *TenantFiscalConfiguration) { c.Provider = "own" }} {
		c := base
		change(&c)
		if _, err := NewProviderRegistry([]ProviderRegistration{{Configuration: c}}); err == nil {
			t.Fatal("invalid activation accepted")
		}
	}
}
