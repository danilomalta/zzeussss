package fiscalproduction

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// Pure contract mapping tests: no HTTP and no claim of acceptance by Focus.
func TestFocusMappingDocumentedFrozenTaxFields(t *testing.T) {
	fields := map[string]any{}
	nodes := []fiscalcore.TaxNode{
		{Name: "ICMS", Children: []fiscalcore.TaxNode{{Name: "ICMSSN102", Children: []fiscalcore.TaxNode{{Name: "orig", Value: "0"}, {Name: "CSOSN", Value: "102"}}}}},
		{Name: "PIS", Children: []fiscalcore.TaxNode{{Name: "PISNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}},
		{Name: "COFINS", Children: []fiscalcore.TaxNode{{Name: "COFINSNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}},
	}
	if err := focusTaxFields(fields, nodes); err != nil {
		t.Fatal(err)
	}
	if fields["icms_situacao_tributaria"] != "102" || fields["pis_situacao_tributaria"] != "07" || fields["cofins_situacao_tributaria"] != "07" {
		t.Fatal("tax classification was changed")
	}
	nodes = append(nodes, fiscalcore.TaxNode{Name: "unmapped-tax"})
	if err := focusTaxFields(map[string]any{}, nodes); !errors.Is(err, fiscalcore.ErrUnsupported) {
		t.Fatal("unmapped tax was silently discarded")
	}
	if focusReference("tenant-a", "sale") == focusReference("tenant-b", "sale") {
		t.Fatal("reference not tenant scoped")
	}
}
func TestFocusMappingOptionalConsumerAndConflict(t *testing.T) {
	template := fiscalcore.Invoice{Recipient: &fiscalcore.Party{CPF: "12345678909", Name: "Synthetic consumer", IEDestination: "9"}}
	body, err := focusRecipientPayload(template, fiscal.ProfileInput{State: "SP"}, map[string]any{"cnpj_emitente": "12345678000123", "valor_total": "1.00"})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil || payload["cpf_destinatario"] != template.Recipient.CPF || payload["valor_total"] != "1.00" {
		t.Fatal("optional recipient changed commercial values")
	}
	template.Recipient.CNPJ = "12345678000123"
	if _, err = focusRecipientPayload(template, fiscal.ProfileInput{State: "SP"}, map[string]any{}); !errors.Is(err, fiscalcore.ErrUnsupported) {
		t.Fatal("ambiguous recipient accepted")
	}
	if strings.Contains(string(body), "token") {
		t.Fatal("unexpected secret field")
	}
}
