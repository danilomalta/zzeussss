package fiscalcore

import (
	"math/big"
	"strings"
)

// Classification is explicitly approved per product/operation. Codes are not
// guessed from an NCM or UF. Optional IPI means outside this invoice's IPI scope,
// not a fabricated zero-rate IPI group.
type GoodsTaxClassification struct {
	SimpleCredit          *SimpleCreditInput            `json:"simple_credit,omitempty"`
	Destination           *DestinationTaxClassification `json:"destination,omitempty"`
	Origin                string                        `json:"origin"`
	ICMS                  string                        `json:"icms"`
	PIS                   string                        `json:"pis"`
	COFINS                string                        `json:"cofins"`
	IPIScope              string                        `json:"ipi_scope"`
	FCPScope              string                        `json:"fcp_scope"`
	BenefitCode           string                        `json:"benefit_code,omitempty"`
	InterstateBasis       string                        `json:"interstate_basis,omitempty"`
	FCI                   string                        `json:"fci,omitempty"`
	FCIExemptionReference string                        `json:"fci_exemption_reference,omitempty"`
	IPI                   string                        `json:"ipi,omitempty"`
	IPIEnquadramento      string                        `json:"ipi_enquadramento,omitempty"`
}
type GoodsTaxMapping struct {
	SimpleCredit        *SimpleCreditResult      `json:"simple_credit,omitempty"`
	Version             int                      `json:"version"`
	Classification      GoodsTaxClassification   `json:"classification"`
	Taxes               []TaxNode                `json:"taxes"`
	Totals              map[string]string        `json:"totals"`
	InterstateReference *InterstateRateSelection `json:"interstate_reference,omitempty"`
}

func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func exactScale(s string, scale int) (string, error) {
	r, err := decimal(s)
	if err != nil {
		return "", ErrRule
	}
	formatted := r.FloatString(scale)
	parsed, ok := new(big.Rat).SetString(formatted)
	if !ok || parsed.Cmp(r) != 0 {
		return "", ErrUnsupported
	}
	return formatted, nil
}
func xmlTaxNumber(s string, integer, scale int) (string, error) {
	v, err := exactScale(s, scale)
	if err != nil {
		return "", err
	}
	if len(strings.SplitN(v, ".", 2)[0]) > integer {
		return "", ErrUnsupported
	}
	return v, nil
}
func taxLeaf(name, value string) TaxNode { return TaxNode{Name: name, Value: value} }
func taxGroup(name string, children ...TaxNode) TaxNode {
	return TaxNode{Name: name, Children: children}
}

