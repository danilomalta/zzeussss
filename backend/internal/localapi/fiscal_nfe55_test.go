package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
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
	certificateType := "A1"
	if environment == "A3" {
		environment, certificateType = "homologation", "A3"
	}
	var revision int
	if err := f.db.QueryRow(`SELECT max(revision) FROM fiscal_profiles WHERE tenant_id=? AND profile_id='issuer-one'`, f.owner.TenantID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	r, err := fiscalproduction.NewProviderRegistry([]fiscalproduction.ProviderRegistration{{Configuration: fiscalproduction.TenantFiscalConfiguration{DocumentModel: "55", Provider: "external", Environment: environment, CRT: "1", CertificateType: certificateType, Binding: fiscalproduction.ProviderBinding{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, DeviceID: f.owner.DeviceID, ProfileID: "issuer-one", ProfileRevision: revision}}, Provider: p}})
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

// Build a separate sale through the existing policy/quote APIs. Synthetic tax
// decisions belong to this disposable fixture, never to application defaults.
func nfe55MappedSale(t *testing.T, f *stockHTTPFixture, unsupported ...bool) string {
	t.Helper()
	// The generic registration fixture intentionally has no issuer timezone.
	// Tax-priced sales require an explicit owner-approved timezone/profile revision.
	code, raw := request(t, f.app, "GET", "/local/v1/fiscal/profiles/issuer-one", "", f.token)
	var profile fiscal.Profile
	if code != 200 || json.Unmarshal(raw, &profile) != nil {
		t.Fatal("profile fixture", code)
	}
	input := profile.Input
	input.OperationID = "nfe55-timezone"
	input.ExpectedRevision = profile.Revision
	input.Timezone = "America/Sao_Paulo"
	taxRequest(t, f, "POST", "/local/v1/fiscal/profiles", input, 201, nil)
	policy := taxPolicyFixture(f)
	policy.ProfileRevision = profile.Revision + 1
	policy.Rule.Model = "55"
	policy.Rule.BaseRounding = "before_tax"
	policy.Rule.Goods = &fiscalcore.GoodsTaxClassification{Origin: "0", ICMS: "102", PIS: "07", COFINS: "07", IPIScope: "not_applicable", FCPScope: "not_applicable"}
	policy.Rule.Components = nil
	for _, code := range []string{"ICMS", "PIS", "COFINS"} {
		policy.Rule.Components = append(policy.Rule.Components, fiscalcore.TaxComponent{Code: code, Method: "exempt", Rate: "0", MVA: "0", ReductionPercent: "0"})
	}
	if len(unsupported) > 0 && unsupported[0] {
		policy.Rule.Goods.ICMS = "202"
		policy.Case.RecipientStatus = "contributor"
		policy.Case.FinalConsumer = "no"
		policy.Case.Purpose = "resale"
		policy.Rule.Components = append(policy.Rule.Components, fiscalcore.TaxComponent{Code: "ICMSST", Method: "percentage", Rate: "18", MVA: "40", ReductionPercent: "0", AddsToTotal: true, OwnOperationDeduction: &fiscalcore.STOwnDeductionInput{Rate: "18", ReductionPercent: "0", BaseMethod: "approved_net_operation", LegalReference: "Synthetic ST fixture only"}})
	}
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
	quote := taxQuoteFixture(policy)
	quote.Context.Model = "55"
	quote.Context.Date = time.Now().In(time.FixedZone("fixture", -3*3600)).Format("2006-01-02")
	quote.Item.UnitPrice = "15.99"
	var priced fiscal.TaxQuote
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", quote, 201, &priced)
	sale := salesInput(f)
	total, err := strconv.ParseInt(strings.ReplaceAll(priced.Result.Total, ".", ""), 10, 64)
	if err != nil {
		t.Fatal("fixture quote total", err)
	}
	sale.Payments[0].AmountCents = total
	sale.SaleID = "mapped-sale"
	sale.OperationID = "mapped-sale-operation"
	sale.TaxQuoteOperations = []string{quote.OperationID}
	salesRequest(t, f, sale, 201)
	return sale.SaleID
}

func TestHTTPNFe55OwnerContractPrivacyAndStates(t *testing.T) {
	f := fiscalFixture(t, "55")
	p := &nfe55ContractProvider{state: "authorized_homologation"}
	nfe55ContractApp(t, f, p, "homologation")
	var logs bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prior)
	base := "/local/v1/fiscal/nfe55/sales/sale-one/"
	code, b := request(t, f.app, "POST", base+"preview", "{}", f.token)
	if code != 200 || !bytes.Contains(b, []byte(`"mapping_supported":false`)) {
		t.Fatal("preview missing tax diagnostic", code)
	}
	body := `{"nature":"VENDA","purpose":"1","freight_mode":"9","recipient":{"cpf":"12345678909","name":"Private recipient fixture","address":{"street":"Private address","number":"1","district":"Centro","city":"Sao Paulo","uf":"SP"}}}`
	code, b = request(t, f.app, "POST", base+"emit", body, f.token)
	if code != 409 || !bytes.Contains(b, []byte("tax_mapping_unsupported")) || p.calls != 0 {
		t.Fatal("unmapped sale reached provider", code)
	}
	base = "/local/v1/fiscal/nfe55/sales/" + nfe55MappedSale(t, f) + "/"
	nfe55ContractApp(t, f, p, "homologation")
	code, b = request(t, f.app, "POST", base+"preview", "{}", f.token)
	if code != 200 || !bytes.Contains(b, []byte(`"mapping_supported":true`)) {
		t.Fatal("mapped preview", code)
	}
	code, b = request(t, f.app, "POST", base+"emit", body, f.token)
	if code != 200 || bytes.Contains(b, []byte("Private recipient")) || bytes.Contains(b, []byte("Private address")) {
		t.Fatal("request data echoed", code)
	}
	if p.last.Model != "55" || p.last.Environment != 2 || p.last.SaleID != "mapped-sale" || p.last.Purpose != "1" {
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
	// Match fiscal-customer credential tests: create real staff memberships and
	// passwords through the API, instead of demoting the unlinked owner with SQL.
	for _, role := range []string{"manager", "accountant", "cashier"} {
		t.Run(role, func(t *testing.T) {
			code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"nfe55-`+role+`","identity_id":"nfe55-`+role+`","name":"Synthetic","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
			if code != 201 {
				t.Fatal("staff", code)
			}
			code, b := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"nfe55-`+role+`","password":"test-password-strong-1"}`, "")
			if code != 200 {
				t.Fatal("staff login", code)
			}
			var auth struct {
				Token string `json:"token"`
			}
			if json.Unmarshal(b, &auth) != nil || auth.Token == "" {
				t.Fatal("staff session missing")
			}
			for _, action := range []string{"preview", "emit", "query", "cancel", "danfe"} {
				method, data := "POST", "{}"
				if action == "emit" {
					data = body
				}
				if action == "danfe" {
					method, data = "GET", ""
				}
				if code, _ := request(t, f.app, method, base+action, data, auth.Token); code != 403 || p.calls != calls {
					t.Fatal("non-owner reached fiscal flow", role, action, code)
				}
			}
		})
	}
	for _, setting := range []string{"production", "A3"} {
		nfe55ContractApp(t, f, p, setting)
		code, b = request(t, f.app, "POST", base+"emit", body, f.token)
		expected := "production_blocked"
		if setting == "A3" {
			expected = "a3_unsupported"
		}
		if code != 409 || !bytes.Contains(b, []byte(expected)) || p.calls != calls {
			t.Fatal("unsupported configuration reached provider", setting, code)
		}
	}
	nfe55ContractApp(t, f, p, "homologation")
	// Removing the fiscal entitlement cannot be bypassed by an owner session.
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Staff}), 200)
	code, _ = request(t, f.app, "POST", base+"emit", body, f.token)
	if code != 403 || p.calls != calls {
		t.Fatal("missing contract reached provider", code)
	}
	if code, _ = request(t, f.app, "POST", "/local/v1/logout", "{}", f.token); code != 204 {
		t.Fatal("logout", code)
	}
	if code, _ = request(t, f.app, "POST", base+"emit", body, f.token); code != 401 || p.calls != calls {
		t.Fatal("revoked session reached provider", code)
	}
	for _, personal := range []string{"Private recipient fixture", "Private address", "12345678909"} {
		if bytes.Contains(logs.Bytes(), []byte(personal)) {
			t.Fatal("recipient leaked to logs")
		}
	}
}

func TestHTTPNFe55UnmappedTaxGroupNeverCallsProvider(t *testing.T) {
	// Reuse an existing approved ST quote with modBCST: the Focus adapter does
	// not map this field. A valid frozen tax decision must still fail closed.
	f := fiscalFixture(t, "55")
	sale := nfe55MappedSale(t, f, true)
	provider := &nfe55ContractProvider{state: "authorized_homologation"}
	nfe55ContractApp(t, f, provider, "homologation")
	base := "/local/v1/fiscal/nfe55/sales/" + sale + "/"
	code, body := request(t, f.app, "POST", base+"preview", "{}", f.token)
	if code != 200 || !bytes.Contains(body, []byte(`"mapping_supported":false`)) {
		t.Fatal("unsupported group not identified", code)
	}
	code, body = request(t, f.app, "POST", base+"emit", `{"nature":"VENDA","purpose":"1","freight_mode":"9","recipient":{}}`, f.token)
	if code != 409 || !bytes.Contains(body, []byte("tax_mapping_unsupported")) || provider.calls != 0 {
		t.Fatal("unsupported tax group reached provider", code)
	}
}
