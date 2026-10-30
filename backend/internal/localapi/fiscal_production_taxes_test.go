package localapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
)

func productionTaxPolicyInput(p *productionFixture) fiscal.TaxPolicyInput {
	in := taxPolicyFixture(p.f)
	in.ProfileRevision = 2
	in.Rule.CRT = p.input.CRT
	in.Rule.Model = p.binding.Model
	in.Rule.NCM = "09012100"
	in.Rule.BaseRounding = "before_tax"
	in.Rule.Goods = &fiscalcore.GoodsTaxClassification{Origin: "0", ICMS: "102", PIS: "07", COFINS: "07", IPIScope: "not_applicable", FCPScope: "not_applicable"}
	in.Rule.Components = []fiscalcore.TaxComponent{}
	for _, code := range []string{"ICMS", "PIS", "COFINS"} {
		in.Rule.Components = append(in.Rule.Components, fiscalcore.TaxComponent{Code: code, Method: "exempt", Rate: "0", MVA: "0", ReductionPercent: "0"})
	}
	if p.input.CRT == "3" {
		in.Rule.Goods.ICMS = "00"
		in.Rule.Goods.PIS = "01"
		in.Rule.Goods.COFINS = "01"
		for i, rate := range []string{"18", "1.65", "7.6"} {
			in.Rule.Components[i].Method = "percentage"
			in.Rule.Components[i].Rate = rate
			if i > 0 {
				in.Rule.Components[i].SubtractApplied = []string{"ICMS"}
			}
		}
	}
	return in
}

