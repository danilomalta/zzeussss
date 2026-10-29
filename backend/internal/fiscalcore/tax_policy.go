package fiscalcore

import (
	"math/big"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// TaxCase is explicit classification context. It is never inferred from an
// establishment's UF alone, or inherited from a different customer's policy.
type TaxCase struct {
	Operation               string `json:"operation"`
	Purpose                 string `json:"purpose"`
	OriginMunicipality      string `json:"origin_municipality"`
	DestinationMunicipality string `json:"destination_municipality"`
	RecipientStatus         string `json:"recipient_status"`
	FinalConsumer           string `json:"final_consumer"`
	ProductOrigin           string `json:"product_origin"`
	CEST                    string `json:"cest"`
}

func cleanTaxText(s string, max int) bool {
	if len(s) < 1 || len(s) > max || !utf8.ValidString(s) || strings.TrimSpace(s) != s {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// ValidateTaxPolicy checks representable calculation/classification data. A
// legal URL is provenance supplied for review, not a claim that its applicability
// was automatically verified. No rates, exemption or collection duty are guessed.
func ValidateTaxPolicy(r TaxRule, c TaxCase, sources []string) error {
	if !cleanTaxText(r.ID, 128) || !cleanTaxText(r.ProductID, 128) || !cleanTaxText(r.LegalReference, 2000) || !cleanTaxText(r.ApprovalReference, 512) ||
		(r.CRT != "1" && r.CRT != "2" && r.CRT != "3" && r.CRT != "4") || (r.Model != "55" && r.Model != "65") ||
		len(r.CFOP) != 4 || !digitsRE.MatchString(r.CFOP) || len(r.NCM) != 8 || !digitsRE.MatchString(r.NCM) ||
		(r.BaseRounding != "before_tax" && r.BaseRounding != "tax_final") || len(r.Components) < 1 || len(r.Components) > 16 || len(sources) < 1 || len(sources) > 16 {
		return ErrRule
	}
	from, e1 := time.Parse("2006-01-02", r.ValidFrom)
	until, e2 := time.Parse("2006-01-02", r.ValidUntil)
	if e1 != nil || e2 != nil || until.Before(from) {
		return ErrRule
	}
	if ValidateTaxCase(c, r.OriginUF, r.DestinationUF) != nil {
		return ErrRule
	}
	seenSources := map[string]bool{}
	for _, raw := range sources {
		u, err := url.Parse(raw)
		if !cleanTaxText(raw, 2048) || err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || !(strings.HasSuffix(strings.ToLower(u.Hostname()), ".gov.br") || strings.HasSuffix(strings.ToLower(u.Hostname()), ".leg.br")) || seenSources[raw] {
			return ErrRule
		}
		seenSources[raw] = true
	}
	if err := validateICMSDeferment(r); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, t := range r.Components {
		if !codeRE.MatchString(t.Code) || seen[t.Code] {
			return ErrRule
		}
		rate, e := decimal(t.Rate)
		if e != nil {
			return ErrRule
		}
		reduction, e := decimal(t.ReductionPercent)
		if e != nil || reduction.Cmp(big.NewRat(100, 1)) > 0 {
			return ErrRule
		}
		mva, e := decimal(t.MVA)
		if e != nil || mva.Cmp(big.NewRat(1000, 1)) > 0 {
			return ErrRule
		}
		refs := map[string]bool{}
		for _, list := range [][]string{t.AddApplied, t.SubtractApplied, t.DeductApplied} {
			for _, key := range list {
				if !seen[key] || refs[key] {
					return ErrRule
				}
				refs[key] = true
			}
		}
		switch t.Method {
		case "percentage":
			if rate.Cmp(big.NewRat(100, 1)) > 0 {
				return ErrRule
			}
		case "gross_up":
			if rate.Cmp(big.NewRat(100, 1)) >= 0 {
				return ErrRule
			}
		case "per_unit":
			if len(t.AddApplied)+len(t.SubtractApplied) != 0 || reduction.Sign() != 0 || mva.Sign() != 0 {
				return ErrRule
			}
		case "exempt":
			if rate.Sign() != 0 || len(refs) != 0 || t.AddsToTotal || reduction.Sign() != 0 || mva.Sign() != 0 {
				return ErrRule
			}
		default:
			return ErrUnsupported
		}
		seen[t.Code] = true
	}
	if r.Goods != nil {
		if r.Goods.SimpleCredit != nil && (c.RecipientStatus != "contributor" || c.FinalConsumer != "no" || !oneOf(c.Purpose, "resale", "industrialization")) {
			return ErrRule
		}
		if c.Operation != "sale" {
			return ErrUnsupported
		}
		if r.Goods.Destination != nil && (c.FinalConsumer != "yes" || c.RecipientStatus != "noncontributor" || c.Purpose != "final_consumption") {
			return ErrRule
		}
		if r.OriginUF != r.DestinationUF {
			if c.FinalConsumer == "yes" {
				if c.RecipientStatus != "noncontributor" || c.Purpose != "final_consumption" || r.Goods.Destination == nil {
					return ErrUnsupported
				}
			} else if c.RecipientStatus != "contributor" || !oneOf(c.Purpose, "resale", "industrialization") {
				return ErrUnsupported
			}
		}
		if r.Goods.Origin != c.ProductOrigin {
			return ErrRule
		}
		return ValidateGoodsTaxClassification(r)
	}
	return validateAdjustedMVA(r)
}

func ValidateTaxCase(c TaxCase, origin, destination string) error {
	o, ok := ufCodes[origin]
	if !ok {
		return ErrInput
	}
	d, ok := ufCodes[destination]
	if !ok {
		return ErrInput
	}
	for _, v := range []struct{ s, prefix string }{{c.OriginMunicipality, o}, {c.DestinationMunicipality, d}} {
		if len(v.s) != 7 || !digitsRE.MatchString(v.s) || !strings.HasPrefix(v.s, v.prefix) {
			return ErrInput
		}
	}
	switch c.Operation {
	case "sale", "purchase", "return", "transfer", "import", "export":
	default:
		return ErrInput
	}
	switch c.Purpose {
	case "resale", "industrialization", "final_consumption", "fixed_asset":
	default:
		return ErrInput
	}
	if c.RecipientStatus != "contributor" && c.RecipientStatus != "exempt_registration" && c.RecipientStatus != "noncontributor" {
		return ErrInput
	}
	if c.FinalConsumer != "yes" && c.FinalConsumer != "no" {
		return ErrInput
	}
	if len(c.ProductOrigin) != 1 || c.ProductOrigin[0] < '0' || c.ProductOrigin[0] > '8' {
		return ErrInput
	}
	if c.CEST != "" && (len(c.CEST) != 7 || !digitsRE.MatchString(c.CEST)) {
		return ErrInput
	}
	// Foreign destinations require country/locality rules not represented here.
	if c.Operation == "import" || c.Operation == "export" {
		return ErrUnsupported
	}
	return nil
}
