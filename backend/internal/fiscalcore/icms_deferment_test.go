package fiscalcore

import (
	"bytes"
	"testing"
	"time"
)

func deferredGoodsRule() TaxRule {
	r := classifiedGoodsRule("3", "51")
	r.Goods.BenefitCode = "SP000001" // synthetic client-reviewed fixture, not a legal classification
	r.Components[0].Deferment = &ICMSDefermentInput{Percent: "33.33", LegalReference: "https://www.nfe.fazenda.gov.br/portal/ — reviewed fixture"}
	return r
}
func TestGoodsICMSDefermentAmounts(t *testing.T) {
	for _, tc := range []struct{ operation, percent, deferred, due string }{
		{"180.00", "33.33", "59.99", "120.01"}, {"18.00", "100", "18.00", "0.00"},
		{"0.01", "50", "0.01", "0.00"}, {"0.03", "50", "0.02", "0.01"}, {"0.00", "100", "0.00", "0.00"},
	} {
		t.Run(tc.operation+"-"+tc.percent, func(t *testing.T) {
			v, err := CalculateICMSDeferment(tc.operation, ICMSDefermentInput{Percent: tc.percent, LegalReference: "reviewed incidence"})
			if err != nil || v.OperationAmount != tc.operation || v.DeferredAmount != tc.deferred || v.DueAmount != tc.due {
				t.Fatalf("%+v %v", v, err)
			}
		})
	}
}
func TestGoodsICMSDefermentOfficialXML(t *testing.T) {
	cred := testCredential(t)
	for _, percent := range []string{"33.33", "100"} {
		t.Run(percent, func(t *testing.T) {
			r := deferredGoodsRule()
			r.Components[0].Deferment.Percent = percent
			out := classifiedGoodsCalculate(t, r)
			trace := out.Taxes[0].Deferment
			if trace == nil || trace.OperationAmount != "18.00" || out.Total != "100.00" || out.GoodsXML.Totals["vICMS"] != trace.DueAmount {
				t.Fatalf("bad trace %+v", out)
			}
			// Subsequent contributions use the explicitly declared due-ICMS deduction.
			if percent == "100" && out.Taxes[1].Base != "100.00" {
				t.Fatal("dependency used pre-deferment amount")
			}
			in := testInvoice()
			in.Key.Model = "55"
			in.CRT = "3"
			in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic recipient", IEDestination: "9", Address: in.Issuer.Address}
			line := &in.Lines[0]
			line.ProductID = r.ProductID
			line.NCM = r.NCM
			line.CFOP = r.CFOP
			line.Quantity = "2"
			line.UnitPrice = "50"
			line.BenefitCode = r.Goods.BenefitCode
			line.Taxes = out.GoodsXML.Taxes
			in.TaxTotals = out.GoodsXML.Totals
			in.Payments[0].Amount = out.Total
			raw, _, err := BuildInvoice(in)
			if err != nil {
				t.Fatal(err)
			}
			signed, err := cred.SignXML("nfe", raw, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err = ValidateXML("nfe", signed); err != nil {
				t.Fatal(err)
			}
			if err = cred.VerifyXML("nfe", signed, time.Now()); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(raw, []byte("<ICMS51>")) || !bytes.Contains(raw, []byte("<vICMSOp>18.00</vICMSOp>")) {
				t.Fatal("missing deferred XML")
			}
			altered := out
			altered.Taxes = append([]AppliedTax(nil), out.Taxes...)
			forged := *trace
			forged.DeferredAmount = "0.00"
			altered.Taxes[0].Deferment = &forged
			if _, err = MapGoodsTaxes(r, altered); err == nil {
				t.Fatal("forged trace accepted")
			}
		})
	}
}
func TestGoodsICMSDefermentRejectsInvalidAndUnsupported(t *testing.T) {
	for _, kind := range []string{"zero", "above100", "negative", "precision", "missing-reference", "missing-input", "wrong-code", "wrong-component", "simple", "nfce", "interstate", "FCP", "reduced-base", "deduct", "total", "without-goods"} {
		t.Run(kind, func(t *testing.T) {
			r := deferredGoodsRule()
			switch kind {
			case "zero":
				r.Components[0].Deferment.Percent = "0"
			case "above100":
				r.Components[0].Deferment.Percent = "100.0001"
			case "negative":
				r.Components[0].Deferment.Percent = "-1"
			case "precision":
				r.Components[0].Deferment.Percent = "33.33333"
			case "missing-reference":
				r.Components[0].Deferment.LegalReference = ""
			case "missing-input":
				r.Components[0].Deferment = nil
			case "wrong-code":
				r.Goods.ICMS = "00"
			case "wrong-component":
				r.Components[1].Deferment = r.Components[0].Deferment
				r.Components[0].Deferment = nil
			case "simple":
				r.CRT = "1"
			case "nfce":
				r.Model = "65"
			case "interstate":
				r.DestinationUF = "RJ"
				r.CFOP = "6102"
			case "FCP":
				r.Goods.FCPScope = "own"
			case "reduced-base":
				r.Components[0].ReductionPercent = "10"
			case "deduct":
				r.Components[0].DeductApplied = []string{"PIS"}
			case "total":
				r.Components[0].AddsToTotal = true
			case "without-goods":
				r.Goods = nil
			}
			_, err := Calculate(TaxContext{OriginUF: r.OriginUF, DestinationUF: r.DestinationUF, CRT: r.CRT, Model: r.Model, Date: "2026-10-29"}, TaxItem{ProductID: r.ProductID, Quantity: "2", UnitPrice: "50", Discount: "0", Freight: "0", Insurance: "0", Other: "0"}, []TaxRule{r})
			if err == nil {
				t.Fatal("invalid rule accepted")
			}
		})
	}
}
