package fiscalproduction

import (
	"strings"
	"titansystem-backend/internal/localdb/fiscal"
	"unicode"
	"unicode/utf8"
)

// Field names and lengths: official homologation NotaFiscalXML, modFrete/transp.
// No freight charge or tax is inferred from these operator-supplied identities.
type FocusTransport struct {
	CNPJ    string `json:"cnpj"`
	CPF     string `json:"cpf"`
	Name    string `json:"name"`
	IE      string `json:"ie"`
	Address string `json:"address"`
	City    string `json:"city"`
	UF      string `json:"uf"`
}

func transportText(s string, min, max int) bool {
	if !utf8.ValidString(s) || strings.TrimSpace(s) != s || utf8.RuneCountInString(s) < min || utf8.RuneCountInString(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func focusTransportFields(dst map[string]any, in DocumentRequest) error {
	switch in.FreightMode {
	case "0", "1", "2", "3", "4", "9":
	default:
		return ErrFocusNFeIncomplete
	}
	t := in.Transport
	if t == nil {
		return nil
	}
	if in.FreightMode == "9" || (t.CNPJ == "") == (t.CPF == "") || (t.CNPJ != "" && !fiscal.ValidCNPJ(t.CNPJ)) || (t.CPF != "" && !validFocusCPF(t.CPF)) || !transportText(t.Name, 2, 60) {
		return ErrFocusNFeIncomplete
	}
	for _, v := range []struct {
		s        string
		min, max int
	}{{t.IE, 2, 14}, {t.Address, 1, 60}, {t.City, 1, 60}, {t.UF, 2, 2}} {
		if v.s != "" && !transportText(v.s, v.min, v.max) {
			return ErrFocusNFeIncomplete
		}
	}
	if t.UF != "" && !strings.Contains("|AC|AL|AP|AM|BA|CE|DF|ES|GO|MA|MT|MS|MG|PA|PB|PR|PE|PI|RJ|RN|RS|RO|RR|SC|SP|SE|TO|", "|"+t.UF+"|") {
		return ErrFocusNFeIncomplete
	}
	for k, v := range map[string]string{"cnpj_transportador": t.CNPJ, "cpf_transportador": t.CPF, "nome_transportador": t.Name, "inscricao_estadual_transportador": t.IE, "endereco_transportador": t.Address, "municipio_transportador": t.City, "uf_transportador": t.UF} {
		if v != "" {
			dst[k] = v
		}
	}
	return nil
}
