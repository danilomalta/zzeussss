package fiscalcore

import (
	"math/big"
	"time"
)

// RetainedSTInput records amounts explicitly allocated to this sale line from a
// prior purchase. It is not a new tax charge, a government settlement receipt,
// or an automatic allocation from stock. Approval and source are preserved.
type RetainedSTInput struct {
	SourceKey         string            `json:"source_key"`
	SourceItem        int               `json:"source_item"`
	ApprovalReference string            `json:"approval_reference"`
	Quantity          string            `json:"quantity"`
	Unit              string            `json:"unit"`
	Base              string            `json:"base"`
	SupportedRate     string            `json:"supported_rate"`
	SubstituteAmount  string            `json:"substitute_amount"`
	RetainedAmount    string            `json:"retained_amount"`
	FCP               *RetainedFCPInput `json:"fcp,omitempty"`
}
type RetainedFCPInput struct {
	Base   string `json:"base"`
	Rate   string `json:"rate"`
	Amount string `json:"amount"`
}

func isRetainedST(r TaxRule) bool { return r.Goods != nil && oneOf(r.Goods.ICMS, "60", "500") }
func validateRetainedSTClassification(r TaxRule) error {
	if !isRetainedST(r) {
		if r.Goods != nil && r.Goods.FCPScope == "retained" {
			return ErrRule
		}
		return nil
	}
	// The initial supported case is resale by the substituted taxpayer inside
	// the same UF. Final-consumer effective ICMS and interstate adjustments are
	// separate classifications, never silently inferred from these fields.
	if r.Model != "55" || r.OriginUF != r.DestinationUF || r.CFOP != "5405" || r.Goods.Destination != nil || r.Goods.SimpleCredit != nil || r.Goods.IPIScope != "not_applicable" || !oneOf(r.Goods.FCPScope, "retained", "not_applicable") {
		return ErrUnsupported
	}
	return nil
}
func validateRetainedSTEvidence(r TaxRule, s *RetainedSTInput) error {
	if !isRetainedST(r) || s == nil || !ValidAccessKey(s.SourceKey) || s.SourceKey[20:22] != "55" || s.SourceItem < 1 || s.SourceItem > 990 || !cleanTaxText(s.ApprovalReference, 512) || !cleanTaxText(s.Unit, 6) {
		return ErrRule
	}
	if _, err := time.Parse("0601", s.SourceKey[2:6]); err != nil {
		return ErrRule
	}
	quantity, err := xmlTaxNumber(s.Quantity, 12, 4)
	if err != nil || decimalZero(quantity) {
		return ErrRule
	}
	for _, value := range []string{s.Base, s.SubstituteAmount, s.RetainedAmount} {
		if _, err = xmlTaxNumber(value, 13, 2); err != nil {
			return err
		}
	}
	base, _ := decimal(s.Base)
	own, _ := decimal(s.SubstituteAmount)
	retained, _ := decimal(s.RetainedAmount)
	if base.Sign() <= 0 || own.Cmp(base) > 0 || retained.Cmp(base) > 0 {
		return ErrRule
	}
	if _, err = xmlTaxNumber(s.SupportedRate, 3, 4); err != nil {
		return err
	}
	rate, _ := decimal(s.SupportedRate)
	if rate.Sign() <= 0 || rate.Cmp(big.NewRat(100, 1)) > 0 {
		return ErrRule
	}
	if (s.FCP != nil) != (r.Goods.FCPScope == "retained") {
		return ErrRule
	}
	if s.FCP != nil {
		f := s.FCP
		for _, value := range []string{f.Base, f.Amount} {
			if _, err = xmlTaxNumber(value, 13, 2); err != nil {
				return err
			}
		}
		if _, err = xmlTaxNumber(f.Rate, 3, 4); err != nil {
			return err
		}
		b, _ := decimal(f.Base)
		p, _ := decimal(f.Rate)
		if b.Sign() <= 0 || p.Sign() <= 0 || p.Cmp(rate) > 0 {
			return ErrRule
		}
		amount := new(big.Rat).Quo(new(big.Rat).Mul(b, p), big.NewRat(100, 1))
		if !equalDecimal(money(amount), f.Amount) {
			return ErrRule
		}
	}
	return nil
}
func equalDecimal(a, b string) bool {
	x, e := decimal(a)
	y, f := decimal(b)
	return e == nil && f == nil && x.Cmp(y) == 0
}
func validateRetainedSTItem(c TaxContext, item TaxItem, r TaxRule) error {
	if !isRetainedST(r) {
		if item.RetainedST != nil {
			return ErrRule
		}
		return nil
	}
	if err := validateRetainedSTClassification(r); err != nil {
		return err
	}
	if err := validateRetainedSTEvidence(r, item.RetainedST); err != nil {
		return err
	}
	s := item.RetainedST
	if !equalDecimal(item.Quantity, s.Quantity) || s.SourceKey[2:6] > c.Date[2:4]+c.Date[5:7] {
		return ErrRule
	}
	// Values have already been assigned to the exact quoted line. Do not scale
	// them implicitly after a quantity, unit or price change.
	return nil
}
func appendRetainedST(nodes []TaxNode, out *GoodsTaxMapping, r TaxRule, s *RetainedSTInput) ([]TaxNode, error) {
	if err := validateRetainedSTEvidence(r, s); err != nil {
		return nil, err
	}
	format := func(v string, scale int) string { x, _ := exactScale(v, scale); return x }
	nodes = append(nodes, taxLeaf("vBCSTRet", format(s.Base, 2)), taxLeaf("pST", format(s.SupportedRate, 4)), taxLeaf("vICMSSubstituto", format(s.SubstituteAmount, 2)), taxLeaf("vICMSSTRet", format(s.RetainedAmount, 2)))
	if s.FCP != nil {
		nodes = append(nodes, taxLeaf("vBCFCPSTRet", format(s.FCP.Base, 2)), taxLeaf("pFCPSTRet", format(s.FCP.Rate, 4)), taxLeaf("vFCPSTRet", format(s.FCP.Amount, 2)))
		out.Totals["vFCPSTRet"] = format(s.FCP.Amount, 2)
	}
	return nodes, nil
}
