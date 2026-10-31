package fiscalcore

import (
	"bytes"
	"testing"
	"time"
)

func simpleCreditRule() TaxRule {
	r := classifiedGoodsRule("1", "101")
	r.ValidFrom = "2026-10-01"
	r.ValidUntil = "2026-10-31"
	r.Goods.SimpleCredit = &SimpleCreditInput{Percent: "2.45", RevenuePeriod: "2026-09", RecipientRegime: "normal", BaseMethod: "approved_net_operation", LegalReference: "Reviewed art. 23 LC123 fixture"}
	return r
}
func TestGoodsSimpleCreditOfficialXML(t *testing.T) {
	r := simpleCreditRule()
	out := classifiedGoodsCalculate(t, r)
	if out.GoodsXML.SimpleCredit == nil || out.GoodsXML.SimpleCredit.Amount != "2.45" || out.Total != "100.00" || out.GoodsXML.Totals["vICMS"] != "0.00" || out.Taxes[0].Amount != "0.00" {
		t.Fatalf("credit changed due tax or sale %+v", out)
	}
	in := testInvoice()
	in.Key.Model = "55"
	in.CRT = "1"
	in.FinalConsumer = "0"
	in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic recipient", IEDestination: "1", IE: "110042490114", Address: in.Issuer.Address}
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
	cred := testCredential(t)
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
	if !bytes.Contains(raw, []byte("<ICMSSN101>")) || !bytes.Contains(raw, []byte("<pCredSN>2.4500</pCredSN><vCredICMSSN>2.45</vCredICMSSN>")) {
		t.Fatal("missing official credit fields")
	}
}
func TestGoodsSimpleCreditRejectsInvalidClassification(t *testing.T) {
	for _, kind := range []string{"missing", "zero", "over100", "precision", "current-period", "old-period", "long-validity", "missing-reference", "normal-CRT", "nfce", "wrong-code", "unknown-base", "unknown-recipient"} {
		t.Run(kind, func(t *testing.T) {
			r := simpleCreditRule()
			switch kind {
			case "missing":
				r.Goods.SimpleCredit = nil
			case "zero":
				r.Goods.SimpleCredit.Percent = "0"
			case "over100":
				r.Goods.SimpleCredit.Percent = "100.01"
			case "precision":
				r.Goods.SimpleCredit.Percent = "2.12345"
			case "current-period":
				r.Goods.SimpleCredit.RevenuePeriod = "2026-10"
			case "old-period":
				r.Goods.SimpleCredit.RevenuePeriod = "2026-08"
			case "long-validity":
				r.ValidUntil = "2026-11-01"
			case "missing-reference":
				r.Goods.SimpleCredit.LegalReference = ""
			case "normal-CRT":
				r.CRT = "3"
			case "nfce":
				r.Model = "65"
			case "wrong-code":
				r.Goods.ICMS = "102"
			case "unknown-base":
				r.Goods.SimpleCredit.BaseMethod = "automatic"
			case "unknown-recipient":
				r.Goods.SimpleCredit.RecipientRegime = "simple"
			}
			if ValidateGoodsTaxClassification(r) == nil {
				t.Fatal("invalid credit accepted")
			}
		})
	}
	r := simpleCreditRule()
	_, c, sources := taxPolicyExample()
	c.RecipientStatus = "contributor"
	c.FinalConsumer = "no"
	c.Purpose = "resale"
	c.ProductOrigin = "0"
	if err := ValidateTaxPolicy(r, c, sources); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"noncontributor", "final", "consumption"} {
		t.Run(kind, func(t *testing.T) {
			bad := c
			switch kind {
			case "noncontributor":
				bad.RecipientStatus = "noncontributor"
			case "final":
				bad.FinalConsumer = "yes"
			case "consumption":
				bad.Purpose = "final_consumption"
			}
			if ValidateTaxPolicy(r, bad, sources) == nil {
				t.Fatal("ineligible case accepted")
			}
		})
	}
}
