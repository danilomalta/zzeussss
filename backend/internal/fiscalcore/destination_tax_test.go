package fiscalcore

import (
	"bytes"
	"testing"
	"time"
)

func destinationGoodsRule(fcp bool) TaxRule {
	r := classifiedGoodsRule("3", "00")
	r.DestinationUF = "BA"
	r.CFOP = "6102"
	r.Components[0].Rate = "7"
	r.Goods.InterstateBasis = InterstateGoods2026
	r.Goods.Destination = &DestinationTaxClassification{Reference: DestinationTax2026, BaseMethod: "single_inclusive_operation_base", FCPScope: "not_applicable"}
	r.Components = append(r.Components, TaxComponent{Code: "DIFAL", Method: "percentage", Rate: "18", ReductionPercent: "0", MVA: "0", DeductApplied: []string{"ICMS"}})
	if fcp {
		r.Goods.Destination.FCPScope = "taxed"
		r.Components = append(r.Components, TaxComponent{Code: "FCPUFDEST", Method: "percentage", Rate: "2", ReductionPercent: "0", MVA: "0"})
	}
	return r
}
func TestDestinationTaxSingleBaseXMLTotalsAndSignature(t *testing.T) {
	cred := testCredential(t)
	for _, fcp := range []bool{false, true} {
		t.Run(map[bool]string{false: "without-FCP", true: "with-FCP"}[fcp], func(t *testing.T) {
			r := destinationGoodsRule(fcp)
			out := classifiedGoodsCalculate(t, r)
			if out.Total != "100.00" || out.GoodsXML.Totals["vICMSUFDest"] != "11.00" || out.GoodsXML.Totals["vICMSUFRemet"] != "0.00" {
				t.Fatalf("bad destination arithmetic %+v", out)
			}
			expectedFCP := "0.00"
			if fcp {
				expectedFCP = "2.00"
			}
			if out.GoodsXML.Totals["vFCPUFDest"] != expectedFCP {
				t.Fatal("FCP mixed with DIFAL")
			}
			in := testInvoice()
			in.Key.Model = "55"
			in.CRT = "3"
			in.Destination = "2"
			address := in.Issuer.Address
			address.UF = "BA"
			address.CityCode = "2927408"
			address.City = "Salvador"
			in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic noncontributor", IEDestination: "9", Address: address}
			in.Lines[0].ProductID = r.ProductID
			in.Lines[0].NCM = r.NCM
			in.Lines[0].CFOP = r.CFOP
			in.Lines[0].Quantity = "2"
			in.Lines[0].UnitPrice = "50"
			in.Lines[0].Taxes = out.GoodsXML.Taxes
			in.TaxTotals = out.GoodsXML.Totals
			in.Payments[0].Amount = "100.00"
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
			if !bytes.Contains(raw, []byte("<pICMSInterPart>100.0000</pICMSInterPart>")) {
				t.Fatal("destination share missing")
			}
			in.TaxTotals["vICMSUFDest"] = "0.00"
			if _, _, err = BuildInvoice(in); err == nil {
				t.Fatal("wrong destination total accepted")
			}
		})
	}
}
func TestDestinationTaxRejectsUnrepresentableOrIncompatiblePolicy(t *testing.T) {
	for _, kind := range []string{"dual-base", "wrong-deduction", "wrong-ICMS-rate", "hidden-FCP", "excess-FCP", "ST", "benefit", "IPI", "wrong-recipient", "not-final", "future"} {
		t.Run(kind, func(t *testing.T) {
			r := destinationGoodsRule(true)
			_, c, sources := taxPolicyExample()
			c.DestinationMunicipality = "2927408"
			switch kind {
			case "dual-base":
				r.Goods.Destination.BaseMethod = "dual_base"
			case "wrong-deduction":
				r.Components[len(r.Components)-2].DeductApplied = nil
			case "wrong-ICMS-rate":
				r.Components[0].Rate = "18"
			case "hidden-FCP":
				r.Goods.Destination.FCPScope = "not_applicable"
			case "excess-FCP":
				r.Components[len(r.Components)-1].Rate = "3"
			case "ST":
				r.Goods.ICMS = "10"
			case "benefit":
				r.Goods.BenefitCode = "SP000001"
			case "IPI":
				r.Goods.IPIScope = "classified"
			case "wrong-recipient":
				c.RecipientStatus = "contributor"
			case "not-final":
				c.FinalConsumer = "no"
			case "future":
				r.ValidUntil = "2027-01-01"
			}
			if ValidateTaxPolicy(r, c, sources) == nil {
				t.Fatal("unsupported destination policy approved")
			}
		})
	}
}
