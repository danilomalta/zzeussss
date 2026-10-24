package sale

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

const MaxCartReportRows = 1000
const MaxCartReportEvents = 10000

var ErrCartReportLimit = errors.New("cart report source limit")

type CartReportRow struct {
	CartID     string `json:"cart_id"`
	State      string `json:"state"`
	Revision   int64  `json:"revision"`
	UpdatedAt  string `json:"updated_at"`
	AgeSeconds int64  `json:"age_seconds"`
	LineCount  int    `json:"line_count"`
	SaleID     string `json:"sale_id"`
}
type CartReport struct {
	From                    string          `json:"from"`
	To                      string          `json:"to"`
	GeneratedAt             string          `json:"generated_at"`
	SourceComplete          bool            `json:"source_complete"`
	FinancialTotalsIncluded bool            `json:"financial_totals_included"`
	Total                   int             `json:"total"`
	Summary                 map[string]int  `json:"summary"`
	Items                   []CartReportRow `json:"items"`
}

func ReadCartReport(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, from, to string) (CartReport, error) {
	now := time.Now().UTC()
	out := CartReport{From: from, To: to, GeneratedAt: now.Format(time.RFC3339Nano), Summary: map[string]int{"suspended": 0, "claimed": 0, "completed": 0, "cancelled": 0}, Items: []CartReportRow{}}
	first, e := time.Parse(time.DateOnly, from)
	if e != nil || first.Year() < 1900 {
		return out, ErrInvalid
	}
	last, e := time.Parse(time.DateOnly, to)
	if e != nil || last.Year() > 2200 || last.Before(first) || last.Sub(first) >= 366*24*time.Hour {
		return out, ErrInvalid
	}
	tx, e := cartBegin(ctx, db, nil, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	where := `c.tenant_id=? AND c.store_id=? AND c.device_id=? AND c.actor_id=? AND c.updated_at>=? AND c.updated_at<?`
	args := []any{a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, first.Format("2006-01-02T15:04:05"), last.AddDate(0, 0, 1).Format("2006-01-02T15:04:05")}
	var n int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM pos_carts c WHERE `+where, args...).Scan(&n); e != nil {
		return out, e
	}
	if n > MaxCartReportRows {
		return out, ErrCartReportLimit
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM pos_cart_operations o JOIN pos_carts c ON c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND c.device_id=o.device_id AND c.id=o.cart_id WHERE `+where, args...).Scan(&n); e != nil {
		return out, e
	}
	if n > MaxCartReportEvents {
		return out, ErrCartReportLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT c.id FROM pos_carts c WHERE `+where+` ORDER BY c.updated_at,c.id`, args...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		v, e := cartGetTx(ctx, tx, a, d, id)
		if e != nil {
			return out, e
		}
		at, e := time.Parse(time.RFC3339Nano, v.UpdatedAt)
		if e != nil || at.After(now.Add(5*time.Minute)) {
			return out, ErrConflict
		}
		age := int64(now.Sub(at).Seconds())
		if age < 0 {
			age = 0
		}
		out.Items = append(out.Items, CartReportRow{CartID: v.ID, State: v.State, Revision: v.Revision, UpdatedAt: v.UpdatedAt, AgeSeconds: age, LineCount: len(v.Items), SaleID: v.SaleID})
		out.Summary[v.State]++
	}
	out.Total = len(out.Items)
	out.SourceComplete = true
	return out, nil
}
func cartCSVCell(s string) string {
	if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "@") {
		return "'" + s
	}
	return s
}

// WriteCartReportCSV only accepts an already validated complete result. It has
// no money columns: saved cart prices are not sales, receivables or settlement.
func WriteCartReportCSV(w io.Writer, r CartReport) error {
	if !r.SourceComplete || r.FinancialTotalsIncluded || r.Total != len(r.Items) {
		return ErrConflict
	}
	c := csv.NewWriter(w)
	if e := c.Write([]string{"cart_id", "state", "revision", "updated_at", "age_seconds", "line_count", "sale_id"}); e != nil {
		return e
	}
	for _, v := range r.Items {
		if e := c.Write([]string{cartCSVCell(v.CartID), v.State, strconv.FormatInt(v.Revision, 10), v.UpdatedAt, strconv.FormatInt(v.AgeSeconds, 10), strconv.Itoa(v.LineCount), cartCSVCell(v.SaleID)}); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
