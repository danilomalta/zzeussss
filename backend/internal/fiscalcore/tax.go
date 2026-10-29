// Package fiscalcore implements deterministic fiscal components. A successful
// calculation, schema validation or signature is not SEFAZ authorization.
package fiscalcore

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInput       = errors.New("invalid fiscal input")
	ErrUnsupported = errors.New("fiscal scope not supported")
	ErrRule        = errors.New("missing, ambiguous or invalid fiscal rule")
	ErrXML         = errors.New("invalid fiscal XML")
	ErrCredential  = errors.New("fiscal credential invalid or unavailable")
	ErrSchema      = errors.New("fiscal schema validation failed")
	ErrUncertain   = errors.New("fiscal transmission outcome uncertain; reconcile original operation")
	decimalRE      = regexp.MustCompile(`^(0|[1-9][0-9]{0,17})(\.[0-9]{1,10})?$`)
	codeRE         = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)
	digitsRE       = regexp.MustCompile(`^[0-9]+$`)
	docRE          = regexp.MustCompile(`^[0-9A-Z]{12}[0-9]{2}$`)
)

func decimal(s string) (*big.Rat, error) {
	if !decimalRE.MatchString(s) {
		return nil, ErrInput
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, ErrInput
	}
	return r, nil
}
func zero() *big.Rat { return new(big.Rat) }
func money(r *big.Rat) string {
	n := new(big.Int).Mul(r.Num(), big.NewInt(100))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(n, r.Denom(), rem)
	if new(big.Int).Lsh(rem, 1).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	s := q.String()
	if len(s) < 3 {
		s = strings.Repeat("0", 3-len(s)) + s
	}
	return s[:len(s)-2] + "." + s[len(s)-2:]
}

type TaxContext struct {
	OriginUF      string `json:"origin_uf"`
	DestinationUF string `json:"destination_uf"`
	CRT           string `json:"crt"`
	Model         string `json:"model"`
	Date          string `json:"date"`
}
type TaxRule struct {
	ID                string         `json:"id"`
	ProductID         string         `json:"product_id"`
	CFOP              string         `json:"cfop"`
	NCM               string         `json:"ncm"`
	OriginUF          string         `json:"origin_uf"`
	DestinationUF     string         `json:"destination_uf"`
	CRT               string         `json:"crt"`
	Model             string         `json:"model"`
	ValidFrom         string         `json:"valid_from"`
	ValidUntil        string         `json:"valid_until"`
	LegalReference    string         `json:"legal_reference"`
	ApprovalReference string         `json:"approval_reference"`
	BaseRounding      string         `json:"base_rounding,omitempty"` // before_tax (legacy) or tax_final
	Components        []TaxComponent `json:"components"`
}
type TaxComponent struct {
	Code             string        `json:"code"`
	Method           string        `json:"method"` // percentage, gross_up, per_unit, exempt
	Rate             string        `json:"rate"`
	ReductionPercent string        `json:"reduction_percent"`
	MVA              percentString `json:"mva_percent"`
	AddApplied       []string      `json:"add_applied"`
	SubtractApplied  []string      `json:"subtract_applied"`
	DeductApplied    []string      `json:"deduct_applied"`
	AddsToTotal      bool          `json:"adds_to_total"`
}

// A string alias keeps JSON monetary/rate inputs exact; never accept float64.
type percentString = string
type TaxItem struct {
	ProductID string `json:"product_id"`
	Quantity  string `json:"quantity"`
	UnitPrice string `json:"unit_price"`
	Discount  string `json:"discount"`
	Freight   string `json:"freight"`
	Insurance string `json:"insurance"`
	Other     string `json:"other"`
}
type AppliedTax struct {
	Code          string `json:"code"`
	Method        string `json:"method"`
	Base          string `json:"base"`
	Rate          string `json:"rate"`
	Amount        string `json:"amount"`
	UnroundedBase string `json:"unrounded_base,omitempty"` // exact rational, not float
}
type TaxResult struct {
	RuleID                string       `json:"rule_id"`
	CFOP                  string       `json:"cfop"`
	NCM                   string       `json:"ncm"`
	Gross                 string       `json:"gross"`
	Net                   string       `json:"net"`
	Total                 string       `json:"total"`
	Taxes                 []AppliedTax `json:"taxes"`
	LegalReference        string       `json:"legal_reference"`
	ApprovalReference     string       `json:"approval_reference"`
	OfficialAuthorization bool         `json:"official_authorization"`
}

