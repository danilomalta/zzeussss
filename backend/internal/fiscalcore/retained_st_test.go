package fiscalcore

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func retainedRule(crt, code string, fcp bool) TaxRule {
	r := classifiedGoodsRule(crt, code)
	r.CFOP = "5405"
	r.Components[0] = TaxComponent{Code: "ICMS", Method: "exempt", Rate: "0", MVA: "0", ReductionPercent: "0"}
	if fcp {
		r.Goods.FCPScope = "retained"
	}
	return r
}
func retainedItem(t *testing.T, fcp bool) TaxItem {
	t.Helper()
	key, err := AccessKey(KeyInput{UF: "SP", IssuedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), CNPJ: "11222333000181", Model: "55", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	s := &RetainedSTInput{SourceKey: key, SourceItem: 1, ApprovalReference: "Owner reviewed purchase line allocation", Quantity: "2", Unit: "UN", Base: "140.00", SupportedRate: "20", SubstituteAmount: "18.00", RetainedAmount: "7.20"}
	if fcp {
		s.FCP = &RetainedFCPInput{Base: "140.00", Rate: "2", Amount: "2.80"}
	}
	return TaxItem{ProductID: "item", Quantity: "2", UnitPrice: "50", Discount: "0", Freight: "0", Insurance: "0", Other: "0", RetainedST: s}
}
func retainedCalculate(t *testing.T, r TaxRule, item TaxItem) (TaxResult, error) {
	t.Helper()
	item.ProductID = r.ProductID
	return Calculate(TaxContext{OriginUF: r.OriginUF, DestinationUF: r.DestinationUF, CRT: r.CRT, Model: r.Model, Date: "2026-10-29"}, item, []TaxRule{r})
}
func TestRetainedSTOfficialXMLDoesNotChargeAgain(t *testing.T) {
	cred := testCredential(t)
	for _, tc := range []struct{ crt, code string }{{"3", "60"}, {"2", "60"}, {"1", "500"}} {
		for _, fcp := range []bool{false, true} {
			name := tc.crt + "-" + tc.code
			if fcp {
				name += "-FCP"
			}
			t.Run(name, func(t *testing.T) {
				r := retainedRule(tc.crt, tc.code, fcp)
				item := retainedItem(t, fcp)
				out, err := retainedCalculate(t, r, item)
				if err != nil {
					t.Fatal(err)
				}
				if out.Total != "100.00" || out.RetainedST == nil || out.GoodsXML.Totals["vST"] != "0.00" || out.GoodsXML.Totals["vICMS"] != "0.00" || out.GoodsXML.Totals["vFCPST"] != "0.00" || out.OfficialAuthorization {
					t.Fatalf("retention charged again %+v", out)
				}
				wantFCP := "0.00"
				if fcp {
					wantFCP = "2.80"
				}
				if out.GoodsXML.Totals["vFCPSTRet"] != wantFCP {
					t.Fatal("retained FCP omitted")
				}
				in := testInvoice()
				in.Key.Model = "55"
				in.CRT = tc.crt
				in.FinalConsumer = "0"
				in.Recipient = &Party{CNPJ: "11222333000181", Name: "Disposable reseller", IEDestination: "1", IE: "110042490114", Address: in.Issuer.Address}
				line := &in.Lines[0]
				line.ProductID = r.ProductID
				line.NCM = r.NCM
				line.CFOP = r.CFOP
				line.CEST = "0100100"
				line.Quantity = item.Quantity
				line.UnitPrice = item.UnitPrice
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
				tag := "ICMS60"
				if tc.code == "500" {
					tag = "ICMSSN500"
				}
				if !bytes.Contains(raw, []byte("<"+tag+">")) || !bytes.Contains(raw, []byte("<vICMSSTRet>7.20</vICMSSTRet>")) || !bytes.Contains(raw, []byte("<vNF>100.00</vNF>")) {
					t.Fatal("wrong XML")
				}
				if !fcp && bytes.Contains(raw, []byte("<pFCPSTRet>")) {
					t.Fatal("invented FCP")
				}
			})
		}
	}
}
func TestRetainedSTInvalidEvidenceAndScope(t *testing.T) {
	for _, kind := range []string{"missing", "checksum", "model", "future", "month", "source-item", "reference", "quantity", "unit", "precision", "negative", "base-zero", "rate-zero", "rate-over", "own-over", "retained-over", "missing-FCP", "FCP-mismatch", "FCP-rate", "FCP-zero", "unexpected-FCP", "other-code", "NFCe", "interstate", "CFOP", "new-ST", "new-FCP", "new-ICMS", "simple-credit"} {
		t.Run(kind, func(t *testing.T) {
			r := retainedRule("3", "60", true)
			item := retainedItem(t, true)
			s := item.RetainedST
			switch kind {
			case "missing":
				item.RetainedST = nil
			case "checksum":
				s.SourceKey = s.SourceKey[:43] + "x"
			case "model":
				s.SourceKey, _ = AccessKey(KeyInput{UF: "SP", IssuedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), CNPJ: "11222333000181", Model: "65", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
			case "future":
				s.SourceKey, _ = AccessKey(KeyInput{UF: "SP", IssuedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), CNPJ: "11222333000181", Model: "55", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
			case "month":
				prefix := s.SourceKey[:4] + "13" + s.SourceKey[6:43]
				s.SourceKey = prefix + string(rune('0'+accessDigit(prefix)))
			case "source-item":
				s.SourceItem = 991
			case "reference":
				s.ApprovalReference = ""
			case "quantity":
				s.Quantity = "3"
			case "unit":
				s.Unit = ""
			case "precision":
				s.Base = "140.001"
			case "negative":
				s.RetainedAmount = "-1"
			case "base-zero":
				s.Base = "0"
			case "rate-zero":
				s.SupportedRate = "0"
			case "rate-over":
				s.SupportedRate = "100.01"
			case "own-over":
				s.SubstituteAmount = "141"
			case "retained-over":
				s.RetainedAmount = "141"
			case "missing-FCP":
				s.FCP = nil
			case "FCP-mismatch":
				s.FCP.Amount = "2.81"
			case "FCP-rate":
				s.FCP.Rate = "21"
			case "FCP-zero":
				s.FCP.Rate = "0"
				s.FCP.Amount = "0"
			case "unexpected-FCP":
				r.Goods.FCPScope = "not_applicable"
			case "other-code":
				r.Goods.ICMS = "40"
				r.Goods.FCPScope = "not_applicable"
			case "NFCe":
				r.Model = "65"
			case "interstate":
				r.DestinationUF = "RJ"
				r.CFOP = "6405"
			case "CFOP":
				r.CFOP = "5102"
			case "new-ST":
				r.Components = append(r.Components, TaxComponent{Code: "ICMSST", Method: "percentage", Rate: "18", MVA: "0", ReductionPercent: "0", AddsToTotal: true})
			case "new-FCP":
				r.Components = append(r.Components, TaxComponent{Code: "FCP", Method: "percentage", Rate: "2", MVA: "0", ReductionPercent: "0"})
			case "new-ICMS":
				r.Components[0].Method = "percentage"
				r.Components[0].Rate = "18"
			case "simple-credit":
				r.Goods.SimpleCredit = &SimpleCreditInput{Percent: "1"}
			}
			if _, err := retainedCalculate(t, r, item); err == nil {
				t.Fatal("invalid evidence/scope accepted")
			}
		})
	}
}
func TestRetainedSTPolicyContextAndLegacy(t *testing.T) {
	r := retainedRule("1", "500", true)
	_, c, sources := taxPolicyExample()
	c.CEST = "0100100"
	c.RecipientStatus = "contributor"
	c.FinalConsumer = "no"
	c.Purpose = "resale"
	if err := ValidateTaxPolicy(r, c, sources); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"final", "status", "purpose", "CEST"} {
		bad := c
		switch kind {
		case "final":
			bad.FinalConsumer = "yes"
		case "status":
			bad.RecipientStatus = "noncontributor"
		case "purpose":
			bad.Purpose = "fixed_asset"
		case "CEST":
			bad.CEST = ""
		}
		if ValidateTaxPolicy(r, bad, sources) == nil {
			t.Fatal("unsupported case accepted", kind)
		}
	}
	legacy := classifiedGoodsRule("3", "00")
	out := classifiedGoodsCalculate(t, legacy)
	raw, _ := json.Marshal(out)
	if bytes.Contains(raw, []byte("retained_st")) {
		t.Fatal("legacy hash shape changed")
	}
}
