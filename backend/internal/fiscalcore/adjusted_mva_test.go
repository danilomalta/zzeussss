package fiscalcore

import (
	"bytes"
	"testing"
	"time"
)

func TestAdjustedMVAApprovedFormulaAndOfficialXML(t *testing.T) {
	r := classifiedGoodsRule("3", "10")
	r.CFOP = "6102"
	r.DestinationUF = "BA"
	r.Components[0].Rate = "7"
	input := AdjustedMVAInput{OriginalPercent: "40", InterstateRate: "7", InternalRate: "18", Rounding: "four_places_before_base", LegalReference: "Synthetic applicability review; formula described in SC normative resolution72"}
	result, err := AdjustMVA(input)
	if err != nil || result.AppliedPercent != "58.7805" {
		t.Fatal(result, err)
	}
	r.Components[1].MVA = "0"
	r.Components[1].AdjustedMVA = &input
	out := classifiedGoodsCalculate(t, r)
	if out.Total != "121.58" || out.GoodsXML.Totals["vBCST"] != "158.78" || out.GoodsXML.Totals["vST"] != "21.58" || out.Taxes[1].MVAAdjustment == nil {
		t.Fatalf("adjustment lost %+v", out)
	}
	in := testInvoice()
	in.Key.Model = "55"
	in.CRT = "3"
	in.Destination = "2"
	address := in.Issuer.Address
	address.UF = "BA"
	address.CityCode = "2927408"
	address.City = "Salvador"
	in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic buyer", IEDestination: "1", IE: "123456789", Address: address}
	in.Lines[0].ProductID = r.ProductID
	in.Lines[0].Quantity = "2"
	in.Lines[0].UnitPrice = "50"
	in.Lines[0].NCM = r.NCM
	in.Lines[0].CFOP = r.CFOP
	in.Lines[0].Taxes = out.GoodsXML.Taxes
	in.TaxTotals = out.GoodsXML.Totals
	in.Payments[0].Amount = out.Total
	raw, _, err := BuildInvoice(in)
	if err != nil {
		t.Fatal(err)
	}
	cred := testCredential(t)
	signed, err := cred.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateXML("nfe", signed); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("<pMVAST>58.7805</pMVAST>")) {
		t.Fatal("adjusted MVA missing from XML")
	}
}
func TestAdjustedMVARejectsUnknownOrConflictingInputs(t *testing.T) {
	for _, kind := range []string{"missing-rounding", "missing-law", "zero-denominator", "negative-adjustment", "internal-operation", "double-MVA", "rate-disagreement"} {
		t.Run(kind, func(t *testing.T) {
			input := AdjustedMVAInput{OriginalPercent: "40", InterstateRate: "7", InternalRate: "18", Rounding: "four_places_before_base", LegalReference: "Synthetic applicability approval"}
			r := classifiedGoodsRule("3", "10")
			r.CFOP = "6102"
			r.DestinationUF = "BA"
			r.Components[0].Rate = "7"
			r.Components[1].MVA = "0"
			r.Components[1].AdjustedMVA = &input
			switch kind {
			case "missing-rounding":
				input.Rounding = ""
			case "missing-law":
				input.LegalReference = ""
			case "zero-denominator":
				input.InternalRate = "100"
			case "negative-adjustment":
				input.OriginalPercent = "0"
				input.InterstateRate = "12"
				input.InternalRate = "7"
				r.Components[0].Rate = "12"
				r.Components[1].Rate = "7"
			case "internal-operation":
				r.DestinationUF = r.OriginUF
				r.CFOP = "5102"
			case "double-MVA":
				r.Components[1].MVA = "40"
			case "rate-disagreement":
				input.InterstateRate = "4"
			}
			if ValidateGoodsTaxClassification(r) == nil {
				t.Fatal("unsupported MVA adjustment accepted")
			}
		})
	}
}
