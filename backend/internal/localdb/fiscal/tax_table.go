package fiscal

import (
	"context"
	"database/sql"
	"encoding/json"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitewrite"
)

// Customer-supplied accounting evidence, never automatic legal approval.
type TaxReview struct {
	FileSHA256          string `json:"file_sha256,omitempty"`
	Regime              string `json:"regime"`
	TableRevision       string `json:"table_revision"`
	AccountantApproved  bool   `json:"accountant_approved"`
	AccountantReference string `json:"accountant_reference,omitempty"`
}

func (r *TaxReview) valid(crt string) bool {
	if r == nil {
		return true
	}
	if r.FileSHA256 != "" && !validTableHash(r.FileSHA256) {
		return false
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
	var presence struct {
		Policies []struct {
			Active *bool `json:"active"`
			Rule   struct {
				Components []struct {
					Adds *bool `json:"adds_to_total"`
				} `json:"components"`
			} `json:"rule"`
		} `json:"policies"`
	}
	if json.Unmarshal(raw, &presence) != nil {
		return TaxTable{}, ErrInvalid
	}
	for _, p := range presence.Policies {
		if p.Active == nil {
			return TaxTable{}, ErrInvalid
		}
		for _, c := range p.Rule.Components {
			if c.Adds == nil {
				return TaxTable{}, ErrInvalid
			}
		}
	}
	ids, ops := map[string]bool{}, map[string]bool{}
	for i := range t.Policies {
		p := &t.Policies[i]
		if p.Review == nil {
			p.Review = &TaxReview{Regime: t.Regime, TableRevision: t.Revision}
		}
		if p.Review.AccountantApproved || p.Review.AccountantReference != "" {
			return TaxTable{}, ErrInvalid
		}
		p.Review.FileSHA256 = taxHash(raw)
		if p.Review.Regime != t.Regime || p.Review.TableRevision != t.Revision || !p.valid() || ids[p.Rule.ID] || ops[p.OperationID] {
			return TaxTable{}, ErrInvalid
		}
		ids[p.Rule.ID] = true
		ops[p.OperationID] = true
	}
	return t, nil
}

// All rows publish in one immediate transaction: a failed row rolls back the
// entire file, and concurrent stale revisions conflict without mixing tables.
func ImportTaxTable(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, raw []byte, token string) ([]TaxPolicy, error) {
	table, err := DecodeTaxTable(raw)
	if err != nil {
		return nil, err
	}
	var out []TaxPolicy
	err = sqlitewrite.Run(ctx, db, ErrConflict, func(tx *sql.Tx) error {
		out = nil
		for _, in := range table.Policies {
			saved, e := publishTaxPolicyTx(ctx, tx, license, a, d, in, token)
			if e != nil {
				return e
			}
			out = append(out, saved)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func validTableHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
