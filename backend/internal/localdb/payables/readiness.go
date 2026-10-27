package payables

import (
	"context"
	"database/sql"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

const MaxCashSources = 10000

type InstallmentBalance struct {
	Installment      Installment `json:"installment"`
	PaidCents        int64       `json:"paid_cents"`
	ReturnedCents    int64       `json:"returned_cents"`
	AppliedCents     int64       `json:"applied_cents"`
	OutstandingCents int64       `json:"outstanding_cents"`
}
type InstallmentBalances struct {
	PayableID   string               `json:"payable_id"`
	State       string               `json:"state"`
	Currency    string               `json:"currency"`
	GeneratedAt string               `json:"generated_at"`
	Complete    bool                 `json:"complete"`
	Summary     CashSummary          `json:"summary"`
	Items       []InstallmentBalance `json:"items"`
}
type CashCandidate struct {
	OperationID string `json:"operation_id"`
	SessionID   string `json:"session_id"`
	MovementID  string `json:"movement_id"`
	ActorID     string `json:"actor_id"`
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents"`
	OccurredAt  string `json:"occurred_at"`
}
type CashCandidates struct {
	Purpose                   string          `json:"purpose"`
	TargetID                  string          `json:"target_id"`
	InstallmentID             string          `json:"installment_id"`
	Currency                  string          `json:"currency"`
	GeneratedAt               string          `json:"generated_at"`
	Complete                  bool            `json:"complete"`
	ReservationCreated        bool            `json:"reservation_created"`
	WriteAuthorizationChecked bool            `json:"write_authorization_checked"`
	AmountRule                string          `json:"amount_rule"`
	AmountLimitCents          int64           `json:"amount_limit_cents"`
	BlockedReason             string          `json:"blocked_reason"`
	Items                     []CashCandidate `json:"items"`
	TotalCount                int             `json:"total_count"`
	Limit                     int             `json:"limit"`
	Offset                    int             `json:"offset"`
	HasMore                   bool            `json:"has_more"`
}

// All installment totals come from the same authorized SQLite read transaction.
func Balances(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (InstallmentBalances, error) {
	if !key(id) {
		return InstallmentBalances{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return InstallmentBalances{}, e
	}
	defer tx.Rollback()
	p, e := load(ctx, tx, a, d, id)
	if e != nil {
		return InstallmentBalances{}, e
	}
	payments, s, _, e := cashForDocument(ctx, tx, a, d, p)
	if e != nil {
		return InstallmentBalances{}, e
	}
	r := InstallmentBalances{PayableID: id, State: p.State, Currency: "BRL", GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Complete: true, Summary: s, Items: []InstallmentBalance{}}
	for _, part := range p.Installments {
		v := InstallmentBalance{Installment: part}
		for _, cash := range payments {
			if cash.InstallmentID == part.ID {
				v.PaidCents += cash.AmountCents
				if cash.Return != nil {
					v.ReturnedCents += cash.AmountCents
				}
			}
		}
		v.AppliedCents = v.PaidCents - v.ReturnedCents
		if p.State == "approved" {
			v.OutstandingCents = part.AmountCents - v.AppliedCents
		}
		r.Items = append(r.Items, v)
	}
	return r, tx.Commit()
}

func candidateBounds(limit, offset int) bool {
	return limit >= 1 && limit <= 100 && offset >= 0 && offset <= 1000000000
}

// Candidates are a snapshot of source rules, not permission/license approval or a
// reservation. The existing write transaction remains the authority at submission.
func PaymentSources(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id, part string, limit, offset int) (CashCandidates, error) {
	if !key(id) || !key(part) || !candidateBounds(limit, offset) {
		return CashCandidates{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return CashCandidates{}, e
	}
	defer tx.Rollback()
	p, e := load(ctx, tx, a, d, id)
	if e != nil {
		return CashCandidates{}, e
	}
	payments, s, parts, e := cashForDocument(ctx, tx, a, d, p)
	if e != nil {
		return CashCandidates{}, e
	}
	amount := int64(-1)
	for _, v := range p.Installments {
		if v.ID == part {
			amount = v.AmountCents - parts[part]
		}
	}
	if amount < 0 {
		return CashCandidates{}, ErrNotFound
	}
	r := CashCandidates{Purpose: "pay", TargetID: id, InstallmentID: part, AmountRule: "up_to_outstanding", AmountLimitCents: amount}
	switch {
	case p.State != "approved":
		r.BlockedReason = "document_not_approved"
	case amount == 0:
		r.BlockedReason = "installment_fully_applied"
	case len(payments) >= 1000:
		r.BlockedReason = "payment_history_limit"
	case s.PaidCents == MaxSafe:
		r.BlockedReason = "historical_amount_limit"
	}
	if amount > MaxSafe-s.PaidCents {
		r.AmountLimitCents = MaxSafe - s.PaidCents
	}
	if r.BlockedReason != "" {
		r.AmountLimitCents = 0
	}
	return candidates(ctx, tx, a, d, r, "withdrawal", time.Time{}, limit, offset)
}
func ReturnSources(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (CashCandidates, error) {
	if !key(id) || !candidateBounds(limit, offset) {
		return CashCandidates{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return CashCandidates{}, e
	}
	defer tx.Rollback()
	payment, e := loadCash(ctx, tx, a, d, id)
	if e != nil {
		return CashCandidates{}, e
	}
	p, e := load(ctx, tx, a, d, payment.PayableID)
	if e != nil {
		return CashCandidates{}, e
	}
	if _, _, _, e = cashForDocument(ctx, tx, a, d, p); e != nil {
		return CashCandidates{}, e
	}
	source, e := sourceCash(ctx, tx, a, d, payment.WithdrawalOperationID)
	if e != nil {
		return CashCandidates{}, e
	}
	earliest, _ := time.Parse(time.RFC3339Nano, source.CreatedAt)
	r := CashCandidates{Purpose: "return", TargetID: id, InstallmentID: payment.InstallmentID, AmountRule: "exact_payment_amount", AmountLimitCents: payment.AmountCents}
	if payment.Return != nil {
		r.BlockedReason = "payment_already_returned"
		r.AmountLimitCents = 0
	}
	return candidates(ctx, tx, a, d, r, "supply", earliest, limit, offset)
}

func candidates(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, r CashCandidates, kind string, earliest time.Time, limit, offset int) (CashCandidates, error) {
	// Scan and validate the complete actor-owned source set before slicing. Unknown
	// actors' movements are never exposed or used as candidates for this caller.
	rows, e := tx.QueryContext(ctx, `SELECT operation_id FROM cash_adjustments WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_identity_id=? AND kind=? ORDER BY operation_id LIMIT ?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, kind, MaxCashSources+1)
	if e != nil {
		return CashCandidates{}, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return CashCandidates{}, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return CashCandidates{}, e
	}
	if len(ids) > MaxCashSources {
		return CashCandidates{}, ErrRange
	}
	now := time.Now().UTC()
	r.GeneratedAt = now.Format(time.RFC3339Nano)
	r.Currency = "BRL"
	r.Complete = true
	r.Limit = limit
	r.Offset = offset
	r.Items = []CashCandidate{}
	all := []CashCandidate{}
	for _, op := range ids {
		if e = ctx.Err(); e != nil {
			return CashCandidates{}, e
		}
		if !key(op) {
			return CashCandidates{}, ErrInconsistent
		}
		s, e := sourceCash(ctx, tx, a, d, op)
		if e != nil {
			return CashCandidates{}, e
		}
		at, _ := time.Parse(time.RFC3339Nano, s.CreatedAt)
		if s.ActorID != a.IdentityID || s.Kind != kind || at.After(now) {
			return CashCandidates{}, ErrInconsistent
		}
		// Used sources remain excluded forever, even after cancellation or return.
		used := 0
		if kind == "withdrawal" {
			e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM cash_bags WHERE tenant_id=? AND device_id=? AND withdrawal_operation_id=?)+(SELECT count(*) FROM payable_cash_payments WHERE tenant_id=? AND device_id=? AND withdrawal_operation_id=?)`, a.TenantID, d.DeviceID, op, a.TenantID, d.DeviceID, op).Scan(&used)
		} else {
			e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM cash_vault_entries WHERE tenant_id=? AND device_id=? AND cash_movement_id=?)+(SELECT count(*) FROM payable_cash_returns WHERE tenant_id=? AND device_id=? AND supply_operation_id=?)`, a.TenantID, d.DeviceID, s.MovementID, a.TenantID, d.DeviceID, op).Scan(&used)
		}
		if e != nil {
			return CashCandidates{}, e
		}
		if r.BlockedReason != "" || used != 0 || s.Amount > r.AmountLimitCents || kind == "supply" && (s.Amount != r.AmountLimitCents || at.Before(earliest)) {
			continue
		}
		all = append(all, CashCandidate{op, s.SessionID, s.MovementID, s.ActorID, s.Kind, s.Amount, s.CreatedAt})
	}
	r.TotalCount = len(all)
	if offset < len(all) {
		end := offset + limit
		if end > len(all) {
			end = len(all)
		}
		r.Items = all[offset:end]
	}
	r.HasMore = offset+len(r.Items) < len(all)
	return r, tx.Commit()
}