// ValidateGoodsTaxClassification validates the supported XML/arithmetical
// combination, not a tax authority's approval of incidence or benefits.
func ValidateGoodsTaxClassification(r TaxRule) error {
	g := r.Goods
	if g == nil {
		return ErrUnsupported
	}
	if !oneOf(r.Model, "55", "65") || len(r.CFOP) != 4 || !digitsRE.MatchString(r.CFOP) {
		return ErrRule
	}
	if r.OriginUF == r.DestinationUF {
		if r.CFOP[0] != '5' {
			return ErrRule
		}
	} else {
		if r.Model != "55" || r.CFOP[0] != '6' {
			return ErrUnsupported
		}
	}
	if len(g.Origin) != 1 || g.Origin[0] < '0' || g.Origin[0] > '8' {
		return ErrRule
	}
	if !oneOf(g.IPIScope, "classified", "not_applicable") || !oneOf(g.FCPScope, "not_applicable", "own", "substitution", "own_and_substitution", "retained") {
		return ErrRule
	}
	if g.BenefitCode != "" && (len(g.BenefitCode) < 8 || !cleanTaxText(g.BenefitCode, 10)) {
		return ErrRule
	}
	simple := r.CRT == "1"
	if g.FCI != "" && !fciRE.MatchString(g.FCI) {
		return ErrRule
	}
	if g.FCIExemptionReference != "" && !cleanTaxText(g.FCIExemptionReference, 512) {
		return ErrRule
	}
	if g.FCI != "" && g.FCIExemptionReference != "" {
		return ErrRule
	}
	if g.InterstateBasis != "" && oneOf(g.Origin, "3", "5", "8") && g.FCI == "" && g.FCIExemptionReference == "" {
		return ErrRule
	}
	if simple {
		if !oneOf(g.ICMS, "101", "102", "103", "201", "202", "203", "300", "400", "500") {
			return ErrUnsupported
		}
	} else if (r.CRT != "2" && r.CRT != "3") || !oneOf(g.ICMS, "00", "10", "20", "40", "41", "50", "51", "60", "70") {
		return ErrUnsupported
	}
	components := map[string]TaxComponent{}
	for _, t := range r.Components {
		if !oneOf(t.Code, "ICMS", "ICMSST", "FCP", "FCPST", "PIS", "COFINS", "IPI", "DIFAL", "FCPUFDEST") || components[t.Code].Code != "" {
			return ErrUnsupported
		}
		if _, err := exactScale(t.Rate, 4); err != nil {
			return err
		}
		if _, err := exactScale(t.ReductionPercent, 4); err != nil {
			return err
		}
		if _, err := exactScale(t.MVA, 4); err != nil {
			return err
		}
		if _, err := xmlTaxNumber(t.MVA, 3, 4); err != nil {
			return err
		}
		if t.Method == "per_unit" {
			if _, err := xmlTaxNumber(t.Rate, 11, 4); err != nil {
				return err
			}
		}
		components[t.Code] = t
	}
	icms, ok := components["ICMS"]
	if !ok || icms.AddsToTotal || len(icms.DeductApplied) != 0 || !decimalZero(icms.MVA) {
		return ErrRule
	}
	exempt := simple || oneOf(g.ICMS, "40", "41", "50", "60")
	if exempt {
		if !pureExempt(icms) {
			return ErrRule
		}
	} else {
		if icms.Method != "percentage" {
			return ErrUnsupported
		}
		reduced := !decimalZero(icms.ReductionPercent)
		if g.ICMS != "51" && reduced != oneOf(g.ICMS, "20", "70") {
			return ErrRule
		}
	}
	st, hasST := components["ICMSST"]
	if hasST != oneOf(g.ICMS, "10", "70", "201", "202", "203") {
		return ErrRule
	}
	if hasST && (st.Method != "percentage" || !st.AddsToTotal || (!simple && (len(st.DeductApplied) != 1 || st.DeductApplied[0] != "ICMS")) || (simple && (len(st.DeductApplied) != 0 || st.OwnOperationDeduction == nil))) {
		return ErrRule
	}
	for _, code := range []string{"FCP", "FCPST"} {
		t, ok := components[code]
		if !ok {
			continue
		}
		if exempt && code != "FCPST" || t.Method != "percentage" || code == "FCP" && !decimalZero(t.MVA) || len(t.DeductApplied) != 0 || t.AddsToTotal != (code == "FCPST") || code == "FCPST" && !hasST {
			return ErrRule
		}
	}
	_, ownFCP := components["FCP"]
	_, stFCP := components["FCPST"]
	if ownFCP != oneOf(g.FCPScope, "own", "own_and_substitution") || stFCP != oneOf(g.FCPScope, "substitution", "own_and_substitution") {
		return ErrRule
	}
	for _, p := range []struct{ code, cst string }{{"PIS", g.PIS}, {"COFINS", g.COFINS}} {
		t, ok := components[p.code]
		if !ok || t.AddsToTotal || !decimalZero(t.MVA) || !decimalZero(t.ReductionPercent) || len(t.DeductApplied) != 0 {
			return ErrRule
		}
		switch {
		case oneOf(p.cst, "01", "02"):
			if t.Method != "percentage" {
				return ErrRule
			}
		case p.cst == "03":
			if t.Method != "per_unit" {
				return ErrRule
			}
		case oneOf(p.cst, "04", "05", "06", "07", "08", "09"):
			if !pureExempt(t) {
				return ErrRule
			}
		case oneOf(p.cst, "49", "99"):
			if !oneOf(t.Method, "percentage", "per_unit", "exempt") {
				return ErrRule
			}
		default:
			return ErrUnsupported
		}
	}
	ipi, hasIPI := components["IPI"]
	if hasIPI != (g.IPI != "") || hasIPI != (g.IPIScope == "classified") {
		return ErrRule
	}
	if hasIPI {
		if len(g.IPIEnquadramento) != 3 || !digitsRE.MatchString(g.IPIEnquadramento) || !decimalZero(ipi.MVA) || !decimalZero(ipi.ReductionPercent) || len(ipi.DeductApplied) != 0 {
			return ErrRule
		}
		if oneOf(g.IPI, "50", "99") {
			if !oneOf(ipi.Method, "percentage", "per_unit") || !ipi.AddsToTotal {
				return ErrRule
			}
		} else if oneOf(g.IPI, "51", "52", "53", "54", "55") {
			if !pureExempt(ipi) {
				return ErrRule
			}
		} else {
			return ErrUnsupported
		}
	} else if g.IPIEnquadramento != "" {
		return ErrRule
	}
	if err := validateRetainedSTClassification(r); err != nil {
		return err
	}
	if err := validateDestinationTax(r, components); err != nil {
		return err
	}
	if err := validateSTOwnDeduction(r); err != nil {
		return err
	}
	if err := validateSimpleCredit(r); err != nil {
		return err
	}
	if err := validateICMSDeferment(r); err != nil {
		return err
	}
	if err := validateAdjustedMVA(r); err != nil {
		return err
	}
	_, err := validateInterstatePolicy(r)
	return err
}
func decimalZero(s string) bool { r, e := decimal(s); return e == nil && r.Sign() == 0 }
func pureExempt(t TaxComponent) bool {
	return t.Method == "exempt" && decimalZero(t.Rate) && decimalZero(t.ReductionPercent) && decimalZero(t.MVA) && len(t.AddApplied)+len(t.SubtractApplied)+len(t.DeductApplied) == 0 && !t.AddsToTotal
}

