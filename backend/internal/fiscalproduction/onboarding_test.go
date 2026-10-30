package fiscalproduction

import (
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
