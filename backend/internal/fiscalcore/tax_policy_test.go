package fiscalcore

import (
	"strings"
	"testing"
)

func taxPolicyExample() (TaxRule, TaxCase, []string) {
	return TaxRule{ID: "test-policy", ProductID: "product-one", CFOP: "5102", NCM: "19059090", OriginUF: "SP", DestinationUF: "SP", CRT: "3", Model: "55", ValidFrom: "2026-01-01", ValidUntil: "2026-12-31", LegalReference: "Synthetic calculation fixture; not a legal classification", ApprovalReference: "test-owner-review", BaseRounding: "tax_final", Components: []TaxComponent{{Code: "ICMS", Method: "percentage", Rate: "18", ReductionPercent: "0", MVA: "0"}}},
		TaxCase{Operation: "sale", Purpose: "final_consumption", OriginMunicipality: "3550308", DestinationMunicipality: "3550308", RecipientStatus: "noncontributor", FinalConsumer: "yes", ProductOrigin: "0"}, []string{"https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp87.htm"}
}
func TestTaxPolicyGrossUpAndExactRounding(t *testing.T) {
	r, _, _ := taxPolicyExample()
	c := TaxContext{OriginUF: "SP", DestinationUF: "SP", CRT: "3", Model: "55", Date: "2026-06-01"}
	item := TaxItem{ProductID: r.ProductID, Quantity: "1", UnitPrice: "82", Discount: "0", Freight: "0", Insurance: "0", Other: "0"}
	r.Components[0].Method = "gross_up"
	r.Components[0].AddsToTotal = true
	out, err := Calculate(c, item, []TaxRule{r})
	if err != nil || out.Taxes[0].Base != "100.00" || out.Taxes[0].Amount != "18.00" || out.Total != "100.00" || out.OfficialAuthorization {
		t.Fatalf("gross up: %+v %v", out, err)
	}
	r.Components[0].Method = "percentage"
	r.Components[0].Rate = "50"
	r.Components[0].ReductionPercent = "50"
	r.Components[0].AddsToTotal = false
	item.UnitPrice = "0.01"
	out, err = Calculate(c, item, []TaxRule{r})
	if err != nil || out.Taxes[0].Amount != "0.00" || out.Taxes[0].UnroundedBase != "1/200" {
		t.Fatalf("unrounded base: %+v %v", out, err)
	}
	r.BaseRounding = "before_tax"
	out, err = Calculate(c, item, []TaxRule{r})
	if err != nil || out.Taxes[0].Amount != "0.01" {
		t.Fatalf("explicit base rounding: %+v %v", out, err)
	}
	r.Components[0].Method = "gross_up"
	r.Components[0].Rate = "100"
	if _, err = Calculate(c, item, []TaxRule{r}); err == nil {
		t.Fatal("division by zero accepted")
	}
	item.Quantity = strings.Repeat("9", 18)
	item.UnitPrice = item.Quantity
	if _, err = Calculate(c, item, []TaxRule{r}); err == nil {
		t.Fatal("oversize monetary multiplication accepted")
	}
}
func TestTaxPolicyValidationAndAllUFContexts(t *testing.T) {
	r, c, sources := taxPolicyExample()
	if err := ValidateTaxPolicy(r, c, sources); err != nil {
		t.Fatal(err)
	}
	// This validates geography syntax, not a nationwide tax-rate catalogue.
	for uf, code := range ufCodes {
		t.Run(uf, func(t *testing.T) {
			rule := r
			rule.OriginUF = uf
			rule.DestinationUF = uf
			condition := c
			condition.OriginMunicipality = code + "00000"
			condition.DestinationMunicipality = code + "00000"
			if err := ValidateTaxPolicy(rule, condition, sources); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, raw := range []string{"http://www.planalto.gov.br/law", "https://example.org/law", "https://www.gov.br.evil.example/law", "https://user:pass@www.gov.br/law", "https://www.gov.br:443/law"} {
		if ValidateTaxPolicy(r, c, []string{raw}) == nil {
			t.Fatal("invalid provenance accepted")
		}
	}
	tests := map[string]func(*TaxRule, *TaxCase){
		"missing rounding":      func(r *TaxRule, _ *TaxCase) { r.BaseRounding = "" },
		"unknown origin":        func(_ *TaxRule, c *TaxCase) { c.ProductOrigin = "9" },
		"missing recipient":     func(_ *TaxRule, c *TaxCase) { c.RecipientStatus = "" },
		"municipality mismatch": func(_ *TaxRule, c *TaxCase) { c.DestinationMunicipality = "3304557" },
		"foreign unsupported":   func(_ *TaxRule, c *TaxCase) { c.Operation = "export" },
		"bad CEST":              func(_ *TaxRule, c *TaxCase) { c.CEST = "123" },
		"negative rate":         func(r *TaxRule, _ *TaxCase) { r.Components[0].Rate = "-1" },
		"unknown method":        func(r *TaxRule, _ *TaxCase) { r.Components[0].Method = "automatic" },
		"fake exemption":        func(r *TaxRule, _ *TaxCase) { r.Components[0].Method = "exempt" },
		"future dependency":     func(r *TaxRule, _ *TaxCase) { r.Components[0].AddApplied = []string{"IPI"} },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			r, c, s := taxPolicyExample()
			change(&r, &c)
			if ValidateTaxPolicy(r, c, s) == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}
