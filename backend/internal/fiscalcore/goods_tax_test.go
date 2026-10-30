package fiscalcore

import (
	"bytes"
	"testing"
	"time"
)

func classifiedGoodsRule(crt, icms string) TaxRule {
	r, _, _ := taxPolicyExample()
	r.CRT = crt
	r.Model = "55"
	r.BaseRounding = "before_tax"
	r.Goods = &GoodsTaxClassification{Origin: "0", ICMS: icms, PIS: "01", COFINS: "01", IPIScope: "not_applicable", FCPScope: "not_applicable"}
	own := TaxComponent{Code: "ICMS", Method: "percentage", Rate: "18", MVA: "0", ReductionPercent: "0"}
	if crt == "1" || oneOf(icms, "40", "41", "50") {
		own.Method = "exempt"
		own.Rate = "0"
	}
	if oneOf(icms, "20", "70") {
		own.ReductionPercent = "20"
	}
	r.Components = []TaxComponent{own}
	if oneOf(icms, "10", "70") {
		r.Components = append(r.Components, TaxComponent{Code: "ICMSST", Method: "percentage", Rate: "18", MVA: "40", ReductionPercent: "0", DeductApplied: []string{"ICMS"}, AddsToTotal: true})
	}
	r.Components = append(r.Components, TaxComponent{Code: "PIS", Method: "percentage", Rate: "1.65", MVA: "0", ReductionPercent: "0", SubtractApplied: []string{"ICMS"}}, TaxComponent{Code: "COFINS", Method: "percentage", Rate: "7.6", MVA: "0", ReductionPercent: "0", SubtractApplied: []string{"ICMS"}})
	return r
}
func classifiedGoodsCalculate(t *testing.T, r TaxRule) TaxResult {
	t.Helper()
	out, err := Calculate(TaxContext{OriginUF: r.OriginUF, DestinationUF: r.DestinationUF, CRT: r.CRT, Model: r.Model, Date: "2026-10-29"}, TaxItem{ProductID: r.ProductID, Quantity: "2", UnitPrice: "50", Discount: "0", Freight: "0", Insurance: "0", Other: "0"}, []TaxRule{r})
	if err != nil {
		t.Fatal(err)
	}
	if out.GoodsXML == nil || out.OfficialAuthorization {
		t.Fatal("missing mapping or fictitious authorization")
	}
	return out
}
func TestGoodsTaxClassificationsOfficialXSDAndSignature(t *testing.T) {
	cred := testCredential(t)
	cases := []struct{ crt, icms string }{{"3", "00"}, {"2", "00"}, {"3", "10"}, {"3", "20"}, {"3", "40"}, {"3", "41"}, {"3", "50"}, {"3", "70"}, {"1", "102"}, {"1", "103"}, {"1", "300"}, {"1", "400"}}
	for _, v := range cases {
		t.Run(v.crt+"-"+v.icms, func(t *testing.T) {
			r := classifiedGoodsRule(v.crt, v.icms)
			out := classifiedGoodsCalculate(t, r)
			in := testInvoice()
			in.Key.Model = "55"
			in.CRT = r.CRT
			in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic recipient", IEDestination: "9", Address: in.Issuer.Address}
			in.Lines[0].ProductID = r.ProductID
			in.Lines[0].NCM = r.NCM
			in.Lines[0].CFOP = r.CFOP
			in.Lines[0].Quantity = "2"
			in.Lines[0].UnitPrice = "50"
			in.Lines[0].Taxes = out.GoodsXML.Taxes
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
				t.Fatalf("official schema: %v", err)
			}
			if err = cred.VerifyXML("nfe", signed, time.Now()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestGoodsTaxContributionIPIAndFCPGroups(t *testing.T) {
	cred := testCredential(t)
	for _, scenario := range []string{"own-fcp", "st-fcp", "ipi-percent", "ipi-quantity", "ipi-exempt", "pis-quantity", "pis-nontax", "pis-other-percent", "pis-other-quantity", "benefit-and-cest"} {
		t.Run(scenario, func(t *testing.T) {
			r := classifiedGoodsRule("3", "00")
			switch scenario {
			case "own-fcp":
				r.Goods.FCPScope = "own"
				r.Components = append(r.Components, TaxComponent{Code: "FCP", Method: "percentage", Rate: "2", ReductionPercent: "0", MVA: "0"})
			case "st-fcp":
				r = classifiedGoodsRule("3", "10")
				r.Goods.FCPScope = "own_and_substitution"
				r.Components = append(r.Components, TaxComponent{Code: "FCP", Method: "percentage", Rate: "2", ReductionPercent: "0", MVA: "0"}, TaxComponent{Code: "FCPST", Method: "percentage", Rate: "2", ReductionPercent: "0", MVA: "40", AddsToTotal: true})
			case "ipi-percent", "ipi-quantity", "ipi-exempt":
				r.Goods.IPIScope = "classified"
				r.Goods.IPI = "50"
				r.Goods.IPIEnquadramento = "999"
				c := TaxComponent{Code: "IPI", Method: "percentage", Rate: "5", MVA: "0", ReductionPercent: "0", AddsToTotal: true}
				if scenario == "ipi-quantity" {
					c.Method = "per_unit"
				}
				if scenario == "ipi-exempt" {
					r.Goods.IPI = "53"
					c.Method = "exempt"
					c.Rate = "0"
					c.AddsToTotal = false
				}
				r.Components = append(r.Components, c)
			case "pis-quantity", "pis-other-quantity":
				r.Goods.PIS = "03"
				r.Goods.COFINS = "03"
				if scenario == "pis-other-quantity" {
					r.Goods.PIS = "49"
					r.Goods.COFINS = "99"
				}
				for i := 1; i < 3; i++ {
					r.Components[i].Method = "per_unit"
					r.Components[i].SubtractApplied = nil
				}
			case "pis-nontax":
				r.Goods.PIS = "06"
				r.Goods.COFINS = "07"
				for i := 1; i < 3; i++ {
					r.Components[i].Method = "exempt"
					r.Components[i].Rate = "0"
					r.Components[i].SubtractApplied = nil
				}
			case "pis-other-percent":
				r.Goods.PIS = "49"
				r.Goods.COFINS = "99"
			case "benefit-and-cest":
				r = classifiedGoodsRule("3", "20")
				r.Goods.BenefitCode = "SP000001"
			}
			out := classifiedGoodsCalculate(t, r)
			in := testInvoice()
			in.Key.Model = "55"
			in.CRT = r.CRT
			in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic recipient", IEDestination: "9", Address: in.Issuer.Address}
			in.Lines[0].ProductID = r.ProductID
			in.Lines[0].NCM = r.NCM
			in.Lines[0].CFOP = r.CFOP
			in.Lines[0].Quantity = "2"
			in.Lines[0].UnitPrice = "50"
			in.Lines[0].Taxes = out.GoodsXML.Taxes
			in.Lines[0].BenefitCode = r.Goods.BenefitCode
			if scenario == "benefit-and-cest" {
				in.Lines[0].CEST = "1700000"
			}
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
			if scenario == "st-fcp" && (out.Total != "110.00" || out.GoodsXML.Totals["vFCPST"] != "2.80") {
				t.Fatalf("wrong ST/FCP totals %+v", out)
			}
			if scenario == "benefit-and-cest" && (!bytes.Contains(raw, []byte("<cBenef>SP000001</cBenef>")) || !bytes.Contains(raw, []byte("<CEST>1700000</CEST>"))) {
				t.Fatal("classification metadata missing")
			}
		})
	}
}
func TestGoodsTaxRejectsUnknownAndInconsistentClassification(t *testing.T) {
	for _, scenario := range []string{"missing-IPI-scope", "missing-FCP-scope", "missing-PIS", "unknown-component", "wrong-CRT", "reduction-code", "fake-exempt", "incorrect-total-flag", "hidden-FCP", "rate-precision", "fractional-base"} {
		t.Run(scenario, func(t *testing.T) {
			r := classifiedGoodsRule("3", "00")
			q := TaxItem{ProductID: r.ProductID, Quantity: "2", UnitPrice: "50", Discount: "0", Freight: "0", Insurance: "0", Other: "0"}
			switch scenario {
			case "missing-IPI-scope":
				r.Goods.IPIScope = ""
			case "missing-FCP-scope":
				r.Goods.FCPScope = ""
			case "missing-PIS":
				r.Components = append(r.Components[:1], r.Components[2:]...)
			case "unknown-component":
				r.Components = append(r.Components, TaxComponent{Code: "CBS", Method: "percentage", Rate: "0.9", MVA: "0", ReductionPercent: "0"})
			case "wrong-CRT":
				r.CRT = "1"
			case "reduction-code":
				r.Components[0].ReductionPercent = "20"
			case "fake-exempt":
				r.Goods.ICMS = "40"
			case "incorrect-total-flag":
				r.Components[1].AddsToTotal = true
			case "hidden-FCP":
				r.Goods.FCPScope = "own"
			case "rate-precision":
				r.Components[0].Rate = "18.00001"
			case "fractional-base":
				r.Goods.ICMS = "20"
				r.BaseRounding = "tax_final"
				r.Components[0].Rate = "50"
				r.Components[0].ReductionPercent = "50"
				q.Quantity = "1"
				q.UnitPrice = "0.01"
			}
			_, err := Calculate(TaxContext{OriginUF: r.OriginUF, DestinationUF: r.DestinationUF, CRT: r.CRT, Model: r.Model, Date: "2026-10-29"}, q, []TaxRule{r})
			if err == nil {
				t.Fatal("unsupported/inconsistent tax XML accepted")
			}
		})
	}
}
func TestInterstateGoodsRateAllUFAndQualifiedOriginCodes(t *testing.T) {
	checked := 0
	for origin := range ufCodes {
		for destination := range ufCodes {
			for _, productOrigin := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"} {
				out, err := InterstateGoodsRate(origin, destination, productOrigin)
				if origin == destination {
					if err == nil {
						t.Fatal("internal rate guessed")
					}
					continue
				}
				if err != nil || out.Reference != InterstateGoods2026 || len(out.Sources) < 3 {
					t.Fatal("missing reference", err)
				}
				if oneOf(productOrigin, "1", "2", "3", "8") && out.Rate != "4.0000" {
					t.Fatal("import rate changed")
				}
				checked++
			}
		}
	}
	if checked != 27*26*9 {
		t.Fatal(checked)
	}
	for _, v := range []struct{ o, d, p, rate string }{{"SP", "BA", "0", "7.0000"}, {"SP", "ES", "0", "7.0000"}, {"ES", "BA", "0", "12.0000"}, {"BA", "SP", "0", "12.0000"}, {"SP", "RJ", "0", "12.0000"}, {"PR", "RJ", "1", "4.0000"}, {"PR", "BA", "6", "7.0000"}, {"BA", "SP", "4", "12.0000"}} {
		out, err := InterstateGoodsRate(v.o, v.d, v.p)
		if err != nil || out.Rate != v.rate {
			t.Fatal(v, out, err)
		}
	}
	t.Logf("%d nominal interstate selections across all 27 UFs; not internal rates or comprehensive legal classification", checked)
}
func TestInterstateClassifiedPolicyRateAndFCI(t *testing.T) {
	for _, origin := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"} {
		t.Run(origin, func(t *testing.T) {
			r := classifiedGoodsRule("3", "00")
			r.Goods.Origin = origin
			r.Goods.InterstateBasis = InterstateGoods2026
			r.DestinationUF = "BA"
			r.CFOP = "6102"
			if oneOf(origin, "3", "5", "8") {
				r.Goods.FCI = "00000000-0000-0000-0000-000000000001"
			}
			selection, _ := InterstateGoodsRate(r.OriginUF, r.DestinationUF, origin)
			r.Components[0].Rate = selection.Rate
			out := classifiedGoodsCalculate(t, r)
			if out.GoodsXML.InterstateReference == nil || out.GoodsXML.InterstateReference.Rate != selection.Rate {
				t.Fatal("nominal provenance missing")
			}
			r.Components[0].Rate = "18"
			if ValidateGoodsTaxClassification(r) == nil {
				t.Fatal("wrong nominal interstate rate accepted")
			}
		})
	}
	r := classifiedGoodsRule("3", "00")
	r.DestinationUF = "BA"
	r.CFOP = "6102"
	r.Goods.InterstateBasis = InterstateGoods2026
	r.Goods.Origin = "3"
	r.Components[0].Rate = "4"
	if err := ValidateGoodsTaxClassification(r); err == nil {
		t.Fatal("missing FCI or explicit waiver accepted")
	}
	r.Goods.FCIExemptionReference = "Synthetic waiver for test only"
	if err := ValidateGoodsTaxClassification(r); err != nil {
		t.Fatal(err)
	}
	r.ValidUntil = "2027-01-01"
	if err := ValidateGoodsTaxClassification(r); err == nil {
		t.Fatal("unreviewed future reference accepted")
	}
}

func TestGoodsPolicyAcceptsOfficialLegislativeSourcesWithoutHostSpoofing(t *testing.T) {
	r := classifiedGoodsRule("3", "00")
	_, condition, _ := taxPolicyExample()
	source := "https://www2.camara.leg.br/legin/fed/ressen/1989/resolucao-22-19-maio-1989-481183-publicacaooriginal-1-pl.html"
	if err := ValidateTaxPolicy(r, condition, []string{source}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"https://www2.camara.leg.br.evil.example/law", "https://user:secret@www2.camara.leg.br/law", "http://www2.camara.leg.br/law"} {
		if ValidateTaxPolicy(r, condition, []string{bad}) == nil {
			t.Fatal("spoofed legal source accepted")
		}
	}
}