// MapGoodsTaxes produces only supported official XML groups from a frozen exact
// calculation. Unknown components never disappear from the document or totals.
func MapGoodsTaxes(r TaxRule, result TaxResult) (*GoodsTaxMapping, error) {
	if err := ValidateGoodsTaxClassification(r); err != nil {
		return nil, err
	}
	if result.RuleID != r.ID || result.CFOP != r.CFOP || result.NCM != r.NCM || len(result.Taxes) != len(r.Components) {
		return nil, ErrRule
	}
	g := *r.Goods
	values := map[string]AppliedTax{}
	components := map[string]TaxComponent{}
	for i, t := range r.Components {
		a := result.Taxes[i]
		if a.Code != t.Code || a.Method != t.Method || a.Rate != t.Rate || (a.OwnOperationDeduction == nil) != (t.OwnOperationDeduction == nil) {
			return nil, ErrRule
		}
		if _, err := xmlTaxNumber(a.Amount, 13, 2); err != nil {
			return nil, err
		}
		base, err := decimal(a.Base)
		if err != nil {
			return nil, err
		}
		rateValue, err := decimal(a.Rate)
		if err != nil {
			return nil, err
		}
		if a.Method == "per_unit" {
			if _, err = xmlTaxNumber(a.Base, 12, 4); err != nil {
				return nil, err
			}
		} else {
			if _, err = xmlTaxNumber(a.Base, 13, 2); err != nil {
				return nil, err
			}
		}
		// Legacy NF-e groups expose a cent-rounded base. Do not silently emit
		// a different tax when tax_final depended on a non-representable base.
		if a.Method != "exempt" {
			calculated := new(big.Rat).Mul(base, rateValue)
			if a.Method != "per_unit" {
				calculated.Quo(calculated, big.NewRat(100, 1))
			}
			calculated, _ = decimal(money(calculated))
			if calculated == nil {
				return nil, ErrUnsupported
			}
			if t.OwnOperationDeduction != nil {
				expected, e := CalculateSTOwnDeduction(result.Net, *t.OwnOperationDeduction)
				if e != nil || a.OwnOperationDeduction == nil || *a.OwnOperationDeduction != expected {
					return nil, ErrRule
				}
				deduction, _ := decimal(expected.Amount)
				calculated.Sub(calculated, deduction)
			}
			if t.Deferment != nil {
				expected, e := CalculateICMSDeferment(money(calculated), *t.Deferment)
				if e != nil || a.Deferment == nil || *a.Deferment != expected {
					return nil, ErrRule
				}
				calculated, _ = decimal(expected.DueAmount)
			} else if a.Deferment != nil {
				return nil, ErrRule
			}
			for _, code := range t.DeductApplied {
				previous, ok := values[code]
				if !ok {
					return nil, ErrRule
				}
				deduction, e := decimal(previous.Amount)
				if e != nil {
					return nil, e
				}
				calculated.Sub(calculated, deduction)
			}
			if calculated.Sign() < 0 || money(calculated) != a.Amount {
				return nil, ErrUnsupported
			}
		}
		values[t.Code] = a
		components[t.Code] = t
	}
	out := &GoodsTaxMapping{Version: 1, Classification: g, Taxes: []TaxNode{}, Totals: map[string]string{}}
	out.InterstateReference, _ = validateInterstatePolicy(r)
	for _, k := range []string{"vBC", "vICMS", "vICMSDeson", "vFCP", "vBCST", "vST", "vFCPST", "vFCPSTRet", "vII", "vIPI", "vIPIDevol", "vPIS", "vCOFINS"} {
		out.Totals[k] = "0.00"
	}
	rate := func(code string) string { v, _ := exactScale(values[code].Rate, 4); return v }
	icms := values["ICMS"]
	own := []TaxNode{taxLeaf("orig", g.Origin)}
	tag := "ICMS" + g.ICMS
	if len(g.ICMS) == 3 {
		tag = "ICMSSN102"
		own = append(own, taxLeaf("CSOSN", g.ICMS))
		if oneOf(g.ICMS, "201", "202", "203") {
			tag = "ICMSSN202"
			var err error
			own, err = appendGoodsST(own, out, values, components, true)
			if err != nil {
				return nil, err
			}
		}
		if oneOf(g.ICMS, "101", "201") {
			credit, err := mapSimpleCredit(r, result.Net)
			if err != nil {
				return nil, err
			}
			out.SimpleCredit = credit
			tag = "ICMSSN" + g.ICMS
			percent, _ := exactScale(credit.Percent, 4)
			own = append(own, taxLeaf("pCredSN", percent), taxLeaf("vCredICMSSN", credit.Amount))
		}
	} else {
		own = append(own, taxLeaf("CST", g.ICMS))
		if g.ICMS == "60" {
			// Prior retention is mapped below; no new own-operation ICMS.
		} else if oneOf(g.ICMS, "40", "41", "50") {
			tag = "ICMS40"
		} else {
			own = append(own, taxLeaf("modBC", "3"))
			if oneOf(g.ICMS, "20", "51", "70") {
				v, _ := exactScale(components["ICMS"].ReductionPercent, 4)
				own = append(own, taxLeaf("pRedBC", v))
			}
			own = append(own, taxLeaf("vBC", icms.Base), taxLeaf("pICMS", rate("ICMS")))
			if icms.Deferment != nil {
				p, _ := exactScale(icms.Deferment.Percent, 4)
				own = append(own, taxLeaf("vICMSOp", icms.Deferment.OperationAmount), taxLeaf("pDif", p), taxLeaf("vICMSDif", icms.Deferment.DeferredAmount))
			}
			own = append(own, taxLeaf("vICMS", icms.Amount))
			out.Totals["vBC"] = icms.Base
			out.Totals["vICMS"] = icms.Amount
			if fcp, ok := values["FCP"]; ok {
				if g.ICMS == "00" {
					if fcp.Base != icms.Base {
						return nil, ErrUnsupported
					}
				} else {
					own = append(own, taxLeaf("vBCFCP", fcp.Base))
				}
				own = append(own, taxLeaf("pFCP", rate("FCP")), taxLeaf("vFCP", fcp.Amount))
				out.Totals["vFCP"] = fcp.Amount
			}
			var err error
			own, err = appendGoodsST(own, out, values, components, false)
			if err != nil {
				return nil, err
			}
		}
	}
	if isRetainedST(r) {
		var err error
		own, err = appendRetainedST(own, out, r, result.RetainedST)
		if err != nil {
			return nil, err
		}
		tag = "ICMS60"
		if g.ICMS == "500" {
			tag = "ICMSSN500"
		}
	} else if result.RetainedST != nil {
		return nil, ErrRule
	}
	out.Taxes = append(out.Taxes, taxGroup("ICMS", taxGroup(tag, own...)))
	if g.IPI != "" {
		a := values["IPI"]
		nodes := []TaxNode{taxLeaf("CST", g.IPI)}
		tag := "IPINT"
		if oneOf(g.IPI, "50", "99") {
			tag = "IPITrib"
			if a.Method == "per_unit" {
				q, err := exactScale(a.Base, 4)
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, taxLeaf("qUnid", q), taxLeaf("vUnid", rate("IPI")))
			} else {
				nodes = append(nodes, taxLeaf("vBC", a.Base), taxLeaf("pIPI", rate("IPI")))
			}
			nodes = append(nodes, taxLeaf("vIPI", a.Amount))
			out.Totals["vIPI"] = a.Amount
		}
		out.Taxes = append(out.Taxes, taxGroup("IPI", taxLeaf("cEnq", g.IPIEnquadramento), taxGroup(tag, nodes...)))
	}
	for _, p := range []struct{ code, cst string }{{"PIS", g.PIS}, {"COFINS", g.COFINS}} {
		a := values[p.code]
		tag := p.code + "NT"
		nodes := []TaxNode{taxLeaf("CST", p.cst)}
		if !oneOf(p.cst, "04", "05", "06", "07", "08", "09") {
			if oneOf(p.cst, "49", "99") {
				tag = p.code + "Outr"
			} else if a.Method == "per_unit" {
				tag = p.code + "Qtde"
			} else {
				tag = p.code + "Aliq"
			}
			if a.Method == "per_unit" {
				q, err := exactScale(a.Base, 4)
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, taxLeaf("qBCProd", q), taxLeaf("vAliqProd", rate(p.code)))
			} else {
				nodes = append(nodes, taxLeaf("vBC", a.Base), taxLeaf("p"+p.code, rate(p.code)))
			}
			nodes = append(nodes, taxLeaf("v"+p.code, a.Amount))
			out.Totals["v"+p.code] = a.Amount
		}
		out.Taxes = append(out.Taxes, taxGroup(p.code, taxGroup(tag, nodes...)))
	}
	if err := appendDestinationTax(out, r, values); err != nil {
		return nil, err
	}
	// Tax values written to a two-decimal field cannot conceal additional cents.
	for _, t := range result.Taxes {
		if !strings.HasPrefix(t.Code, "ICMS") && t.Code != "PIS" && t.Code != "COFINS" && t.Code != "IPI" && t.Code != "FCP" && t.Code != "FCPST" && t.Code != "DIFAL" && t.Code != "FCPUFDEST" {
			return nil, ErrUnsupported
		}
	}
	return out, nil
}
