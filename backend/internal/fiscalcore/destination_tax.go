package fiscalcore

import "math/big"

type DestinationTaxClassification struct {
	Reference  string `json:"reference"`
	BaseMethod string `json:"base_method"`
	FCPScope   string `json:"fcp_scope"`
}

const DestinationTax2026 = "cv236_21_single_base_2026"

func validateDestinationTax(r TaxRule, components map[string]TaxComponent) error {
	g := r.Goods
	_, hasDIFAL := components["DIFAL"]
	_, hasFCP := components["FCPUFDEST"]
	if g.Destination == nil {
		if hasDIFAL || hasFCP {
			return ErrRule
		}
		return nil
	}
	d := g.Destination
	if d.Reference != DestinationTax2026 || d.BaseMethod != "single_inclusive_operation_base" || !oneOf(d.FCPScope, "not_applicable", "taxed") || r.Model != "55" || g.ICMS != "00" || g.InterstateBasis != InterstateGoods2026 || g.IPIScope != "not_applicable" || g.FCPScope != "not_applicable" || g.BenefitCode != "" {
		return ErrUnsupported
	}
	if !hasDIFAL || hasFCP != (d.FCPScope == "taxed") {
		return ErrRule
	}
	own := components["ICMS"]
	if len(own.AddApplied)+len(own.SubtractApplied) != 0 {
		return ErrUnsupported
	}
	difal := components["DIFAL"]
	if difal.Method != "percentage" || difal.AddsToTotal || !decimalZero(difal.MVA) || !decimalZero(difal.ReductionPercent) || len(difal.AddApplied)+len(difal.SubtractApplied) != 0 || len(difal.DeductApplied) != 1 || difal.DeductApplied[0] != "ICMS" {
		return ErrRule
	}
	inter, err := decimal(own.Rate)
	if err != nil {
		return err
	}
	intra, err := decimal(difal.Rate)
	if err != nil || intra.Cmp(inter) < 0 || intra.Cmp(big.NewRat(100, 1)) > 0 {
		return ErrRule
	}
	if hasFCP {
		fcp := components["FCPUFDEST"]
		rate, err := decimal(fcp.Rate)
		if err != nil || rate.Sign() <= 0 || rate.Cmp(big.NewRat(2, 1)) > 0 || fcp.Method != "percentage" || fcp.AddsToTotal || !decimalZero(fcp.MVA) || !decimalZero(fcp.ReductionPercent) || len(fcp.AddApplied)+len(fcp.SubtractApplied)+len(fcp.DeductApplied) != 0 {
			return ErrRule
		}
	}
	return nil
}

func appendDestinationTax(out *GoodsTaxMapping, r TaxRule, values map[string]AppliedTax) error {
	if r.Goods.Destination == nil {
		return nil
	}
	own, difal := values["ICMS"], values["DIFAL"]
	if own.Base != difal.Base {
		return ErrUnsupported
	}
	intra, err := exactScale(difal.Rate, 4)
	if err != nil {
		return err
	}
	inter, err := exactScale(own.Rate, 2)
	if err != nil {
		return err
	}
	nodes := []TaxNode{taxLeaf("vBCUFDest", difal.Base)}
	fcpBase, fcpRate, fcpAmount := "0.00", "0.0000", "0.00"
	if fcp, ok := values["FCPUFDEST"]; ok {
		if fcp.Base != difal.Base {
			return ErrUnsupported
		}
		fcpBase = fcp.Base
		fcpAmount = fcp.Amount
		fcpRate, err = exactScale(fcp.Rate, 4)
		if err != nil {
			return err
		}
	}
	nodes = append(nodes, taxLeaf("vBCFCPUFDest", fcpBase), taxLeaf("pFCPUFDest", fcpRate), taxLeaf("pICMSUFDest", intra), taxLeaf("pICMSInter", inter), taxLeaf("pICMSInterPart", "100.0000"), taxLeaf("vFCPUFDest", fcpAmount), taxLeaf("vICMSUFDest", difal.Amount), taxLeaf("vICMSUFRemet", "0.00"))
	out.Taxes = append(out.Taxes, taxGroup("ICMSUFDest", nodes...))
	out.Totals["vFCPUFDest"] = fcpAmount
	out.Totals["vICMSUFDest"] = difal.Amount
	out.Totals["vICMSUFRemet"] = "0.00"
	return nil
}
