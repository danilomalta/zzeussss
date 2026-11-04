package fiscalproduction

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFocusTransportDocumentedFieldsWithoutFreightOrTaxDefaults(t *testing.T) {
	for _, mode := range []string{"0", "1", "2", "3", "4"} {
		req := DocumentRequest{FreightMode: mode, Transport: &FocusTransport{CPF: "12345678909", Name: "Synthetic carrier", Address: "Synthetic road", City: "Synthetic city", UF: "SP"}}
		dst := map[string]any{}
		if err := focusTransportFields(dst, req); err != nil {
			t.Fatal(err)
		}
		if dst["cpf_transportador"] != "12345678909" || dst["uf_transportador"] != "SP" {
			t.Fatal("documented mapping")
		}
		for _, k := range []string{"valor_frete", "transporte_icms_aliquota", "transporte_icms_cfop"} {
			if _, ok := dst[k]; ok {
				t.Fatal("invented value")
			}
		}
		raw, _ := json.Marshal(dst)
		if strings.Contains(string(raw), "token") {
			t.Fatal("credential")
		}
	}
	for _, in := range []DocumentRequest{{FreightMode: ""}, {FreightMode: "8"}, {FreightMode: "9", Transport: &FocusTransport{CPF: "12345678909", Name: "Test"}}, {FreightMode: "0", Transport: &FocusTransport{CPF: "bad", Name: "Test"}}, {FreightMode: "0", Transport: &FocusTransport{CPF: "12345678909", Name: "Test\n"}}, {FreightMode: "0", Transport: &FocusTransport{CPF: "12345678909", CNPJ: "12345678000123", Name: "Test"}}} {
		if focusTransportFields(map[string]any{}, in) == nil {
			t.Fatal("invalid transport accepted")
		}
	}
}
