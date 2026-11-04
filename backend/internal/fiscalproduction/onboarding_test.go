package fiscalproduction

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/testsupport/fiscalcert"
)

func TestOnboardingOfflineScopeRevisionAndRevocation(t *testing.T) {
	c, err := fiscalcert.New("12ABC34501DE35", "synthetic.gov.br")
	if err != nil {
		t.Fatal(err)
	}
	crl, err := c.CRL(false, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	b := bindingForTest(c)
	runtime, err := Provision(b, c.Credential, func() ([][]byte, error) { return [][]byte{crl}, nil }, nil, "", NetworkTrust{})
	if err != nil {
		t.Fatal(err)
	}
	a := RequestContext{Actor: identity.Scope{TenantID: b.TenantID, StoreID: b.StoreID}, Device: identity.DeviceContext{TenantID: b.TenantID, StoreID: b.StoreID, DeviceID: b.DeviceID}}
	p := fiscal.Profile{Revision: 1, Input: fiscal.ProfileInput{ProfileID: b.ProfileID, CNPJ: b.Issuer, State: b.UF, Model: b.Model, Series: b.Series, Environment: "production", CRT: "1"}}
	valid := DiagnoseOnboarding(nil, runtime, a, p)
	if !valid.ConfigurationValid || !valid.ProductionBlocked || valid.OfficialAuthorization || valid.TransmissionPerformed {
		t.Fatal("offline check changed authority")
	}
	for _, change := range []func(*RequestContext, *fiscal.Profile){
		func(a *RequestContext, p *fiscal.Profile) { a.Actor.TenantID = "other" },
		func(a *RequestContext, p *fiscal.Profile) { a.Device.StoreID = "other" },
		func(a *RequestContext, p *fiscal.Profile) { p.Revision++ },
		func(a *RequestContext, p *fiscal.Profile) { p.Input.Environment = "homologation" },
		func(a *RequestContext, p *fiscal.Profile) { p.Input.CNPJ = "other" },
	} {
		aa, pp := a, p
		change(&aa, &pp)
		if DiagnoseOnboarding(nil, runtime, aa, pp).ConfigurationValid {
			t.Fatal("mismatch accepted")
		}
	}
	registry, err := NewProviderRegistry([]ProviderRegistration{{Configuration: TenantFiscalConfiguration{Binding: bindingOf(runtime), Provider: "own", Environment: "production", CRT: "1"}, Provider: NewOwnProvider(runtime)}})
	if err != nil {
		t.Fatal(err)
	}
	if !DiagnoseOnboarding(registry, nil, a, p).ConfigurationValid {
		t.Fatal("installed provider rejected")
	}
	pp := p
	pp.Input.CRT = "3"
	if DiagnoseOnboarding(registry, nil, a, pp).ConfigurationValid {
		t.Fatal("CRT mismatch accepted")
	}
	aa := a
	aa.Device.DeviceID = "other"
	if DiagnoseOnboarding(registry, runtime, aa, p).ConfigurationValid {
		t.Fatal("registry fell back across device")
	}
	crl, err = c.CRL(true, time.Now().Add(-time.Minute), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if DiagnoseOnboarding(nil, runtime, a, p).ConfigurationValid {
		t.Fatal("revoked credential accepted")
	}
}

// These are offline configuration checks, not evidence of Focus acceptance.
type noOnboardingNetwork struct{}

func (noOnboardingNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	panic("diagnostic attempted HTTP")
}

func TestOnboardingFocusCustomerIsolationAndNoSecrets(t *testing.T) {
	entries := []ProviderRegistration{}
	for _, tenant := range []string{"client-one", "client-two"} {
		c := TenantFiscalConfiguration{Binding: ProviderBinding{TenantID: tenant, StoreID: "store", DeviceID: "device", ProfileID: "profile-" + tenant, ProfileRevision: 3}, Provider: "external", Environment: "homologation", CRT: "1", CertificateType: "A1"}
		provider, err := newFocusProvider(c, []byte("PrivateToken"+strings.ReplaceAll(tenant, "-", "")), []byte("PrivateCSC"))
		if err != nil {
			t.Fatal(err)
		}
		provider.client.Transport = noOnboardingNetwork{}
		entries = append(entries, ProviderRegistration{Configuration: c, Provider: provider})
	}
	registry, err := NewProviderRegistry(entries)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		c := entry.Configuration
		a := RequestContext{Actor: identity.Scope{TenantID: c.Binding.TenantID, StoreID: "store"}, Device: identity.DeviceContext{TenantID: c.Binding.TenantID, StoreID: "store", DeviceID: "device"}}
		p := fiscal.Profile{Revision: 3, Input: fiscal.ProfileInput{ProfileID: c.Binding.ProfileID, CNPJ: "11222333000181", Environment: "homologation", CRT: "1", Model: "65", State: "SP", Series: 1}}
		out := DiagnoseOnboarding(registry, nil, a, p)
		if !out.ConfigurationValid || !out.ProductionBlocked || out.TransmissionPerformed || out.OfficialAuthorization || len(out.Blockers) != 2 {
			t.Fatal("incorrect offline readiness")
		}
		d := out.ProviderConfiguration
		if d == nil || d.Provider != "external" || !d.TokenPresent || !d.CSCPresent || d.CertificateType != "A1" || d.Environment != "homologation" {
			t.Fatal("missing safe credential presence")
		}
		raw, _ := json.Marshal(out)
		for _, secret := range []string{"PrivateToken", "PrivateCSC", "token_reference", c.Binding.TenantID} {
			if strings.Contains(string(raw), secret) {
				t.Fatal("diagnostic leaked private data")
			}
		}
		for i, change := range []func(*RequestContext, *fiscal.Profile){
			func(a *RequestContext, p *fiscal.Profile) { a.Actor.TenantID = "foreign" },
			func(a *RequestContext, p *fiscal.Profile) { a.Device.TenantID = "foreign" },
			func(a *RequestContext, p *fiscal.Profile) { a.Actor.StoreID = "foreign" },
			func(a *RequestContext, p *fiscal.Profile) { a.Device.DeviceID = "foreign" },
			func(a *RequestContext, p *fiscal.Profile) { p.Input.ProfileID = "foreign" },
			func(a *RequestContext, p *fiscal.Profile) { p.Revision++ },
			func(a *RequestContext, p *fiscal.Profile) { p.Input.Environment = "production" },
			func(a *RequestContext, p *fiscal.Profile) { p.Input.CRT = "3" },
			func(a *RequestContext, p *fiscal.Profile) { p.Input.Model = "55" },
		} {
			t.Run(fmt.Sprintf("%s-mismatch-%d", c.Binding.TenantID, i), func(t *testing.T) {
				aa, pp := a, p
				change(&aa, &pp)
				got := DiagnoseOnboarding(registry, nil, aa, pp)
				if got.ConfigurationValid || got.ProviderConfiguration != nil || !got.ProductionBlocked || got.TransmissionPerformed || got.OfficialAuthorization {
					t.Fatal("foreign or stale configuration disclosed")
				}
			})
		}
		// No local CSC is also meaningful: Focus manages its own credentials;
		// absence here must be reported, never interpreted as a remote check.
		entry.Provider.(*FocusProvider).csc = nil
		if got := DiagnoseOnboarding(registry, nil, a, p); !got.ConfigurationValid || got.ProviderConfiguration.CSCPresent {
			t.Fatal("CSC presence invented")
		}
	}
}
