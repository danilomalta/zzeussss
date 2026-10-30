package fiscalcore

import (
	"bytes"
	"testing"
	"time"
)

func simpleSTRule(code string, fcp bool) TaxRule {
	r := classifiedGoodsRule("1", code)
	r.Components = append(r.Components, TaxComponent{Code: "ICMSST", Method: "percentage", Rate: "18", MVA: "40", ReductionPercent: "0", AddsToTotal: true, OwnOperationDeduction: &STOwnDeductionInput{Rate: "18", ReductionPercent: "0", BaseMethod: "approved_net_operation", LegalReference: "Approved synthetic own-operation deduction"}})
	if code == "201" {
		r.ValidFrom = "2026-10-01"
		r.ValidUntil = "2026-10-31"
		r.Goods.SimpleCredit = &SimpleCreditInput{Percent: "2.45", RevenuePeriod: "2026-09", RecipientRegime: "normal", BaseMethod: "approved_net_operation", LegalReference: "Approved synthetic credit"}
	}
	if fcp {
		r.Goods.FCPScope = "substitution"
		r.Components = append(r.Components, TaxComponent{Code: "FCPST", Method: "percentage", Rate: "2", MVA: "40", ReductionPercent: "0", AddsToTotal: true})
	}
	return r
}
func TestGoodsSimpleSTOfficialXML(t *testing.T) {
	cred := testCredential(t)
	for _, code := range []string{"201", "202", "203"} {
		for _, fcp := range []bool{false, true} {
			name := code
			if fcp {
				name += "-FCP"
			}
			t.Run(name, func(t *testing.T) {
				r := simpleSTRule(code, fcp)
				out := classifiedGoodsCalculate(t, r)
				st := out.Taxes[3]
				wantTotal := "107.20"
				if fcp {
					wantTotal = "110.00"
				}
				if out.Total != wantTotal || st.Base != "140.00" || st.Amount != "7.20" || st.OwnOperationDeduction == nil || st.OwnOperationDeduction.Amount != "18.00" || out.GoodsXML.Totals["vICMS"] != "0.00" {
					t.Fatalf("wrong ST math %+v", out)
				}
				if code == "201" && (out.GoodsXML.SimpleCredit == nil || out.GoodsXML.SimpleCredit.Amount != "2.45" || st.OwnOperationDeduction.Amount == out.GoodsXML.SimpleCredit.Amount) {
					t.Fatal("credit confused with ST deduction")
				}
				if code != "201" && out.GoodsXML.SimpleCredit != nil {
					t.Fatal("invented credit")
				}
				in := testInvoice()
				in.Key.Model = "55"
				in.CRT = "1"
				in.FinalConsumer = "0"
				in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic recipient", IEDestination: "1", IE: "110042490114", Address: in.Issuer.Address}
				line := &in.Lines[0]
				line.ProductID = r.ProductID
				line.NCM = r.NCM
				line.CFOP = r.CFOP
				line.Quantity = "2"
				line.UnitPrice = "50"
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
				tag := "ICMSSN202"
				if code == "201" {
					tag = "ICMSSN201"
				}
				if !bytes.Contains(raw, []byte("<"+tag+">")) || bytes.Contains(raw, []byte("<pRedBCST>0.0000</pRedBCST>")) {
					t.Fatal("wrong group/optional zero fields")
				}
				forged := out
				forged.Taxes = append([]AppliedTax(nil), out.Taxes...)
				trace := *st.OwnOperationDeduction
				trace.Amount = "2.45"
				forged.Taxes[3].OwnOperationDeduction = &trace
				if _, err = MapGoodsTaxes(r, forged); err == nil {
					t.Fatal("forged deduction accepted")
				}
			})
		}
	}
}
func TestGoodsSimpleSTDeductionsAndBounds(t *testing.T) {
	for _, tc := range []struct{ net, rate, reduction, base, amount string }{{"100.00", "18", "0", "100.00", "18.00"}, {"100.00", "18", "20", "80.00", "14.40"}, {"0.03", "50", "0", "0.03", "0.02"}, {"100.00", "0", "0", "100.00", "0.00"}} {
		got, err := CalculateSTOwnDeduction(tc.net, STOwnDeductionInput{Rate: tc.rate, ReductionPercent: tc.reduction, BaseMethod: "approved_net_operation", LegalReference: "Reviewed calculation"})
		if err != nil || got.Base != tc.base || got.Amount != tc.amount {
			t.Fatalf("%+v %v", got, err)
		}
	}
	for _, kind := range []string{"missing-deduction", "wrong-component", "double-deduction", "wrong-CRT", "NFCe", "missing-reference", "over-rate", "over-reduction", "precision", "negative", "unknown-base", "deduction-exceeds-ST", "credit-on202", "missing-credit201", "own-FCP"} {
		t.Run(kind, func(t *testing.T) {
			r := simpleSTRule("202", false)
			switch kind {
			case "missing-deduction":
				r.Components[3].OwnOperationDeduction = nil
			case "wrong-component":
				r.Components[0].OwnOperationDeduction = r.Components[3].OwnOperationDeduction
				r.Components[3].OwnOperationDeduction = nil
			case "double-deduction":
				r.Components[3].DeductApplied = []string{"ICMS"}
			case "wrong-CRT":
				r.CRT = "3"
			case "NFCe":
				r.Model = "65"
			case "missing-reference":
				r.Components[3].OwnOperationDeduction.LegalReference = ""
			case "over-rate":
				r.Components[3].OwnOperationDeduction.Rate = "100.01"
			case "over-reduction":
				r.Components[3].OwnOperationDeduction.ReductionPercent = "100.01"
			case "precision":
				r.Components[3].OwnOperationDeduction.Rate = "18.00001"
			case "negative":
				r.Components[3].OwnOperationDeduction.Rate = "-1"
			case "unknown-base":
				r.Components[3].OwnOperationDeduction.BaseMethod = "guess"
			case "deduction-exceeds-ST":
				r.Components[3].OwnOperationDeduction.Rate = "30"
			case "credit-on202":
				r.Goods.SimpleCredit = simpleSTRule("201", false).Goods.SimpleCredit
			case "missing-credit201":
				r.Goods.ICMS = "201"
			case "own-FCP":
				r.Goods.FCPScope = "own_and_substitution"
				r.Components = append(r.Components, TaxComponent{Code: "FCP", Method: "percentage", Rate: "2", MVA: "0", ReductionPercent: "0"})
			}
			_, err := Calculate(TaxContext{OriginUF: r.OriginUF, DestinationUF: r.DestinationUF, CRT: r.CRT, Model: r.Model, Date: "2026-10-29"}, TaxItem{ProductID: r.ProductID, Quantity: "2", UnitPrice: "50", Discount: "0", Freight: "0", Insurance: "0", Other: "0"}, []TaxRule{r})
			if err == nil {
				t.Fatal("unsupported/invalid ST accepted")
			}
		})
	}
}

func TestGoodsSimpleSTApprovedCase(t *testing.T) {
	for _, code := range []string{"201", "202", "203"} {
		t.Run(code, func(t *testing.T) {
			r := simpleSTRule(code, false)
			_, c, sources := taxPolicyExample()
			c.RecipientStatus = "contributor"
			c.FinalConsumer = "no"
			c.Purpose = "resale"
			if err := ValidateTaxPolicy(r, c, sources); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []string{"final", "noncontributor", "industrialization"} {
				t.Run(kind, func(t *testing.T) {
					bad := c
					switch kind {
					case "final":
						bad.FinalConsumer = "yes"
					case "noncontributor":
						bad.RecipientStatus = "noncontributor"
					case "industrialization":
						bad.Purpose = "industrialization"
					}
					if ValidateTaxPolicy(r, bad, sources) == nil {
						t.Fatal("out-of-scope ST case accepted")
					}
				})
			}
		})
	}
}
