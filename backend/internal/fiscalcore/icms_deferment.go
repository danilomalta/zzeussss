package fiscalcore

import "math/big"

// Deferment is a client-reviewed incidence choice, never inferred from the UF.
// The original operation amount is rounded to cents before calculating the
// deferred share, so the three official XML values reconcile exactly.
type ICMSDefermentInput struct {
	Percent        string `json:"percent"`
	LegalReference string `json:"legal_reference"`
}
type ICMSDefermentResult struct {
	Percent         string `json:"percent"`
	OperationAmount string `json:"operation_amount"`
	DeferredAmount  string `json:"deferred_amount"`
	DueAmount       string `json:"due_amount"`
}

func CalculateICMSDeferment(operation string, in ICMSDefermentInput) (ICMSDefermentResult, error) {
	var out ICMSDefermentResult
	amount, e1 := decimal(operation)
	percent, e2 := decimal(in.Percent)
	if e1 != nil || e2 != nil || percent.Sign() <= 0 || percent.Cmp(big.NewRat(100, 1)) > 0 || !cleanTaxText(in.LegalReference, 2000) {
		return out, ErrRule
	}
	if _, e := xmlTaxNumber(operation, 13, 2); e != nil {
		return out, e
	}
	if _, e := exactScale(in.Percent, 4); e != nil {
		return out, e
	}
	deferred, _ := decimal(money(new(big.Rat).Quo(new(big.Rat).Mul(amount, percent), big.NewRat(100, 1))))
	due := new(big.Rat).Sub(amount, deferred)
	return ICMSDefermentResult{Percent: in.Percent, OperationAmount: money(amount), DeferredAmount: money(deferred), DueAmount: money(due)}, nil
}
func validateICMSDeferment(r TaxRule) error {
	count := 0
	for _, c := range r.Components {
		if c.Deferment == nil {
			continue
		}
		count++
		if r.Goods == nil || r.Goods.ICMS != "51" || !oneOf(r.CRT, "2", "3") || r.Model != "55" || r.OriginUF != r.DestinationUF || c.Code != "ICMS" || c.Method != "percentage" || c.AddsToTotal || len(c.DeductApplied) != 0 || c.AdjustedMVA != nil || !decimalZero(c.MVA) {
			return ErrUnsupported
		}
		if _, e := CalculateICMSDeferment("0.00", *c.Deferment); e != nil {
			return e
		}
	}
	if r.Goods != nil && r.Goods.ICMS == "51" {
		// Reduced-base benefit identifiers and FCP deferment require their own
		// classifications; do not silently use an own-ICMS percentage for either.
		if count != 1 || r.Goods.FCPScope != "not_applicable" || r.Goods.Destination != nil {
			return ErrUnsupported
		}
		for _, c := range r.Components {
			if c.Code == "ICMS" && !decimalZero(c.ReductionPercent) {
				return ErrUnsupported
			}
		}
	}
	return nil
}
