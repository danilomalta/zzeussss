package stocktaking

import (
	"context"
	"database/sql"
	"math/big"
	"titansystem-backend/internal/localdb/identity"
)

type VarianceLine struct {
	Line
	CostCents             int64   `json:"current_catalog_cost_cents"`
	CostRequiresReview    bool    `json:"cost_requires_review"`
	DifferenceNumerator   *string `json:"difference_value_cents_numerator"`
	DifferenceDenominator *string `json:"difference_value_cents_denominator"`
}
type VarianceReport struct {
	Session             Session        `json:"inventory"`
	Basis               string         `json:"basis"`
	Lines               []VarianceLine `json:"lines"`
	CountedLines        int            `json:"counted_lines"`
	UncountedLines      int            `json:"uncounted_lines"`
	ShortageLines       int            `json:"shortage_lines"`
	SurplusLines        int            `json:"surplus_lines"`
	EqualLines          int            `json:"equal_lines"`
	ShortageNumerator   string         `json:"shortage_value_cents_numerator"`
	ShortageDenominator string         `json:"shortage_value_cents_denominator"`
	SurplusNumerator    string         `json:"surplus_value_cents_numerator"`
	SurplusDenominator  string         `json:"surplus_value_cents_denominator"`
	NetNumerator        string         `json:"net_value_cents_numerator"`
	NetDenominator      string         `json:"net_value_cents_denominator"`
	PostedToFinance     bool           `json:"posted_to_finance"`
}

// Valuation and counts use one snapshot. Current catalogue cost is explicitly
// an estimate; this report is neither a historical write-off nor a DRE posting.
func Variance(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (VarianceReport, error) {
	out := VarianceReport{Basis: "recorded_count_difference_at_current_catalog_cost", Lines: []VarianceLine{}}
	if db == nil || !validID(id) {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, a, d); err != nil {
		return out, err
	}
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return out, err
	}
	out.Session, err = load(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	lines, err := linesTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	shortage, surplus := new(big.Rat), new(big.Rat)
	for _, line := range lines {
		row := VarianceLine{Line: line}
		var unit string
		err = tx.QueryRowContext(ctx, `SELECT unit,cost_cents FROM products WHERE tenant_id=? AND id=?`, a.TenantID, line.ProductID).Scan(&unit, &row.CostCents)
		if err != nil {
			return out, err
		}
		if unit != line.Unit || row.CostCents < 0 || row.CostCents > MaxQuantity {
			return out, ErrConflict
		}
		row.CostRequiresReview = row.CostCents == 0
		if line.CountedMilli == nil {
			out.UncountedLines++
		} else {
			out.CountedLines++
			if line.DifferenceMilli == nil || line.PreviousMilli == nil || !quantityOK(line.Unit, *line.CountedMilli) || !quantityOK(line.Unit, *line.PreviousMilli) || *line.DifferenceMilli != *line.CountedMilli-*line.PreviousMilli {
				return out, ErrConflict
			}
			q := *line.DifferenceMilli
			value := new(big.Rat).SetFrac(new(big.Int).Mul(big.NewInt(q), big.NewInt(row.CostCents)), big.NewInt(1000))
			n, d := value.Num().String(), value.Denom().String()
			row.DifferenceNumerator = &n
			row.DifferenceDenominator = &d
			if q < 0 {
				out.ShortageLines++
				shortage.Sub(shortage, value)
			} else if q > 0 {
				out.SurplusLines++
				surplus.Add(surplus, value)
			} else {
				out.EqualLines++
			}
		}
		out.Lines = append(out.Lines, row)
	}
	out.ShortageNumerator, out.ShortageDenominator = shortage.Num().String(), shortage.Denom().String()
	out.SurplusNumerator, out.SurplusDenominator = surplus.Num().String(), surplus.Denom().String()
	net := new(big.Rat).Sub(surplus, shortage)
	out.NetNumerator, out.NetDenominator = net.Num().String(), net.Denom().String()
	return out, tx.Commit()
}
