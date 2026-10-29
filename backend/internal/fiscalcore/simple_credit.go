package fiscalcore

import (
	"math/big"
	"time"
)

// This input records an approved effective ICMS credit percentage, not a
// guessed DAS bracket. A new policy is required each operation month.
type SimpleCreditInput struct {
	Percent         string `json:"percent"`
	RevenuePeriod   string `json:"revenue_period"`
	RecipientRegime string `json:"recipient_regime"`
	BaseMethod      string `json:"base_method"`
	LegalReference  string `json:"legal_reference"`
}
type SimpleCreditResult struct {
	Base          string `json:"base"`
	Percent       string `json:"percent"`
	Amount        string `json:"amount"`
	RevenuePeriod string `json:"revenue_period"`
}

func validateSimpleCredit(r TaxRule) error {
	if r.Goods == nil {
		return nil
	}
	g := r.Goods
	if g.SimpleCredit == nil {
		if g.ICMS == "101" {
			return ErrRule
		}
		return nil
	}
	c := g.SimpleCredit
	if g.ICMS != "101" || r.CRT != "1" || r.Model != "55" || g.FCPScope != "not_applicable" || g.Destination != nil || c.BaseMethod != "approved_net_operation" || c.RecipientRegime != "normal" || !cleanTaxText(c.LegalReference, 2000) {
		return ErrUnsupported
	}
	percent, e := decimal(c.Percent)
	if e != nil || percent.Sign() <= 0 || percent.Cmp(big.NewRat(100, 1)) > 0 {
		return ErrRule
	}
	if _, e = exactScale(c.Percent, 4); e != nil {
		return e
	}
	from, e1 := time.Parse("2006-01-02", r.ValidFrom)
	until, e2 := time.Parse("2006-01-02", r.ValidUntil)
	period, e3 := time.Parse("2006-01", c.RevenuePeriod)
	if e1 != nil || e2 != nil || e3 != nil || until.Before(from) || from.Format("2006-01") != until.Format("2006-01") {
		return ErrRule
	}
	previous := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	if !period.Equal(previous) {
		return ErrRule
	}
	return nil
}
func mapSimpleCredit(r TaxRule, base string) (*SimpleCreditResult, error) {
	if err := validateSimpleCredit(r); err != nil {
		return nil, err
	}
	if r.Goods.SimpleCredit == nil {
		return nil, nil
	}
	c := r.Goods.SimpleCredit
	amount, e := decimal(base)
	if e != nil {
		return nil, e
	}
	if _, e = xmlTaxNumber(base, 13, 2); e != nil {
		return nil, e
	}
	percent, _ := decimal(c.Percent)
	value := money(new(big.Rat).Quo(new(big.Rat).Mul(amount, percent), big.NewRat(100, 1)))
	if _, e = xmlTaxNumber(value, 13, 2); e != nil {
		return nil, e
	}
	return &SimpleCreditResult{Base: base, Percent: c.Percent, Amount: value, RevenuePeriod: c.RevenuePeriod}, nil
}
