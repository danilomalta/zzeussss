package fiscalcore

import (
	"github.com/beevik/etree"
	"math/big"
)

// reconcileTaxTotals checks supported legacy tax sums, not the legal tax
// classification. Unsupported groups fail explicitly instead of disappearing
// from the total (notably RTC/IBS/CBS, ISSQN and destination sharing).
func reconcileTaxTotals(info *etree.Element) error {
	sums := map[string]*big.Rat{}
	for _, name := range []string{"vBC", "vICMS", "vICMSDeson", "vFCP", "vBCST", "vST", "vFCPST", "vFCPSTRet", "vII", "vIPI", "vIPIDevol", "vPIS", "vCOFINS"} {
		sums[name] = zero()
	}
	mappings := map[string]map[string]string{
		"ICMS": {"vBC": "vBC", "vICMS": "vICMS", "vICMSDeson": "vICMSDeson", "vFCP": "vFCP", "vBCST": "vBCST", "vICMSST": "vST", "vFCPST": "vFCPST", "vFCPSTRet": "vFCPSTRet"},
		"II":   {"vII": "vII"}, "IPI": {"vIPI": "vIPI"}, "PIS": {"vPIS": "vPIS"}, "COFINS": {"vCOFINS": "vCOFINS"},
	}
	for _, det := range info.SelectElements("det") {
		imposto := det.SelectElement("imposto")
		if imposto == nil {
			return ErrRule
		}
		for _, group := range imposto.ChildElements() {
			if group.Tag == "vTotTrib" {
				continue
			}
			mapping, ok := mappings[group.Tag]
			if !ok {
				return ErrUnsupported
			}
			var walk func(*etree.Element) error
			walk = func(e *etree.Element) error {
				if dest, ok := mapping[e.Tag]; ok {
					v, err := decimal(e.Text())
					if err != nil {
						return ErrInput
					}
					sums[dest].Add(sums[dest], v)
				}
				for _, child := range e.ChildElements() {
					if err := walk(child); err != nil {
						return err
					}
				}
				return nil
			}
			if err := walk(group); err != nil {
				return err
			}
		}
		if det.SelectElement("impostoDevol") != nil {
			return ErrUnsupported
		}
	}
	total := info.FindElement("total/ICMSTot")
	if total == nil {
		return ErrRule
	}
	for name, sum := range sums {
		e := total.SelectElement(name)
		if e == nil {
			return ErrRule
		}
		v, err := decimal(e.Text())
		if err != nil || v.Cmp(sum) != 0 {
			return ErrRule
		}
	}
	return nil
}
