package payables

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

func cashOperation(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (CashResult, string, string, error) {
	var r CashResult
	var who, action, pid, body, payload, event, at string
	e := tx.QueryRowContext(ctx, `SELECT o.actor_id,o.action,o.payment_id,o.result_json,e.payload_json,e.event_type,o.created_at FROM payable_cash_operations o LEFT JOIN payable_cash_events e ON e.tenant_id=o.tenant_id AND e.store_id=o.store_id AND e.device_id=o.device_id AND e.operation_id=o.operation_id WHERE o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &action, &pid, &body, &payload, &event, &at)
	if errors.Is(e, sql.ErrNoRows) {
		return r, "", "", ErrNotFound
	}
	if e != nil {
		return r, "", "", ErrInconsistent
	}
	if json.Unmarshal([]byte(body), &r) != nil || r.Repeated || r.OperationID != id || r.Payment.ID != pid || body != payload || event != "payable.cash."+action {
		return r, "", "", ErrInconsistent
	}
	p := r.Payment
	if !key(p.ID) || !key(p.PayableID) || !key(p.InstallmentID) || !key(p.WithdrawalOperationID) || p.AmountCents < 1 || p.AmountCents > MaxSafe || !text(p.ActorID, 128) || !text(p.Recipient, 255) || !key(p.Reference) || !key(p.OperationID) {
		return r, "", "", ErrInconsistent
	}
	if _, e = time.Parse(time.RFC3339Nano, p.CreatedAt); e != nil {
		return r, "", "", ErrInconsistent
	}
	switch action {
	case "pay":
		if p.Return != nil || p.OperationID != id || p.ActorID != who || p.CreatedAt != at {
			return r, "", "", ErrInconsistent
		}
	case "return":
		if p.Return == nil || p.Return.OperationID != id || p.Return.ActorID != who || p.Return.CreatedAt != at || !key(p.Return.SupplyOperationID) || !text(p.Return.Reason, 500) {
			return r, "", "", ErrInconsistent
		}
	default:
		return r, "", "", ErrInconsistent
	}
	return r, who, action, nil
}
func loadCash(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (CashPayment, error) {
	var p CashPayment
	e := tx.QueryRowContext(ctx, `SELECT id,payable_id,installment_id,withdrawal_operation_id,amount_cents,actor_id,recipient,reference,created_at,operation_id FROM payable_cash_payments WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&p.ID, &p.PayableID, &p.InstallmentID, &p.WithdrawalOperationID, &p.AmountCents, &p.ActorID, &p.Recipient, &p.Reference, &p.CreatedAt, &p.OperationID)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if e != nil {
		return p, e
	}
	r, _, action, e := cashOperation(ctx, tx, a, d, p.OperationID)
	if e != nil || action != "pay" || !reflect.DeepEqual(r.Payment, p) {
		return p, ErrInconsistent
	}
	src, e := sourceCash(ctx, tx, a, d, p.WithdrawalOperationID)
	if e != nil || src.Kind != "withdrawal" || src.Amount != p.AmountCents || src.ActorID != p.ActorID {
		return p, ErrInconsistent
	}
	sourceTime, _ := time.Parse(time.RFC3339Nano, src.CreatedAt)
	created, _ := time.Parse(time.RFC3339Nano, p.CreatedAt)
	if created.Before(sourceTime) {
		return p, ErrInconsistent
	}
	var bags int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_bags WHERE tenant_id=? AND device_id=? AND withdrawal_operation_id=?`, a.TenantID, d.DeviceID, p.WithdrawalOperationID).Scan(&bags); e != nil {
		return p, e
	}
	if bags != 0 {
		return p, ErrInconsistent
	}
	var ret CashReturn
	e = tx.QueryRowContext(ctx, `SELECT supply_operation_id,actor_id,reason,created_at,operation_id FROM payable_cash_returns WHERE tenant_id=? AND store_id=? AND device_id=? AND payment_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&ret.SupplyOperationID, &ret.ActorID, &ret.Reason, &ret.CreatedAt, &ret.OperationID)
	if errors.Is(e, sql.ErrNoRows) {
		return p, nil
	}
	if e != nil {
		return p, e
	}
	p.Return = &ret
	r, _, action, e = cashOperation(ctx, tx, a, d, ret.OperationID)
	if e != nil || action != "return" || !reflect.DeepEqual(r.Payment, p) {
		return p, ErrInconsistent
	}
	returned, e := sourceCash(ctx, tx, a, d, ret.SupplyOperationID)
	if e != nil || returned.Kind != "supply" || returned.Amount != p.AmountCents || returned.ActorID != ret.ActorID {
		return p, ErrInconsistent
	}
	returnTime, _ := time.Parse(time.RFC3339Nano, returned.CreatedAt)
	recordTime, err := time.Parse(time.RFC3339Nano, ret.CreatedAt)
	if err != nil || returnTime.Before(sourceTime) || recordTime.Before(created) || recordTime.Before(returnTime) {
		return p, ErrInconsistent
	}
	var vaults int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_vault_entries WHERE tenant_id=? AND device_id=? AND cash_movement_id=?`, a.TenantID, d.DeviceID, returned.MovementID).Scan(&vaults)
	if e != nil {
		return p, e
	}
	if vaults != 0 {
		return p, ErrInconsistent
	}
	return p, nil
}
func cashForDocument(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, p Payable) ([]CashPayment, CashSummary, map[string]int64, error) {
	s := CashSummary{OutstandingCents: p.TotalCents, Basis: "declared_cash_audited_sources"}
	parts := map[string]int64{}
	limits := map[string]int64{}
	for _, v := range p.Installments {
		limits[v.ID] = v.AmountCents
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM payable_cash_payments WHERE tenant_id=? AND store_id=? AND device_id=? AND payable_id=? ORDER BY created_at,id LIMIT 1001`, a.TenantID, a.StoreID, d.DeviceID, p.ID)
	if e != nil {
		return nil, s, nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, s, nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, s, nil, e
	}
	if len(ids) > 1000 {
		return nil, s, nil, ErrRange
	}
	out := []CashPayment{}
	for _, id := range ids {
		v, e := loadCash(ctx, tx, a, d, id)
		if e != nil {
			return nil, s, nil, e
		}
		if v.AmountCents > MaxSafe-s.PaidCents {
			return nil, s, nil, ErrRange
		}
		s.PaidCents += v.AmountCents
		if v.Return != nil {
			s.ReturnedCents += v.AmountCents
		} else {
			if v.AmountCents > limits[v.InstallmentID]-parts[v.InstallmentID] {
				return nil, s, nil, ErrInconsistent
			}
			parts[v.InstallmentID] += v.AmountCents
			s.AppliedCents += v.AmountCents
		}
		out = append(out, v)
	}
	if s.AppliedCents > p.TotalCents || s.PaidCents-s.ReturnedCents != s.AppliedCents || s.AppliedCents > 0 && p.State != "approved" {
		return nil, s, nil, ErrInconsistent
	}
	s.OutstandingCents = p.TotalCents - s.AppliedCents
	if p.State != "approved" {
		s.OutstandingCents = 0
	}
	return out, s, parts, nil
}
func CashPayments(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (CashPage, error) {
	if !key(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return CashPage{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return CashPage{}, e
	}
	defer tx.Rollback()
	p, e := load(ctx, tx, a, d, id)
	if e != nil {
		return CashPage{}, e
	}
	items, s, _, e := cashForDocument(ctx, tx, a, d, p)
	if e != nil {
		return CashPage{}, e
	}
	r := CashPage{Items: []CashPayment{}, Summary: s, TotalCount: len(items), Limit: limit, Offset: offset}
	if offset < len(items) {
		end := offset + limit
		if end > len(items) {
			end = len(items)
		}
		r.Items = items[offset:end]
	}
	r.HasMore = offset+len(r.Items) < len(items)
	return r, tx.Commit()
}
func CashReceipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (CashResult, error) {
	if !key(id) {
		return CashResult{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return CashResult{}, e
	}
	defer tx.Rollback()
	r, who, _, e := cashOperation(ctx, tx, a, d, id)
	if e != nil {
		return r, e
	}
	if who != a.IdentityID {
		return CashResult{}, ErrNotFound
	}
	p, e := load(ctx, tx, a, d, r.Payment.PayableID)
	if e != nil {
		return CashResult{}, e
	}
	if _, _, _, e = cashForDocument(ctx, tx, a, d, p); e != nil {
		return CashResult{}, e
	}
	return r, tx.Commit()
}

// WithdrawalUseTx classifies this source for custody without creating a second
// withdrawal. Returned money is a different, audited supply, never free custody.
// Caller authorizes ManageCash and owns the transaction.
func WithdrawalUseTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op string) (int64, int64, error) {
	var id, pid string
	e := tx.QueryRowContext(ctx, `SELECT id,payable_id FROM payable_cash_payments WHERE tenant_id=? AND store_id=? AND device_id=? AND withdrawal_operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&id, &pid)
	if errors.Is(e, sql.ErrNoRows) {
		return 0, 0, nil
	}
	if e != nil {
		return 0, 0, e
	}
	p, e := load(ctx, tx, a, d, pid)
	if e != nil {
		return 0, 0, e
	}
	items, _, _, e := cashForDocument(ctx, tx, a, d, p)
	if e != nil {
		return 0, 0, e
	}
	for _, v := range items {
		if v.ID == id {
			if v.Return != nil {
				return v.AmountCents, v.AmountCents, nil
			}
			return v.AmountCents, 0, nil
		}
	}
	return 0, 0, ErrInconsistent
}
