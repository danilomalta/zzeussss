package localapi

import (
	"encoding/json"
	"testing"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestFiscalTaxVersionedTableDefaultsUnapprovedAndPreservesProvenance(t *testing.T) {
	f := fiscalFixture(t)
	input := taxPolicyFixture(f)
	table := fiscal.TaxTable{Version: 1, Revision: "accountant-v1", Regime: "simples_nacional", Policies: []fiscal.TaxPolicyInput{input}}
	raw, _ := json.Marshal(table)
	decoded, err := fiscal.DecodeTaxTable(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Policies[0].AccountantApproved() || decoded.Policies[0].Review == nil {
		t.Fatal("approval inferred")
	}
	var saved fiscal.TaxPolicy
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", decoded.Policies[0], 201, &saved)
	if saved.Input.Review.TableRevision != "accountant-v1" || saved.Input.Rule.ValidFrom != input.Rule.ValidFrom || len(saved.Input.Sources) != 1 || saved.Input.AccountantApproved() {
		t.Fatal("provenance lost")
	}
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", decoded.Policies[0], 200, &saved)
	for _, mutate := range []func(*fiscal.TaxTable){func(t *fiscal.TaxTable) { t.Policies[0].Review.AccountantApproved = true }, func(t *fiscal.TaxTable) { t.Regime = "lucro_real" }, func(t *fiscal.TaxTable) { t.Policies[0].Sources = nil }, func(t *fiscal.TaxTable) { t.Policies[0].Rule.ValidFrom = "" }} {
		copy := decoded
		copy.Policies = append([]fiscal.TaxPolicyInput(nil), decoded.Policies...)
		review := *decoded.Policies[0].Review
		copy.Policies[0].Review = &review
		mutate(&copy)
		body, _ := json.Marshal(copy)
		if _, e := fiscal.DecodeTaxTable(body); e == nil {
			t.Fatal("invalid provenance accepted")
		}
	}
}
