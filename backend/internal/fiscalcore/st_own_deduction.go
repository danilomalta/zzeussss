package fiscalcore

import "math/big"

// ST deduction is not the SN credit, not an own-tax highlight and not DAS.
// The effective base/rate must be approved under the applicable legislation.
type STOwnDeductionInput struct {
	Rate             string `json:"rate"`
	ReductionPercent string `json:"reduction_percent"`
	BaseMethod       string `json:"base_method"`
	LegalReference   string `json:"legal_reference"`
}
type STOwnDeductionResult struct {
	Base   string `json:"base"`
	Rate   string `json:"rate"`
	Amount string `json:"amount"`
}

func CalculateSTOwnDeduction(net string, in STOwnDeductionInput) (STOwnDeductionResult, error) {
	var out STOwnDeductionResult
	amount, e1 := decimal(net)
	rate, e2 := decimal(in.Rate)
	reduction, e3 := decimal(in.ReductionPercent)
	if e1 != nil || e2 != nil || e3 != nil || rate.Cmp(big.NewRat(100, 1)) > 0 || reduction.Cmp(big.NewRat(100, 1)) > 0 || in.BaseMethod != "approved_net_operation" || !cleanTaxText(in.LegalReference, 2000) {
		return out, ErrRule
	}
	for _, s := range []string{in.Rate, in.ReductionPercent} {
		if _, err := exactScale(s, 4); err != nil {
			return out, err
		}
	}
	if _, err := xmlTaxNumber(net, 13, 2); err != nil {
		return out, err
	}
	base, _ := decimal(money(new(big.Rat).Mul(amount, new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(100, 1), reduction), big.NewRat(100, 1)))))
	value := money(new(big.Rat).Quo(new(big.Rat).Mul(base, rate), big.NewRat(100, 1)))
	if _, err := xmlTaxNumber(value, 13, 2); err != nil {
		return out, err
	}
	return STOwnDeductionResult{Base: money(base), Rate: in.Rate, Amount: value}, nil
}
func validateSTOwnDeduction(r TaxRule) error {
	count := 0
	for _, c := range r.Components {
		if c.OwnOperationDeduction == nil {
			continue
		}
		count++
		if r.Goods == nil || r.CRT != "1" || r.Model != "55" || !oneOf(r.Goods.ICMS, "201", "202", "203") || c.Code != "ICMSST" || c.Method != "percentage" || !c.AddsToTotal || len(c.DeductApplied) != 0 || c.AdjustedMVA != nil || c.Deferment != nil {
			return ErrUnsupported
		}
		if _, err := CalculateSTOwnDeduction("0.00", *c.OwnOperationDeduction); err != nil {
			return err
		}
	}
	if r.Goods != nil && oneOf(r.Goods.ICMS, "201", "202", "203") && count != 1 {
		return ErrRule
	}
	return nil
}

// appendGoodsST preserves the normal-regime legacy ordering; optional zero
// percentages are omitted for SN groups whose official optional type is >0.
func appendGoodsST(nodes []TaxNode, out *GoodsTaxMapping, values map[string]AppliedTax, components map[string]TaxComponent, simple bool) ([]TaxNode, error) {
	st, ok := values["ICMSST"]
	if !ok {
		return nodes, nil
	}
	c := components["ICMSST"]
	mva, err := exactScale(c.MVA, 4)
	if err != nil {
		return nil, err
	}
	if st.MVAAdjustment != nil {
		mva, err = xmlTaxNumber(st.MVAAdjustment.AppliedPercent, 3, 4)
		if err != nil {
			return nil, err
		}
	}
	reduction, err := exactScale(c.ReductionPercent, 4)
	if err != nil {
		return nil, err
	}
	rate, err := exactScale(st.Rate, 4)
	if err != nil {
		return nil, err
	}
	nodes = append(nodes, taxLeaf("modBCST", "4"))
	if !simple || !decimalZero(mva) {
		nodes = append(nodes, taxLeaf("pMVAST", mva))
	}
	if !simple || !decimalZero(reduction) {
		nodes = append(nodes, taxLeaf("pRedBCST", reduction))
	}
	nodes = append(nodes, taxLeaf("vBCST", st.Base), taxLeaf("pICMSST", rate), taxLeaf("vICMSST", st.Amount))
	out.Totals["vBCST"] = st.Base
	out.Totals["vST"] = st.Amount
	if fcp, ok := values["FCPST"]; ok {
		if fcp.Base != st.Base {
			return nil, ErrUnsupported
		}
		rate, err = exactScale(fcp.Rate, 4)
		if err != nil {
			return nil, err
		}
		if simple && decimalZero(rate) {
			return nil, ErrRule
		}
		nodes = append(nodes, taxLeaf("vBCFCPST", fcp.Base), taxLeaf("pFCPST", rate), taxLeaf("vFCPST", fcp.Amount))
		out.Totals["vFCPST"] = fcp.Amount
	}
	return nodes, nil
}
