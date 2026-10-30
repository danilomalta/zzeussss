package fiscalcore

import "regexp"

var fciRE = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)

// This dated reference is only the nominal interstate ICMS rate for ordinary
// goods. It is not an internal rate table, a CAMEX/FCI classifier, a benefit or
// a statement that ICMS is payable. Origin codes must be previously classified.
const InterstateGoods2026 = "ordinary_goods_2026"

type InterstateRateSelection struct {
	Reference     string   `json:"reference"`
	Rate          string   `json:"rate"`
	OriginUF      string   `json:"origin_uf"`
	DestinationUF string   `json:"destination_uf"`
	ProductOrigin string   `json:"product_origin"`
	ValidFrom     string   `json:"valid_from"`
	ValidUntil    string   `json:"valid_until"`
	Sources       []string `json:"sources"`
}

func InterstateGoodsRate(origin, destination, productOrigin string) (InterstateRateSelection, error) {
	var out InterstateRateSelection
	if _, ok := ufCodes[origin]; !ok {
		return out, ErrInput
	}
	if _, ok := ufCodes[destination]; !ok {
		return out, ErrInput
	}
	if origin == destination || !oneOf(productOrigin, "0", "1", "2", "3", "4", "5", "6", "7", "8") {
		return out, ErrInput
	}
	rate := "12.0000"
	if oneOf(origin, "MG", "PR", "RJ", "RS", "SC", "SP") && !oneOf(destination, "MG", "PR", "RJ", "RS", "SC", "SP") {
		rate = "7.0000"
	}
	if oneOf(productOrigin, "1", "2", "3", "8") {
		rate = "4.0000"
	}
	out = InterstateRateSelection{Reference: InterstateGoods2026, Rate: rate, OriginUF: origin, DestinationUF: destination, ProductOrigin: productOrigin, ValidFrom: "2026-01-01", ValidUntil: "2026-12-31", Sources: []string{
		"https://www2.camara.leg.br/legin/fed/ressen/1989/resolucao-22-19-maio-1989-481183-publicacaooriginal-1-pl.html",
		"https://www.nfe.fazenda.gov.br/Portal/exibirArquivo.aspx?conteudo=slmjSi5sbdk%3D",
		"https://orientacaotributaria.economia.go.gov.br/spo-web/perguntasfrequentes/perguntafrequente/21629",
	}}
	return out, nil
}

func validateInterstatePolicy(r TaxRule) (*InterstateRateSelection, error) {
	if r.Goods == nil || r.Goods.InterstateBasis == "" {
		return nil, nil
	}
	if r.Goods.InterstateBasis != InterstateGoods2026 || r.Model != "55" || !oneOf(r.CRT, "2", "3") || !oneOf(r.Goods.ICMS, "00", "10", "20", "70") || r.ValidFrom < "2026-01-01" || r.ValidUntil > "2026-12-31" || len(r.CFOP) != 4 || r.CFOP[0] != '6' || len(r.NCM) != 8 || r.NCM[:2] == "27" {
		return nil, ErrUnsupported
	}
	chosen, err := InterstateGoodsRate(r.OriginUF, r.DestinationUF, r.Goods.Origin)
	if err != nil {
		return nil, err
	}
	for _, t := range r.Components {
		if t.Code == "ICMS" {
			rate, e := exactScale(t.Rate, 4)
			if e != nil || rate != chosen.Rate {
				return nil, ErrRule
			}
			return &chosen, nil
		}
	}
	return nil, ErrRule
}
