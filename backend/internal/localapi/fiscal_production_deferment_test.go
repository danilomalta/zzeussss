package localapi

import (
	"bytes"
	"testing"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestHTTPFiscalProductionTaxDeferment(t *testing.T) {
	for _, percent := range []string{"33.33", "100"} {
		t.Run(percent, func(t *testing.T) {
			p := newProductionFixture(t, "55", "3")
			policy := productionTaxPolicy(t, p, func(in *fiscal.TaxPolicyInput) {
				in.Rule.Goods.ICMS = "51"
				in.Rule.Goods.BenefitCode = "SP000001"
				in.Rule.Components[0].Deferment = &fiscalcore.ICMSDefermentInput{Percent: percent, LegalReference: "Reviewed synthetic incidence; not official homologation"}
			})
			quote := productionTaxQuote(t, p, policy)
			if quote.Result.Taxes[0].Deferment == nil {
				t.Fatal("missing persisted trace")
			}
			prep := p.prepare(t, "deferred-review", 0, 201)
			p.approve(t, prep, 204)
			out := p.emit(t, prep, "deferred-issue", 200)
			if out.State != "authorized" || p.issues.Load() != 1 || !bytes.Contains(out.ProcessedXML, []byte("<ICMS51>")) || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil {
				t.Fatal("deferment API/signature/transport failed")
			}
			again := p.emit(t, prep, "deferred-issue", 200)
			if p.issues.Load() != 1 || !bytes.Equal(out.ProcessedXML, again.ProcessedXML) {
				t.Fatal("duplicate issue")
			}
			var recovered fiscal.TaxQuote
			taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &recovered)
			if recovered.ResultSHA256 != quote.ResultSHA256 || recovered.Result.Taxes[0].Deferment == nil || *recovered.Result.Taxes[0].Deferment != *quote.Result.Taxes[0].Deferment {
				t.Fatal("trace lost during persistence")
			}
			t.Log("Real API/SQLite and signed mTLS to disposable authority, not official homologation")
		})
	}
}
