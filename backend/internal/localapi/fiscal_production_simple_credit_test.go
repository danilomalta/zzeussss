package localapi

import (
	"bytes"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestHTTPFiscalProductionTaxSimpleCredit(t *testing.T) {
	p := newProductionFixture(t, "55", "1")
	p.input.FinalConsumer = "0"
	p.input.Recipient.IEDestination = "1"
	p.input.Recipient.IE = "110042490114"
	zone, err := time.LoadLocation(p.binding.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(zone)
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	policy := productionTaxPolicy(t, p, func(in *fiscal.TaxPolicyInput) {
		in.Rule.ValidFrom = first.Format("2006-01-02")
		in.Rule.ValidUntil = first.AddDate(0, 1, -1).Format("2006-01-02")
		in.Rule.Goods.ICMS = "101"
		in.Rule.Goods.SimpleCredit = &fiscalcore.SimpleCreditInput{Percent: "2.45", RevenuePeriod: first.AddDate(0, -1, 0).Format("2006-01"), RecipientRegime: "normal", BaseMethod: "approved_net_operation", LegalReference: "Reviewed synthetic credit fixture"}
		in.Case.RecipientStatus = "contributor"
		in.Case.FinalConsumer = "no"
		in.Case.Purpose = "resale"
	})
	quote := productionTaxQuote(t, p, policy)
	if quote.Result.GoodsXML.SimpleCredit == nil {
		t.Fatal("missing credit calculation")
	}
	prep := p.prepare(t, "credit-review", 0, 201)
	p.approve(t, prep, 204)
	out := p.emit(t, prep, "credit-issue", 200)
	if out.State != "authorized" || p.issues.Load() != 1 || !bytes.Contains(out.ProcessedXML, []byte("<ICMSSN101>")) || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil {
		t.Fatal("credit API/signature/transport failed")
	}
	var recovered fiscal.TaxQuote
	taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &recovered)
	if recovered.ResultSHA256 != quote.ResultSHA256 || recovered.Result.GoodsXML.SimpleCredit == nil || *recovered.Result.GoodsXML.SimpleCredit != *quote.Result.GoodsXML.SimpleCredit {
		t.Fatal("persisted credit changed")
	}
	t.Log("Real API/SQLite and signed mTLS to disposable authority; no official credit authorization or DAS assessment")
}
