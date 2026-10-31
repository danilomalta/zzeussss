package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/fiscalproduction"
)

// API contract fixture only. No Focus/SEFAZ request or acceptance is claimed.
type nfe55ContractProvider struct {
	last  fiscalproduction.DocumentRequest
	state string
	calls int
}

func (p *nfe55ContractProvider) ValidateCheckout(context.Context, fiscalproduction.RequestContext, int) error {
	return nil
}
func (p *nfe55ContractProvider) Emit(_ context.Context, _ fiscalproduction.RequestContext, in fiscalproduction.DocumentRequest) (fiscalproduction.DocumentResult, error) {
	p.calls++
	p.last = in
	return fiscalproduction.DocumentResult{State: p.state}, nil
}
func (p *nfe55ContractProvider) Query(ctx context.Context, a fiscalproduction.RequestContext, in fiscalproduction.DocumentRequest) (fiscalproduction.DocumentResult, error) {
	return p.Emit(ctx, a, in)
}
func (p *nfe55ContractProvider) Reprocess(ctx context.Context, a fiscalproduction.RequestContext, in fiscalproduction.DocumentRequest) (fiscalproduction.DocumentResult, error) {
	return p.Query(ctx, a, in)
}
func (p *nfe55ContractProvider) Cancel(ctx context.Context, a fiscalproduction.RequestContext, in fiscalproduction.DocumentRequest) (fiscalproduction.DocumentResult, error) {
	return p.Query(ctx, a, in)
}
func (p *nfe55ContractProvider) DANFE(context.Context, fiscalproduction.RequestContext, fiscalproduction.DocumentRequest) ([]byte, error) {
	return []byte("%PDF-1.4\ncontract fixture"), nil
}
func nfe55ContractApp(t *testing.T, f *stockHTTPFixture, p *nfe55ContractProvider, environment string) {
	t.Helper()
	r, err := fiscalproduction.NewProviderRegistry([]fiscalproduction.ProviderRegistration{{Configuration: fiscalproduction.TenantFiscalConfiguration{DocumentModel: "55", Provider: "external", Environment: environment, CRT: "1", CertificateType: "A1", Binding: fiscalproduction.ProviderBinding{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, DeviceID: f.owner.DeviceID, ProfileID: "issuer-one", ProfileRevision: 1}}, Provider: p}})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithFiscalProvidersAndGate(f.db, f.device, verifier, nil, nil, nil, r)
	if err != nil {
		t.Fatal(err)
	}
}
func TestHTTPNFe55OwnerContractPrivacyAndStates(t *testing.T) {
	f := fiscalFixture(t, "55")
	p := &nfe55ContractProvider{state: "authorized_homologation"}
	nfe55ContractApp(t, f, p, "homologation")
	base := "/local/v1/fiscal/nfe55/sales/sale-one/"
	code, b := request(t, f.app, "POST", base+"preview", "{}", f.token)
	if code != 200 || !bytes.Contains(b, []byte(`"mapping_supported":false`)) {
		t.Fatal("preview did not expose missing tax mapping", code)
	}
	body := `{"nature":"VENDA","purpose":"1","freight_mode":"9","recipient":{"cpf":"12345678909","name":"Private recipient fixture","address":{"street":"Private address","number":"1","district":"Centro","city":"Sao Paulo","uf":"SP"}}}`
	code, b = request(t, f.app, "POST", base+"emit", body, f.token)
	if code != 200 || bytes.Contains(b, []byte("Private recipient")) || bytes.Contains(b, []byte("Private address")) {
		t.Fatal("request data echoed", code)
	}
	if p.last.Model != "55" || p.last.Environment != 2 || p.last.SaleID != "sale-one" || p.last.Purpose != "1" {
		t.Fatal("wrong forced homologation contract")
	}
	for _, state := range []string{"failed", "cancelled_homologation", "cancellation_failed", "recovery_required"} {
		p.state = state
		code, b = request(t, f.app, "POST", base+"query", "{}", f.token)
		if code != 200 || !bytes.Contains(b, []byte(state)) {
			t.Fatal("status hidden", code)
		}
	}
	calls := p.calls
	code, _ = request(t, f.app, "POST", base+"cancel", `{"operation_id":"cancel-one","reason":"explicit cancellation fixture","confirmed":false}`, f.token)
	if code != 400 || p.calls != calls {
		t.Fatal("cancel without explicit confirmation")
	}
	// A manager/cashier may sell, but cannot use the owner-only fiscal registration flow.
	if _, err := f.db.Exec(`UPDATE memberships SET role='manager' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	managerToken := loginToken(t, f.app, f.owner.OwnerID)
	code, _ = request(t, f.app, "POST", base+"emit", body, managerToken)
	if code != 403 || p.calls != calls {
		t.Fatal("manager reached issuer", code)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='owner' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	f.token = loginToken(t, f.app, f.owner.OwnerID)
	nfe55ContractApp(t, f, p, "production")
	code, b = request(t, f.app, "POST", base+"emit", body, f.token)
	if code != 409 || !bytes.Contains(b, []byte("production_blocked")) || p.calls != calls {
		t.Fatal("production reached provider", code)
	}
	var data map[string]any
	_ = json.Unmarshal(b, &data)
}
