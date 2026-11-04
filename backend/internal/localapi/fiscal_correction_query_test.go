package localapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"titansystem-backend/internal/fiscalproduction"
)

// API/permission contract double, not Focus authorization evidence.
func (p *nfe55ContractProvider) QueryCorrection(_ context.Context, _ fiscalproduction.RequestContext, in fiscalproduction.DocumentRequest) (fiscalproduction.CorrectionResult, error) {
	p.calls++
	p.last = in
	return fiscalproduction.CorrectionResult{State: p.state}, nil
}
func TestHTTPNFe55CorrectionQueryIDOnlyAndOwnerGate(t *testing.T) {
	f := fiscalFixture(t, "55")
	p := &nfe55ContractProvider{state: "correction_reconciliation_required"}
	nfe55ContractApp(t, f, p, "homologation")
	path := "/local/v1/fiscal/nfe55/sales/sale-one/correction/query"
	for _, body := range []string{`{"correction":"private text"}`, `{"sequence":1}`, `{"tenant_id":"other"}`} {
		code, _ := request(t, f.app, "POST", path, body, f.token)
		if code != 400 || p.calls != 0 {
			t.Fatal("client overrides accepted", code)
		}
	}
	for _, state := range []string{"correction_not_requested", "correction_reconciliation_required", "correction_authorized_homologation", "correction_failed"} {
		p.state = state
		code, body := request(t, f.app, "POST", path, `{}`, f.token)
		if code != 200 || !strings.Contains(string(body), state) || p.last.SaleID != "sale-one" || p.last.Model != "55" || p.last.Environment != 2 {
			t.Fatal("ID-only contract", code)
		}
	}
	calls := p.calls
	for _, role := range []string{"manager", "accountant", "cashier"} {
		t.Run(role, func(t *testing.T) {
			code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"cce-query-`+role+`","identity_id":"cce-query-`+role+`","name":"Synthetic","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
			if code != 201 {
				t.Fatal("staff", code)
			}
			code, body := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"cce-query-`+role+`","password":"test-password-strong-1"}`, "")
			var auth struct {
				Token string `json:"token"`
			}
			if code != 200 || json.Unmarshal(body, &auth) != nil {
				t.Fatal("login", code)
			}
			code, _ = request(t, f.app, "POST", path, `{}`, auth.Token)
			if code != 403 || p.calls != calls {
				t.Fatal("non-owner query", code)
			}
		})
	}
	for _, env := range []string{"production", "A3"} {
		nfe55ContractApp(t, f, p, env)
		code, _ := request(t, f.app, "POST", path, `{}`, f.token)
		if code != 409 || p.calls != calls {
			t.Fatal("gate bypass", env, code)
		}
	}
}
