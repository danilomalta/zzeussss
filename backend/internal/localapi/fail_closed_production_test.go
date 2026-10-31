package localapi

import (
	"bytes"
	"encoding/json"
	"testing"
)

// The disposable authority is a counter only: no real fiscal service is used.
// Even an existing production profile, approved journal and live certificate
// cannot replace attaching the trusted runtime to this server process.
func TestHTTPProductionIssueAndCancelRequireExplicitProcessRuntime(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) {
			p := newProductionFixture(t, model, "1")
			prep := p.prepare(t, "fail-closed-review", 0, 201)
			p.approve(t, prep, 204)
			p.installApp(t, nil)
			emit, _ := json.Marshal(map[string]any{"operation_id": "must-not-issue", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256})
			cancel := `{"operation_id":"must-not-cancel","query_operation_id":"previous-query","reason":"explicit test cancellation reason","approval_reference":"owner-test-approval"}`
			for _, tc := range []struct{ action, body string }{
				{"prepare", `{"enabled":true,"environment":"production","fiscal_production":true}`},
				{"approve", `{"enabled":true,"environment":"production"}`},
				{"emit", string(emit)}, {"cancel", cancel}, {"query", `{}`},
			} {
				code, body := request(t, p.f.app, "POST", p.path("/"+tc.action), tc.body, p.f.token)
				if code != 503 || !bytes.Contains(body, []byte("fiscal_production_unavailable")) {
					t.Fatal("unprovisioned action not refused", tc.action, code)
				}
			}
			// No UI flag, query option or header installs runtime/certificate material.
			code, _ := request(t, p.f.app, "POST", p.path("/emit")+"?production=true", string(emit), p.f.token)
			if code != 400 {
				t.Fatal("query override accepted", code)
			}
			code, body := request(t, p.f.app, "POST", "/local/v1/checkout/plans/nonexistent/finalize-cash", `{"operation_id":"blocked-production","received_cents":3198,"fiscal_invoice":{"environment":1}}`, p.f.token)
			if code != 400 || !bytes.Contains(body, []byte("fiscal_checkout_production_disabled")) {
				t.Fatal("checkout production override accepted", code)
			}
			// Reprocess accepts no caller-supplied original invoice or production flag.
			code, _ = request(t, p.f.app, "POST", "/local/v1/checkout/plans/nonexistent/fiscal/reprocess", `{"environment":1,"fiscal_invoice":{"environment":1}}`, p.f.token)
			if code < 400 {
				t.Fatal("reprocess accepted production input", code)
			}
			if p.issues.Load() != 0 || p.cancels.Load() != 0 || p.queries.Load() != 0 {
				t.Fatal("disabled server reached authority")
			}
		})
	}
}

func TestHTTPNFe55ProductionConfigurationCannotEmitOrCancel(t *testing.T) {
	f := fiscalFixture(t, "55")
	provider := &nfe55ContractProvider{state: "authorized_homologation"}
	nfe55ContractApp(t, f, provider, "production")
	base := "/local/v1/fiscal/nfe55/sales/sale-one/"
	for _, action := range []string{"preview", "emit", "query", "cancel"} {
		code, body := request(t, f.app, "POST", base+action, `{"environment":1,"enabled":true}`, f.token)
		// Preview's strict empty-body check also refuses the injected flag.
		if action == "preview" {
			if code != 400 {
				t.Fatal("preview override accepted", code)
			}
			continue
		}
		if code != 409 || !bytes.Contains(body, []byte("production_blocked")) {
			t.Fatal("production provider reached", action, code)
		}
	}
	if provider.calls != 0 {
		t.Fatal("production provider called by customer route")
	}
}
