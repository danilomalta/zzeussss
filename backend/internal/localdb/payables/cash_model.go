package payables

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type CashReturn struct {
	SupplyOperationID string `json:"supply_operation_id"`
	ActorID           string `json:"actor_id"`
	Reason            string `json:"reason"`
	CreatedAt         string `json:"created_at"`
	OperationID       string `json:"operation_id"`
}
type CashPayment struct {
	ID                    string      `json:"id"`
	PayableID             string      `json:"payable_id"`
	InstallmentID         string      `json:"installment_id"`
	WithdrawalOperationID string      `json:"withdrawal_operation_id"`
	AmountCents           int64       `json:"amount_cents"`
	ActorID               string      `json:"actor_id"`
	Recipient             string      `json:"recipient"`
	Reference             string      `json:"reference"`
	CreatedAt             string      `json:"created_at"`
	OperationID           string      `json:"operation_id"`
	Return                *CashReturn `json:"return"`
}
type CashResult struct {
	OperationID string      `json:"operation_id"`
	Repeated    bool        `json:"repeated"`
	Payment     CashPayment `json:"payment"`
}
type CashInput struct {
	OperationID           string `json:"operation_id"`
	PaymentID             string `json:"payment_id"`
	InstallmentID         string `json:"installment_id"`
	WithdrawalOperationID string `json:"withdrawal_operation_id"`
	Recipient             string `json:"recipient"`
	Reference             string `json:"reference"`
}
type CashReturnInput struct {
	OperationID       string `json:"operation_id"`
	SupplyOperationID string `json:"supply_operation_id"`
	Reason            string `json:"reason"`
}
type CashSummary struct {
	PaidCents        int64  `json:"paid_cents"`
	ReturnedCents    int64  `json:"returned_cents"`
	AppliedCents     int64  `json:"applied_cents"`
	OutstandingCents int64  `json:"outstanding_cents"`
	Basis            string `json:"basis"`
}
type CashPage struct {
	Items      []CashPayment `json:"items"`
	Summary    CashSummary   `json:"summary"`
	TotalCount int           `json:"total_count"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
	HasMore    bool          `json:"has_more"`
}
type cashSource struct {
	SessionID, MovementID, ActorID, Kind, CreatedAt string
	Amount                                          int64
}

// Validate the immutable journal against its live source and audited adjustment.
// Caller owns the transaction and performs authorization before calling.
func sourceCash(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op string) (cashSource, error) {
	var s cashSource
	e := tx.QueryRowContext(ctx, `SELECT session_id,movement_id,actor_identity_id,kind,created_at,amount_cents FROM cash_adjustments WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&s.SessionID, &s.MovementID, &s.ActorID, &s.Kind, &s.CreatedAt, &s.Amount)
	if errors.Is(e, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	if e != nil {
		return s, e
	}
	if s.Amount < 1 || s.Amount > MaxSafe {
		return s, ErrInconsistent
	}
	if _, e = time.Parse(time.RFC3339Nano, s.CreatedAt); e != nil {
		return s, ErrInconsistent
	}
	var n int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_journal_entries j JOIN cash_sessions c ON c.tenant_id=j.tenant_id AND c.store_id=j.store_id AND c.device_id=j.device_id AND c.id=j.session_id JOIN cash_session_operators b ON b.tenant_id=c.tenant_id AND b.store_id=c.store_id AND b.session_id=c.id AND b.identity_id=c.operator_id JOIN cash_journal_sources v ON v.tenant_id=j.tenant_id AND v.store_id=j.store_id AND v.device_id=j.device_id AND v.entry_id=j.entry_id AND v.session_id=j.session_id AND v.source_kind=j.source_kind AND v.source_id=j.source_id AND v.movement_id=j.movement_id AND v.cash_delta_cents=j.cash_delta_cents AND v.occurred_at=j.occurred_at AND v.category=j.category AND v.operation_id=j.operation_id AND v.actor_identity_id=j.actor_identity_id AND v.counter_account=j.counter_account WHERE j.tenant_id=? AND j.store_id=? AND j.device_id=? AND j.source_kind='movement' AND j.movement_id=? AND j.operation_id=? AND j.actor_identity_id=? AND j.category=? AND j.occurred_at=? AND j.cash_delta_cents=?`, a.TenantID, a.StoreID, d.DeviceID, s.MovementID, op, s.ActorID, s.Kind, s.CreatedAt, func() int64 {
		if s.Kind == "withdrawal" {
			return -s.Amount
		}
		return s.Amount
	}()).Scan(&n)
	if e != nil {
		return s, e
	}
	if n != 1 {
		return s, ErrInconsistent
	}
	return s, nil
}
