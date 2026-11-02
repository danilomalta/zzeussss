package fiscal

import "titansystem-backend/internal/fiscalcore"

// Customer-supplied accounting evidence, never automatic legal approval.
type TaxReview struct {
	Regime              string `json:"regime"`
	TableRevision       string `json:"table_revision"`
	AccountantApproved  bool   `json:"accountant_approved"`
	AccountantReference string `json:"accountant_reference,omitempty"`
}

func (r *TaxReview) valid(crt string) bool {
	if r == nil {
		return true
	}
	if !idPattern.MatchString(r.TableRevision) || len(r.AccountantReference) > 512 {
		return false
	}
	if r.AccountantApproved {
		if r.AccountantReference == "" {
			return false
		}
		for _, c := range r.AccountantReference {
			if c < ' ' || c == 127 {
				return false
			}
		}
	}
	switch r.Regime {
	case "simples_nacional":
		return crt == "1" || crt == "2" || crt == "4"
	case "lucro_presumido", "lucro_real":
		return crt == "3"
	}
	return false
}
func (in TaxPolicyInput) AccountantApproved() bool {
	return in.Review != nil && in.Review.AccountantApproved && in.Review.valid(in.Rule.CRT)
}

type TaxTable struct {
	Version  int              `json:"version"`
	Revision string           `json:"revision"`
	Regime   string           `json:"regime"`
	Policies []TaxPolicyInput `json:"policies"`
}

// Validates the entire versioned file before publication; never fetches sources
// or invents rates, classifications, legal applicability or accountant approval.
func DecodeTaxTable(raw []byte) (TaxTable, error) {
	var t TaxTable
	if len(raw) > 1024*1024 || fiscalcore.DecodeInput(raw, &t) != nil || t.Version != 1 || !idPattern.MatchString(t.Revision) || len(t.Policies) == 0 || len(t.Policies) > 100 {
		return TaxTable{}, ErrInvalid
	}
	ids, ops := map[string]bool{}, map[string]bool{}
	for i := range t.Policies {
		p := &t.Policies[i]
		if p.Review == nil {
			p.Review = &TaxReview{Regime: t.Regime, TableRevision: t.Revision}
		}
		if p.Review.Regime != t.Regime || p.Review.TableRevision != t.Revision || !p.valid() || ids[p.Rule.ID] || ops[p.OperationID] {
			return TaxTable{}, ErrInvalid
		}
		ids[p.Rule.ID] = true
		ops[p.OperationID] = true
	}
	return t, nil
}