func productionTaxPolicy(t *testing.T, p *productionFixture, updates ...func(*fiscal.TaxPolicyInput)) fiscal.TaxPolicyInput {
	t.Helper()
	in := productionTaxPolicyInput(p)
	for _, update := range updates {
		update(&in)
	}
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-policies", in, 201, nil)
	return in
}
func productionTaxQuote(t *testing.T, p *productionFixture, in fiscal.TaxPolicyInput) fiscal.TaxQuote {
	t.Helper()
	quote := taxQuoteFixture(in)
	quote.Context.CRT = in.Rule.CRT
	quote.Context.Model = in.Rule.Model
	quote.Context.OriginUF = in.Rule.OriginUF
	quote.Context.DestinationUF = in.Rule.DestinationUF
	zone, err := time.LoadLocation(p.binding.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	quote.Context.Date = time.Now().In(zone).Format("2006-01-02")
	line := p.input.Lines[0]
	quote.Item = fiscalcore.TaxItem{ProductID: line.ProductID, Quantity: line.Quantity, UnitPrice: line.UnitPrice, Discount: line.Discount, Freight: line.Freight, Insurance: line.Insurance, Other: line.Other}
	var out fiscal.TaxQuote
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", quote, 201, &out)
	p.input.Lines[0].TaxQuoteOperationID = quote.OperationID
	p.input.Lines[0].Taxes = nil
	p.input.Lines[0].NCM = ""
	p.input.Lines[0].CFOP = ""
	p.input.TaxTotals = nil
	return out
}

func TestHTTPFiscalProductionTaxQuotesBuildSignTransmitAndRecover(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		for _, crt := range []string{"1", "3"} {
			t.Run(model+"-CRT"+crt, func(t *testing.T) {
				p := newProductionFixture(t, model, crt)
				policy := productionTaxPolicy(t, p)
				quote := productionTaxQuote(t, p, policy)
				if quote.Result.GoodsXML == nil {
					t.Fatal("missing server mapping")
				}
				prep := p.prepare(t, "quote-backed-review", 0, 201)
				if prep.Invoice.Lines[0].TaxQuoteOperationID != quote.Input.OperationID || prep.Invoice.Lines[0].NCM != policy.Rule.NCM || len(prep.Invoice.Lines[0].Taxes) != 3 {
					t.Fatal("approved tax source not bound")
				}
				// Repeating the exact explicit review is idempotent and conserves its number.
				repeated := p.prepare(t, "quote-backed-review", 0, 201)
				if repeated.ReviewSHA256 != prep.ReviewSHA256 || repeated.Invoice.Key != prep.Invoice.Key {
					t.Fatal("review replay changed invoice")
				}
				p.approve(t, prep, 204)
				out := p.emit(t, prep, "quote-backed-issue", 200)
				if out.State != "authorized" || p.issues.Load() != 1 || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil {
					t.Fatal("real fixture transport or official schema failed")
				}
				again := p.emit(t, prep, "quote-backed-issue", 200)
				if p.issues.Load() != 1 || !bytes.Equal(out.ProcessedXML, again.ProcessedXML) {
					t.Fatal("duplicated transmission")
				}
				if crt == "3" && !bytes.Contains(out.ProcessedXML, []byte("<ICMS00>")) {
					t.Fatal("normal-regime XML absent")
				}
				if crt == "1" && !bytes.Contains(out.ProcessedXML, []byte("<ICMSSN102>")) {
					t.Fatal("Simples XML absent")
				}
				policy.OperationID = "deactivate-after-issue"
				policy.ExpectedRevision = 1
				policy.Active = false
				taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
				code, body := request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token)
				if code != 200 || !bytes.Equal(out.ProcessedXML, body) {
					t.Fatal("policy change hid original XML")
				}
				var recovered fiscal.TaxQuote
				taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &recovered)
				if recovered.ResultSHA256 != quote.ResultSHA256 || recovered.PolicySHA256 != quote.PolicySHA256 {
					t.Fatal("policy change recalculated original")
				}
				t.Log("Real local API/SQLite, official embedded XSD, signature and mTLS; synthetic disposable authority only, no official homologation")
			})
		}
	}
}
func TestHTTPFiscalProductionTaxQuotesRejectTamperWithoutNumberOrPost(t *testing.T) {
	for _, kind := range []string{"price", "tax-nodes", "tax-total", "NCM", "CFOP", "CEST", "benefit", "FCI", "missing-quote"} {
		t.Run(kind, func(t *testing.T) {
			p := newProductionFixture(t, "65", "3")
			policy := productionTaxPolicy(t, p)
			productionTaxQuote(t, p, policy)
			want := 400
			switch kind {
			case "price":
				p.input.Lines[0].UnitPrice = "99.99"
			case "tax-nodes":
				p.input.Lines[0].Taxes = []fiscalcore.TaxNode{{Name: "ICMS", Value: "forged"}}
			case "tax-total":
				p.input.TaxTotals = map[string]string{"vICMS": "0.00"}
			case "NCM":
				p.input.Lines[0].NCM = "00000000"
			case "CFOP":
				p.input.Lines[0].CFOP = "5101"
			case "CEST":
				p.input.Lines[0].CEST = "0000000"
			case "benefit":
				p.input.Lines[0].BenefitCode = "SP000001"
			case "FCI":
				p.input.Lines[0].FCI = "00000000-0000-0000-0000-000000000001"
			case "missing-quote":
				p.input.Lines[0].TaxQuoteOperationID = "absent"
				want = 404
			}
			p.prepare(t, "invalid-review", 0, want)
			var n int
			if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_production_allocations`).Scan(&n); err != nil || n != 0 {
				t.Fatal("invalid quote allocated number", n, err)
			}
			if p.issues.Load() != 0 {
				t.Fatal("invalid quote sent")
			}
		})
	}
}
func TestHTTPFiscalProductionTaxPolicyChangeBlocksApprovalAndSend(t *testing.T) {
	for _, at := range []string{"before-approval", "before-send"} {
		t.Run(at, func(t *testing.T) {
			p := newProductionFixture(t, "65", "3")
			policy := productionTaxPolicy(t, p)
			quote := productionTaxQuote(t, p, policy)
			prep := p.prepare(t, "review-before-policy-change", 0, 201)
			if at == "before-send" {
				p.approve(t, prep, 204)
			}
			policy.OperationID = "policy-withdraw"
			policy.ExpectedRevision = 1
			policy.Active = false
			taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
			if at == "before-approval" {
				p.approve(t, prep, 409)
			}
			p.emit(t, prep, "blocked-stale-policy", 409)
			if p.issues.Load() != 0 {
				t.Fatal("withdrawn policy transmitted")
			}
			var recovered fiscal.TaxQuote
			taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &recovered)
			if recovered.ResultSHA256 != quote.ResultSHA256 {
				t.Fatal("uncertain/original tax data replaced")
			}
		})
	}
}

func TestHTTPFiscalProductionTaxQuoteRestartAndEncryptedRestore(t *testing.T) {
	p := newProductionFixture(t, "65", "3")
	policy := productionTaxPolicy(t, p)
	quote := productionTaxQuote(t, p, policy)
	prep := p.prepare(t, "durable-tax-review", 0, 201)
	p.approve(t, prep, 204)
	// Reopen the real same SQLite and application before transmission.
	var sequence int
	var name, path string
	if err := p.f.db.QueryRow(`PRAGMA database_list`).Scan(&sequence, &name, &path); err != nil {
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
	t.Cleanup(func() { db.Close() })
	p.installApp(t, p.runtime)
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	// The old review's session must not regain send authority after login.
	p.emit(t, prep, "old-review-after-restart", 403)
	taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &quote)
	// Preserve every immutable record through an actual encrypted backup restore.
	restored := productionTaxBackupRoundTrip(t, p)
	restoredDB, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer restoredDB.Close()
	p.f.db = restoredDB
	p.installApp(t, p.runtime)
	old := p.f.token
	code, _ := request(t, p.f.app, "GET", p.path(""), "", old)
	if code != 401 {
		t.Fatal("restored session survived", code)
	}
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	var after fiscal.TaxQuote
	taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &after)
	if after.ResultSHA256 != quote.ResultSHA256 || after.PolicySHA256 != quote.PolicySHA256 {
		t.Fatal("restore changed tax source")
	}
	raw := productionReadBody(t, p)
	if !strings.Contains(string(raw), prep.ReviewSHA256) {
		t.Fatal("restored review missing")
	}
	if p.issues.Load() != 0 {
		t.Fatal("restart/restore caused transmission")
	}
}

// Helpers retain the normal database and backup code; no mock persistence.
func productionReadBody(t *testing.T, p *productionFixture) []byte {
	t.Helper()
	code, body := request(t, p.f.app, "GET", p.path(""), "", p.f.token)
	if code != 200 {
		t.Fatal(code, string(body))
	}
	return body
}

func productionTaxBackupRoundTrip(t *testing.T, p *productionFixture) string {
	t.Helper()
	dir := t.TempDir()
	archive := filepath.Join(dir, "tax-production.tytbak")
	restored := filepath.Join(dir, "restored.sqlite")
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if err := backup.Create(context.Background(), p.f.db, p.f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err := backup.Verify(context.Background(), archive, p.f.device, key); err != nil {
		t.Fatal(err)
	}
	if err := backup.Restore(context.Background(), archive, restored, p.f.device, key); err != nil {
		t.Fatal(err)
	}
	return restored
}

func TestHTTPFiscalProductionTaxDestinationDIFALAndFCP(t *testing.T) {
	for _, withFCP := range []bool{false, true} {
		t.Run(map[bool]string{false: "without-FCP", true: "with-FCP"}[withFCP], func(t *testing.T) {
			p := newProductionFixture(t, "55", "3")
			p.input.Destination = "2"
			p.input.Presence = "9"
			p.input.Recipient.Address.UF = "BA"
			p.input.Recipient.Address.CityCode = "2927408"
			p.input.Recipient.Address.City = "Salvador"
			p.input.Recipient.Address.PostalCode = "40000000"
			policy := productionTaxPolicy(t, p, func(in *fiscal.TaxPolicyInput) {
				in.Rule.DestinationUF = "BA"
				in.Rule.CFOP = "6102"
				in.Case.DestinationMunicipality = "2927408"
				in.Rule.Components[0].Rate = "7"
				in.Rule.Goods.InterstateBasis = fiscalcore.InterstateGoods2026
				in.Rule.Goods.Destination = &fiscalcore.DestinationTaxClassification{Reference: fiscalcore.DestinationTax2026, BaseMethod: "single_inclusive_operation_base", FCPScope: "not_applicable"}
				in.Rule.Components = append(in.Rule.Components, fiscalcore.TaxComponent{Code: "DIFAL", Method: "percentage", Rate: "18", MVA: "0", ReductionPercent: "0", DeductApplied: []string{"ICMS"}})
				if withFCP {
					in.Rule.Goods.Destination.FCPScope = "taxed"
					in.Rule.Components = append(in.Rule.Components, fiscalcore.TaxComponent{Code: "FCPUFDEST", Method: "percentage", Rate: "2", MVA: "0", ReductionPercent: "0"})
				}
			})
			quote := productionTaxQuote(t, p, policy)
			if quote.Result.GoodsXML.Totals["vICMSUFDest"] != "3.52" {
				t.Fatal("wrong DIFAL", quote.Result.GoodsXML.Totals)
			}
			prep := p.prepare(t, "destination-tax-review", 0, 201)
			p.approve(t, prep, 204)
			out := p.emit(t, prep, "destination-tax-issue", 200)
			if out.State != "authorized" || p.issues.Load() != 1 || !bytes.Contains(out.ProcessedXML, []byte("<ICMSUFDest>")) || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil {
				t.Fatal("destination groups not preserved by transport")
			}
			if withFCP && quote.Result.GoodsXML.Totals["vFCPUFDest"] != "0.64" {
				t.Fatal("FCP not separated")
			}
			// Recording the fiscal values is not payment of a GNRE or bank transaction.
			var after fiscal.TaxQuote
			taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &after)
			if after.ResultSHA256 != quote.ResultSHA256 {
				t.Fatal("emission modified calculation")
			}
		})
	}
}
