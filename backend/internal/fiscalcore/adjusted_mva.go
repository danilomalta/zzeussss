package fiscalcore

import "math/big"

type AdjustedMVAInput struct {
	OriginalPercent string `json:"original_percent"`
	InterstateRate  string `json:"interstate_rate"`
	InternalRate    string `json:"internal_rate"`
	Rounding        string `json:"rounding"`
	LegalReference  string `json:"legal_reference"`
}
type AdjustedMVAResult struct {
	Input          AdjustedMVAInput `json:"input"`
	ExactPercent   string           `json:"exact_percent"`
	AppliedPercent string           `json:"applied_percent"`
}

// AdjustedMVA applies only an explicitly chosen adjustment formula. It neither
// selects original MVAs by NCM nor declares an interstate ST agreement valid.
func AdjustMVA(in AdjustedMVAInput) (AdjustedMVAResult, error) {
	var out AdjustedMVAResult
	if in.Rounding != "four_places_before_base" || !cleanTaxText(in.LegalReference, 2000) {
		return out, ErrRule
	}
	original, e1 := decimal(in.OriginalPercent)
	inter, e2 := decimal(in.InterstateRate)
	intra, e3 := decimal(in.InternalRate)
	if e1 != nil || e2 != nil || e3 != nil || original.Cmp(big.NewRat(1000, 1)) > 0 || inter.Cmp(big.NewRat(100, 1)) >= 0 || intra.Cmp(big.NewRat(100, 1)) >= 0 {
		return out, ErrRule
	}
	hundred := big.NewRat(100, 1)
	ratio := new(big.Rat).Quo(new(big.Rat).Sub(hundred, inter), new(big.Rat).Sub(hundred, intra))
	result := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Add(hundred, original), ratio), hundred)
	if result.Sign() < 0 || result.Cmp(big.NewRat(1000, 1)) > 0 {
		return out, ErrUnsupported
	}
	return AdjustedMVAResult{Input: in, ExactPercent: result.RatString(), AppliedPercent: result.FloatString(4)}, nil
}
func validateAdjustedMVA(r TaxRule) error {
	var own, st string
	for _, c := range r.Components {
		if c.Code == "ICMS" {
			own = c.Rate
		}
		if c.Code == "ICMSST" {
			st = c.Rate
		}
	}
	for _, c := range r.Components {
		if c.AdjustedMVA == nil {
			continue
		}
		if r.OriginUF == r.DestinationUF || !oneOf(c.Code, "ICMSST", "FCPST") || c.Method != "percentage" || !decimalZero(c.MVA) {
			return ErrRule
		}
		resolved, err := AdjustMVA(*c.AdjustedMVA)
		if err != nil {
			return err
		}
		if _, err = decimal(resolved.AppliedPercent); err != nil {
			return err
		}
		if own == "" || st == "" {
			return ErrRule
		}
		a, e1 := decimal(own)
		b, e2 := decimal(c.AdjustedMVA.InterstateRate)
		d, e3 := decimal(st)
		e, e4 := decimal(c.AdjustedMVA.InternalRate)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || a.Cmp(b) != 0 || d.Cmp(e) != 0 {
			return ErrRule
		}
	}
	return nil
}
