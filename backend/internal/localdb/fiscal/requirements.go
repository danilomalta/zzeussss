package fiscal

import (
	_ "embed"
	"encoding/json"
)

//go:embed uf_requirements.json
var requirementsJSON []byte

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
type UFRequirements struct {
	UF                         string   `json:"uf"`
	Name                       string   `json:"name"`
	ReferenceDate              string   `json:"reference_date"`
	ProductionValidated        bool     `json:"production_validated"`
	NFe55AuthorizerDocumented  string   `json:"nfe55_authorizer_documented"`
	NFCe65AuthorizerDocumented string   `json:"nfce65_authorizer_documented"`
	IssuerRequirements         string   `json:"issuer_requirements"`
	DeveloperRequirements      string   `json:"developer_requirements"`
	Limitations                string   `json:"limitations"`
	Verification               string   `json:"verification"`
	Sources                    []Source `json:"sources"`
}

// Requirements is a dated documentary reference, not live tax advice or an
// endpoint-routing configuration. Unknown requirements never imply exemption.
// It cannot activate transmission or select an issuer's rates.
func Requirements(uf string) (UFRequirements, error) {
	if _, ok := stateCodes[uf]; !ok {
		return UFRequirements{}, ErrInvalid
	}
	var rows []UFRequirements
	if json.Unmarshal(requirementsJSON, &rows) != nil {
		return UFRequirements{}, ErrIntegrity
	}
	for _, r := range rows {
		if r.UF == uf {
			return r, nil
		}
	}
	return UFRequirements{}, ErrIntegrity
}