var ufCodes = map[string]string{"RO": "11", "AC": "12", "AM": "13", "RR": "14", "PA": "15", "AP": "16", "TO": "17", "MA": "21", "PI": "22", "CE": "23", "RN": "24", "PB": "25", "PE": "26", "AL": "27", "SE": "28", "BA": "29", "MG": "31", "ES": "32", "RJ": "33", "SP": "35", "PR": "41", "SC": "42", "RS": "43", "MS": "50", "MT": "51", "GO": "52", "DF": "53"}

// Calculate applies only an explicitly approved matching policy, with inclusive
// validity dates. No fallback, NCM inference, state-wide rate or unknown-as-zero.
// The trusted caller must load rules from authorized versioned configuration;
// this function does not authenticate the declared approval reference itself.
func Calculate(c TaxContext, item TaxItem, rules []TaxRule) (TaxResult, error) {
	var out TaxResult
	if _, ok := ufCodes[c.OriginUF]; !ok {
		return out, ErrInput
	}
	if _, ok := ufCodes[c.DestinationUF]; !ok {
		return out, ErrInput
	}
	if c.CRT != "1" && c.CRT != "2" && c.CRT != "3" && c.CRT != "4" {
		return out, ErrInput
	}
	if c.Model != "55" && c.Model != "65" && c.Model != "nfse" {
		return out, ErrInput
	}
	date, err := time.Parse("2006-01-02", c.Date)
	if err != nil || item.ProductID == "" {
		return out, ErrInput
	}
	var selected *TaxRule
	for i := range rules {
		r := &rules[i]
		if r.ProductID != item.ProductID || r.OriginUF != c.OriginUF || r.DestinationUF != c.DestinationUF || r.CRT != c.CRT || r.Model != c.Model {
			continue
		}
		from, e1 := time.Parse("2006-01-02", r.ValidFrom)
		until, e2 := time.Parse("2006-01-02", r.ValidUntil)
		if e1 != nil || e2 != nil || until.Before(from) {
			return out, ErrRule
		}
		if date.Before(from) || date.After(until) {
			continue
		}
		if selected != nil {
			return out, ErrRule
		}
		selected = r
	}
	if selected == nil {
		return out, ErrRule
	}
	r := selected
	if r.BaseRounding != "" && r.BaseRounding != "before_tax" && r.BaseRounding != "tax_final" {
		return out, ErrRule
	}
	if r.ID == "" || len(r.ID) > 128 || r.LegalReference == "" || r.ApprovalReference == "" || len(r.Components) == 0 || len(r.Components) > 16 {
		return out, ErrRule
	}
	if c.Model != "nfse" && (len(r.CFOP) != 4 || !digitsRE.MatchString(r.CFOP) || len(r.NCM) != 8 || !digitsRE.MatchString(r.NCM)) {
		return out, ErrRule
	}
	quantity, err := decimal(item.Quantity)
	if err != nil || quantity.Sign() <= 0 {
		return out, ErrInput
	}
	unit, err := decimal(item.UnitPrice)
	if err != nil {
		return out, err
	}
	grossRaw := new(big.Rat).Mul(quantity, unit)
	gross, err := decimal(money(grossRaw))
	if err != nil {
		return out, ErrInput
	}
	discount, err := decimal(item.Discount)
	if err != nil || discount.Cmp(gross) > 0 {
		return out, ErrInput
	}
	net := new(big.Rat).Sub(gross, discount)
	for _, s := range []string{item.Freight, item.Insurance, item.Other} {
		a, e := decimal(s)
		if e != nil {
			return out, e
		}
		net.Add(net, a)
	}
	// The monetary operation base is rounded before percentage calculations.
	net, err = decimal(money(net))
	if err != nil {
		return out, ErrInput
	}
	total := new(big.Rat).Set(net)
	applied := map[string]*big.Rat{}
	out = TaxResult{RuleID: r.ID, CFOP: r.CFOP, NCM: r.NCM, Gross: money(gross), Net: money(net), LegalReference: r.LegalReference, ApprovalReference: r.ApprovalReference, Taxes: []AppliedTax{}}
	for _, t := range r.Components {
		if !codeRE.MatchString(t.Code) || applied[t.Code] != nil {
			return TaxResult{}, ErrRule
		}
		rate, e := decimal(t.Rate)
		if e != nil {
			return TaxResult{}, ErrRule
		}
		reduction, e := decimal(t.ReductionPercent)
		if e != nil || reduction.Cmp(big.NewRat(100, 1)) > 0 {
			return TaxResult{}, ErrRule
		}
		mva, e := decimal(t.MVA)
		if e != nil || mva.Cmp(big.NewRat(1000, 1)) > 0 {
			return TaxResult{}, ErrRule
		}
		base := new(big.Rat).Set(net)
		for _, entry := range []struct {
			codes    []string
			subtract bool
		}{{t.AddApplied, false}, {t.SubtractApplied, true}} {
			seen := map[string]bool{}
			for _, code := range entry.codes {
				a, ok := applied[code]
				if !ok || seen[code] {
					return TaxResult{}, ErrRule
				}
				seen[code] = true
				if entry.subtract {
					base.Sub(base, a)
				} else {
					base.Add(base, a)
				}
			}
		}
		base.Mul(base, new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Quo(mva, big.NewRat(100, 1))))
		base.Mul(base, new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(reduction, big.NewRat(100, 1))))
		if base.Sign() < 0 {
			return TaxResult{}, ErrRule
		}
		if r.BaseRounding != "tax_final" {
			base, err = decimal(money(base))
			if err != nil {
				return TaxResult{}, ErrInput
			}
		}
		amount := zero()
		switch t.Method {
		case "percentage":
			if rate.Cmp(big.NewRat(100, 1)) > 0 {
				return TaxResult{}, ErrRule
			}
			amount.Mul(base, new(big.Rat).Quo(rate, big.NewRat(100, 1)))
		case "gross_up":
			if rate.Cmp(big.NewRat(100, 1)) >= 0 {
				return TaxResult{}, ErrRule
			}
			base.Quo(base, new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(rate, big.NewRat(100, 1))))
			if r.BaseRounding != "tax_final" {
				base, err = decimal(money(base))
				if err != nil {
					return TaxResult{}, ErrInput
				}
			}
			amount.Mul(base, new(big.Rat).Quo(rate, big.NewRat(100, 1)))
		case "per_unit":
			if len(t.AddApplied) > 0 || len(t.SubtractApplied) > 0 || mva.Sign() != 0 || reduction.Sign() != 0 {
				return TaxResult{}, ErrRule
			}
			base = new(big.Rat).Set(quantity)
			amount.Mul(quantity, rate)
		case "exempt":
			if rate.Sign() != 0 || len(t.DeductApplied) > 0 || t.AddsToTotal {
				return TaxResult{}, ErrRule
			}
		default:
			return TaxResult{}, ErrUnsupported
		}
		amount, err = decimal(money(amount))
		if err != nil {
			return TaxResult{}, ErrInput
		}
		seen := map[string]bool{}
		for _, code := range t.DeductApplied {
			a, ok := applied[code]
			if !ok || seen[code] {
				return TaxResult{}, ErrRule
			}
			seen[code] = true
			amount.Sub(amount, a)
		}
		if amount.Sign() < 0 {
			return TaxResult{}, ErrRule
		}
		if t.AddsToTotal {
			total.Add(total, amount)
		}
		applied[t.Code] = amount
		baseValue := money(base)
		if t.Method == "per_unit" {
			baseValue = item.Quantity
		}
		value := AppliedTax{Code: t.Code, Method: t.Method, Base: baseValue, Rate: t.Rate, Amount: money(amount)}
		if r.BaseRounding == "tax_final" {
			value.UnroundedBase = base.RatString()
		}
		out.Taxes = append(out.Taxes, value)
	}
	out.Total = money(total)
	if len(out.Total) > 18 {
		return TaxResult{}, fmt.Errorf("%w: total limit", ErrInput)
	}
	return out, nil
}
