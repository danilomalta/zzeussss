package cashaccounting

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"sort"
	"time"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
)

func Read(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, w Window) (Report, error) {
	from, to, err := parseWindow(w)
	if db == nil || err != nil {
		return Report{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return Report{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,operator_id,closed_at FROM cash_sessions WHERE tenant_id=? AND store_id=? AND device_id=? AND closed_at IS NOT NULL ORDER BY id LIMIT ?`, a.TenantID, a.StoreID, d.DeviceID, MaxScopeSessions+1)
	if err != nil {
		return Report{}, err
	}
	type selected struct {
		id, operator, closed string
		at                   time.Time
	}
	selectedSessions := []selected{}
	scanned := 0
	for rows.Next() {
		var s selected
		if err = rows.Scan(&s.id, &s.operator, &s.closed); err != nil {
			rows.Close()
			return Report{}, err
		}
		scanned++
		if scanned > MaxScopeSessions {
			rows.Close()
			return Report{}, ErrRange
		}
		s.at, err = time.Parse(time.RFC3339Nano, s.closed)
		if err != nil {
			rows.Close()
			return Report{}, ErrInconsistent
		}
		if !s.at.Before(from) && s.at.Before(to) {
			selectedSessions = append(selectedSessions, s)
			if len(selectedSessions) > MaxPeriodSessions {
				rows.Close()
				return Report{}, ErrRange
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Report{}, err
	}
	sort.Slice(selectedSessions, func(i, j int) bool {
		if selectedSessions[i].at.Equal(selectedSessions[j].at) {
			return selectedSessions[i].id < selectedSessions[j].id
		}
		return selectedSessions[i].at.Before(selectedSessions[j].at)
	})
	r := Report{Scope: "local_closed_cash_sessions", Window: Window{from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano)}, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Summary: map[string]int64{}, Payments: []register.PaymentSummary{{Method: "cash"}, {Method: "card"}, {Method: "pix"}, {Method: "other"}}, Sessions: []Session{}, Entries: []Posting{}}
	totals := map[string]*big.Int{}
	for _, name := range metricNames {
		totals[name] = new(big.Int)
	}
	paymentTotals := make([][]*big.Int, len(r.Payments))
	for i := range paymentTotals {
		paymentTotals[i] = []*big.Int{new(big.Int), new(big.Int), new(big.Int), new(big.Int)}
	}
	for _, s := range selectedSessions {
		if err = ctx.Err(); err != nil {
			return Report{}, err
		}
		closed, err := register.ReadClosedReportForAccountingTx(ctx, tx, a, d, s.id)
		if err != nil {
			if errors.Is(err, register.ErrReportNotFound) {
				return Report{}, ErrInconsistent
			}
			return Report{}, err
		}
		opened, err := time.Parse(time.RFC3339Nano, closed.OpenedAt)
		if err != nil || opened.After(s.at) || closed.ClosedAt != s.closed || !closed.Complete {
			return Report{}, ErrInconsistent
		}
		remaining := MaxEntries - len(r.Entries)
		if remaining < 1 {
			return Report{}, ErrRange
		}
		journal, err := cashjournal.ReadForAccountingTx(ctx, tx, a, d, s.id, remaining)
		if err != nil {
			return Report{}, err
		}
		if !journal.SourceComplete || journal.HasMore || journal.TotalCount != int64(len(journal.Items)) || journal.Summary.ExpectedCents != closed.Cash.ExpectedCents || journal.Summary.DeclaredCents != closed.Cash.DeclaredCents || journal.Summary.DifferenceCents != closed.Cash.DifferenceCents {
			return Report{}, ErrInconsistent
		}
		values := map[string]int64{"session_count": 1, "sale_count": closed.SaleCount, "gross_sales_cents": closed.GrossSalesCents, "cancelled_sales_cents": closed.CancelledSalesCents, "returned_sales_cents": closed.ReturnedSalesCents, "net_sales_cents": closed.NetSalesCents, "sum_openings_cents": closed.Cash.OpeningCents, "sum_expected_at_closures_cents": closed.Cash.ExpectedCents, "sum_declared_at_closures_cents": closed.Cash.DeclaredCents, "sum_closure_differences_cents": closed.Cash.DifferenceCents, "supply_cents": closed.Cash.SupplyCents, "withdrawal_cents": closed.Cash.WithdrawalCents, "journal_entry_count": journal.TotalCount, "journal_debit_cents": journal.Summary.DebitCents, "journal_credit_cents": journal.Summary.CreditCents, "unclassified_count": journal.Summary.UnclassifiedCount}
		for name, n := range values {
			totals[name].Add(totals[name], big.NewInt(n))
		}
		for i, p := range closed.Payments {
			if i >= len(r.Payments) || p.Method != r.Payments[i].Method {
				return Report{}, ErrInconsistent
			}
			for j, n := range []int64{p.Parts, p.ConfirmedCents, p.PendingCents, p.ReversedCents} {
				paymentTotals[i][j].Add(paymentTotals[i][j], big.NewInt(n))
			}
		}
		r.Sessions = append(r.Sessions, Session{s.operator, closed})
		for _, entry := range journal.Items {
			r.Entries = append(r.Entries, Posting{s.id, s.operator, entry})
		}
	}
	for name, n := range totals {
		v, err := exact(n)
		if err != nil {
			return Report{}, err
		}
		r.Summary[name] = v
	}
	for i := range r.Payments {
		values := make([]int64, 4)
		for j, n := range paymentTotals[i] {
			v, err := exact(n)
			if err != nil {
				return Report{}, err
			}
			values[j] = v
		}
		r.Payments[i].Parts = values[0]
		r.Payments[i].ConfirmedCents = values[1]
		r.Payments[i].PendingCents = values[2]
		r.Payments[i].ReversedCents = values[3]
	}
	r.Complete = true
	if err = tx.Commit(); err != nil {
		return Report{}, err
	}
	return r, nil
}
func exact(n *big.Int) (int64, error) {
	if !n.IsInt64() || n.Cmp(big.NewInt(maxSafe)) > 0 || n.Cmp(big.NewInt(-maxSafe)) < 0 {
		return 0, ErrRange
	}
	return n.Int64(), nil
}
func bounds(limit, offset int) bool {
	return limit >= 1 && limit <= 100 && offset >= 0 && offset <= 1000000000
}
func Sessions(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, w Window, limit, offset int) (SessionPage, error) {
	if !bounds(limit, offset) {
		return SessionPage{}, ErrInvalid
	}
	r, err := Read(ctx, db, a, d, w)
	if err != nil {
		return SessionPage{}, err
	}
	p := SessionPage{Scope: r.Scope, Window: r.Window, Complete: r.Complete, Summary: r.Summary, Payments: r.Payments, Items: []Session{}, TotalCount: len(r.Sessions), Limit: limit, Offset: offset}
	end := offset + limit
	if end > len(r.Sessions) {
		end = len(r.Sessions)
	}
	if offset < len(r.Sessions) {
		p.Items = r.Sessions[offset:end]
	}
	p.HasMore = offset+len(p.Items) < p.TotalCount
	return p, nil
}
func Postings(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, w Window, limit, offset int) (PostingPage, error) {
	if !bounds(limit, offset) {
		return PostingPage{}, ErrInvalid
	}
	r, err := Read(ctx, db, a, d, w)
	if err != nil {
		return PostingPage{}, err
	}
	p := PostingPage{Scope: r.Scope, Window: r.Window, Complete: r.Complete, Summary: r.Summary, Items: []Posting{}, TotalCount: len(r.Entries), Limit: limit, Offset: offset}
	end := offset + limit
	if end > len(r.Entries) {
		end = len(r.Entries)
	}
	if offset < len(r.Entries) {
		p.Items = r.Entries[offset:end]
	}
	p.HasMore = offset+len(p.Items) < p.TotalCount
	return p, nil
}
