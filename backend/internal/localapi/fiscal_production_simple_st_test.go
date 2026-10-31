package localapi

import (
	"bytes"
	"context"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/fiscal"
)

func productionSimpleSTPolicy(t *testing.T, p *productionFixture, code, mva string) fiscal.TaxPolicyInput {
	t.Helper()
	p.input.FinalConsumer = "0"
	p.input.Recipient.IEDestination = "1"
	p.input.Recipient.IE = "110042490114"
	return productionTaxPolicy(t, p, func(in *fiscal.TaxPolicyInput) {
		in.Rule.Goods.ICMS = code
		in.Case.RecipientStatus = "contributor"
		in.Case.FinalConsumer = "no"
		in.Case.Purpose = "resale"
		in.Rule.Components = append(in.Rule.Components, fiscalcore.TaxComponent{Code: "ICMSST", Method: "percentage", Rate: "18", MVA: mva, ReductionPercent: "0", AddsToTotal: true, OwnOperationDeduction: &fiscalcore.STOwnDeductionInput{Rate: "18", ReductionPercent: "0", BaseMethod: "approved_net_operation", LegalReference: "Approved synthetic ST deduction; not legal applicability"}})
		if code == "201" {
			zone, err := time.LoadLocation(p.binding.Timezone)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().In(zone)
			first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			in.Rule.ValidFrom = first.Format("2006-01-02")
			in.Rule.ValidUntil = first.AddDate(0, 1, -1).Format("2006-01-02")
			in.Rule.Goods.SimpleCredit = &fiscalcore.SimpleCreditInput{Percent: "2.45", RevenuePeriod: first.AddDate(0, -1, 0).Format("2006-01"), RecipientRegime: "normal", BaseMethod: "approved_net_operation", LegalReference: "Approved synthetic credit"}
		}
	})
}
func TestHTTPFiscalProductionTaxSimpleSTQuoteAndSaleGuard(t *testing.T) {
	for _, code := range []string{"201", "202", "203"} {
		t.Run(code, func(t *testing.T) {
			p := newProductionFixture(t, "55", "1")
			policy := productionSimpleSTPolicy(t, p, code, "40")
			quote := productionTaxQuote(t, p, policy)
			if len(quote.Result.Taxes) != 4 || quote.Result.Taxes[3].OwnOperationDeduction == nil || quote.Result.GoodsXML.Totals["vICMS"] != "0.00" {
				t.Fatal("missing deduction or invented own-tax highlight")
			}
			// Do not invent an extra payment or alter the original cash-sale amount.
			p.prepare(t, "unpaid-st-review", 0, 400)
			var count int
			if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_production_allocations`).Scan(&count); err != nil || count != 0 || p.issues.Load() != 0 {
				t.Fatal("unsupported commercial amount consumed number/sent", count, err)
			}
			var recovered fiscal.TaxQuote
			taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &recovered)
			if recovered.ResultSHA256 != quote.ResultSHA256 || *recovered.Result.Taxes[3].OwnOperationDeduction != *quote.Result.Taxes[3].OwnOperationDeduction {
				t.Fatal("deduction changed on read")
			}
			if code == "202" {
				var seq int
				var name, path string
				if err := p.f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
					t.Fatal(err)
				}
				if err := p.f.db.Close(); err != nil {
					t.Fatal(err)
				}
				db, err := localdb.Open(context.Background(), path)
				if err != nil {
					t.Fatal(err)
				}
				p.f.db = db
				defer db.Close()
				p.installApp(t, p.runtime)
				p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
				restored := productionTaxBackupRoundTrip(t, p)
				restoredDB, err := localdb.Open(context.Background(), restored)
				if err != nil {
					t.Fatal(err)
				}
				defer restoredDB.Close()
				p.f.db = restoredDB
				p.installApp(t, p.runtime)
				p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
				taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &recovered)
				if recovered.ResultSHA256 != quote.ResultSHA256 || *recovered.Result.Taxes[3].OwnOperationDeduction != *quote.Result.Taxes[3].OwnOperationDeduction {
					t.Fatal("reopen/restore lost original ST evidence")
				}
			}
		})
	}
}
func TestHTTPFiscalProductionTaxSimpleSTZeroAdditionalAmount(t *testing.T) {
	for _, code := range []string{"201", "202", "203"} {
		t.Run(code, func(t *testing.T) {
			p := newProductionFixture(t, "55", "1")
			policy := productionSimpleSTPolicy(t, p, code, "0")
			quote := productionTaxQuote(t, p, policy)
			if quote.Result.GoodsXML.Totals["vST"] != "0.00" {
				t.Fatal("fixture must have no unpaid additional amount")
			}
			prep := p.prepare(t, "zero-st-review", 0, 201)
			p.approve(t, prep, 204)
			out := p.emit(t, prep, "zero-st-issue", 200)
			if out.State != "authorized" || p.issues.Load() != 1 || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil {
				t.Fatal("real API/signature/fixture mTLS failed")
			}
			tag := "ICMSSN202"
			if code == "201" {
				tag = "ICMSSN201"
			}
			if !bytes.Contains(out.ProcessedXML, []byte("<"+tag+">")) || bytes.Contains(out.ProcessedXML, []byte("<pMVAST>0.0000</pMVAST>")) {
				t.Fatal("incorrect SN-ST group")
			}
			again := p.emit(t, prep, "zero-st-issue", 200)
			if p.issues.Load() != 1 || !bytes.Equal(out.ProcessedXML, again.ProcessedXML) {
				t.Fatal("duplicate send")
			}
			t.Log("Disposable authority, no official homologation; nonzero additional ST remains blocked by the unchanged sale contract")
		})
	}
}
