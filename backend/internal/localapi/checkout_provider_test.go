package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"strings"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/sale"
)

type checkoutTestProvider struct {
	calls  int
	tenant string
	marker string
}

func (p *checkoutTestProvider) ValidateCheckout(_ context.Context, a fiscalproduction.RequestContext, environment int) error {
	if a.Actor.TenantID != p.tenant || environment != 2 {
		return fiscalproduction.ErrUnavailable
	}
	return nil
}
func (p *checkoutTestProvider) Emit(ctx context.Context, a fiscalproduction.RequestContext, in fiscalproduction.DocumentRequest) (fiscalproduction.DocumentResult, error) {
	if err := p.ValidateCheckout(ctx, a, in.Environment); err != nil {
		return fiscalproduction.DocumentResult{}, err
	}
	p.calls++
	state := fiscal.CheckoutStatus{State: "failed", OperationID: p.marker, ErrorCode: "test_provider_no_external_transport"}
	err := fiscal.RecordCheckoutStatus(ctx, a.DB, a.Actor, a.Device, in.SaleID, state)
	return fiscalproduction.DocumentResult{State: state.State, OperationID: state.OperationID, ErrorCode: state.ErrorCode}, err
}
func (p *checkoutTestProvider) Reprocess(ctx context.Context, a fiscalproduction.RequestContext, in fiscalproduction.DocumentRequest) (fiscalproduction.DocumentResult, error) {
	return p.Emit(ctx, a, in)
}
func (p *checkoutTestProvider) Query(context.Context, fiscalproduction.RequestContext, fiscalproduction.DocumentRequest) (fiscalproduction.DocumentResult, error) {
	return fiscalproduction.DocumentResult{}, fiscalproduction.ErrUnavailable
}
func (p *checkoutTestProvider) Cancel(context.Context, fiscalproduction.RequestContext, fiscalproduction.DocumentRequest) (fiscalproduction.DocumentResult, error) {
	return fiscalproduction.DocumentResult{}, fiscalproduction.ErrUnavailable
}

func TestHTTPCheckoutFiscalProviderTenantSelection(t *testing.T) {
	// Real authenticated checkout/database; fake providers are explicitly test-only.
	first := newProductionFixture(t, "65", "1", "America/Sao_Paulo")
	second := newProductionFixture(t, "65", "1", "America/Sao_Paulo")
	fixtures := []*productionFixture{first, second}
	providers := []*checkoutTestProvider{{tenant: first.f.owner.TenantID, marker: "provider-own-test"}, {tenant: second.f.owner.TenantID, marker: "provider-external-test"}}
	entries := make([]fiscalproduction.ProviderRegistration, 0, 2)
	for i, p := range fixtures {
		body := strings.Replace(strings.Replace(fiscalProfileBody, `"issuer-create"`, `"provider-profile"`, 1), `"expected_revision":0`, `"expected_revision":2`, 1)
		body = strings.TrimSuffix(body, "}") + `,"timezone":"America/Sao_Paulo","require_tax_pricing":true}`
		if code, raw := request(t, p.f.app, "POST", "/local/v1/fiscal/profiles", body, p.f.token); code != 201 {
			t.Fatalf("profile %d %s", code, raw)
		}
		policy := productionTaxPolicyInput(p)
		policy.ProfileRevision = 3
		taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
		q := taxQuoteFixture(policy)
		q.OperationID = "provider-quote"
		q.Context.Date = issuerToday(t, p)
		q.Item.Quantity = "1"
		q.Item.UnitPrice = "15.99"
		taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", q, 201, nil)
		name := "own"
		if i == 1 {
			name = "external"
		}
		entries = append(entries, fiscalproduction.ProviderRegistration{Configuration: fiscalproduction.TenantFiscalConfiguration{Binding: fiscalproduction.ProviderBinding{TenantID: p.f.owner.TenantID, StoreID: p.f.owner.StoreID, DeviceID: p.f.device.DeviceID, ProfileID: "issuer-one", ProfileRevision: 3}, Provider: name, Environment: "homologation", CRT: "1"}, Provider: providers[i]})
	}
	registry, err := fiscalproduction.NewProviderRegistry(entries)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range fixtures {
		v, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": p.f.private.Public().(ed25519.PublicKey)})
		if e != nil {
			t.Fatal(e)
		}
		p.f.app, err = NewWithFiscalProvidersAndGate(p.f.db, p.f.device, v, nil, nil, nil, registry)
		if err != nil {
			t.Fatal(err)
		}
		plan := sale.PlanInput{OperationID: "provider-plan", CheckoutID: "provider-sale", CashSessionID: "turn-one", ExpectedTotalCents: 1599, Items: []sale.Item{{ProductID: p.f.product, LocationID: p.f.shelf, QuantityMilli: 1000}}, Parts: []sale.PlanPartInput{{PartID: "provider-cash", Method: "cash", AmountCents: 1599}}, TaxQuoteOperations: []string{"provider-quote"}}
		if code, raw := request(t, p.f.app, "POST", planPath, planJSON(t, plan), p.f.token); code != 201 {
			t.Fatalf("plan %d %s", code, raw)
		}
		invoice := p.input
		invoice.Environment = 1
		payload := map[string]any{"operation_id": "provider-finish", "received_cents": 2000, "fiscal_invoice": invoice}
		path := planPath + "/provider-sale/finalize-cash"
		if code, _ := request(t, p.f.app, "POST", path, planJSON(t, payload), p.f.token); code != 400 {
			t.Fatal("production not blocked", code)
		}
		invoice.Environment = 2
		payload["fiscal_invoice"] = invoice
		code, raw := request(t, p.f.app, "POST", path, planJSON(t, payload), p.f.token)
		var got sale.Plan
		if code != 201 || json.Unmarshal(raw, &got) != nil || got.Fiscal == nil || got.Fiscal.OperationID != providers[i].marker || got.Completion == nil {
			t.Fatalf("wrong tenant provider: %d %s", code, raw)
		}
		if code, raw = request(t, p.f.app, "GET", planPath+"/provider-sale", "", p.f.token); code != 200 || !strings.Contains(string(raw), providers[i].marker) {
			t.Fatalf("status not durable %d %s", code, raw)
		}
	}
	if providers[0].calls != 1 || providers[1].calls != 1 {
		t.Fatal("cross-tenant routing", providers[0].calls, providers[1].calls)
	}
}
