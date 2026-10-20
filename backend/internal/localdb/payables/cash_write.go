package payables

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func replayCash(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id, h string) (CashResult, bool, error) {
	var who, saved string
	e := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash FROM payable_cash_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &saved)
	if errors.Is(e, sql.ErrNoRows) {
		return CashResult{}, false, nil
	}
	if e != nil {
		return CashResult{}, false, e
	}
	if who != a.IdentityID {
		return CashResult{}, false, ErrNotFound
	}
	if saved != h {
		return CashResult{}, false, ErrConflict
	}
	r, _, _, e := cashOperation(ctx, tx, a, d, id)
	if e != nil {
		return r, false, e
	}
	p, e := load(ctx, tx, a, d, r.Payment.PayableID)
	if e != nil {
		return CashResult{}, false, e
	}
	if _, _, _, e = cashForDocument(ctx, tx, a, d, p); e != nil {
		return CashResult{}, false, e
	}
	r.Repeated = true
	return r, true, nil
}
func finishCash(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, p CashPayment, op, action, h, now string) (CashResult, error) {
	r := CashResult{OperationID: op, Payment: p}
	b, e := json.Marshal(r)
	if e != nil {
		return r, e
	}
	if e = checked(ctx, tx, `INSERT INTO payable_cash_operations VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, p.ID, a.IdentityID, action, h, string(b), now); e != nil {
		return CashResult{}, e
	}
	if e = checked(ctx, tx, `INSERT INTO payable_cash_events VALUES(?,?,?,?,?,1,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, "payable.cash."+action, string(b), now); e != nil {
		return CashResult{}, e
	}
	return r, tx.Commit()
}
func PayCash(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in CashInput) (CashResult, error) {
	if db == nil || contracts == nil || !key(id) || !key(in.OperationID) || !key(in.PaymentID) || !key(in.InstallmentID) || !key(in.WithdrawalOperationID) || !text(in.Recipient, 255) || !key(in.Reference) {
		return CashResult{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return CashResult{}, e
	}
	defer tx.Rollback()
	if e = contracts.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.Finance); e != nil {
		return CashResult{}, e
	}
	h := hash(struct {
		ID    string
		Input CashInput
	}{id, in})
	if r, ok, e := replayCash(ctx, tx, a, d, in.OperationID, h); ok || e != nil {
		return r, e
	}
	p, e := load(ctx, tx, a, d, id)
	if e != nil {
		return CashResult{}, e
	}
	if p.State != "approved" {
		return CashResult{}, ErrConflict
	}
	items, s, parts, e := cashForDocument(ctx, tx, a, d, p)
	if e != nil {
		return CashResult{}, e
	}
	if len(items) >= 1000 {
		return CashResult{}, ErrRange
	}
	src, e := sourceCash(ctx, tx, a, d, in.WithdrawalOperationID)
	if e != nil {
		return CashResult{}, e
	}
	if src.ActorID != a.IdentityID {
		return CashResult{}, identity.ErrDenied
	}
	if src.Kind != "withdrawal" {
		return CashResult{}, ErrConflict
	}
	limit := int64(0)
	for _, v := range p.Installments {
		if v.ID == in.InstallmentID {
			limit = v.AmountCents
		}
	}
	if limit == 0 {
		return CashResult{}, ErrNotFound
	}
	if src.Amount > limit-parts[in.InstallmentID] {
		return CashResult{}, ErrConflict
	}
	if src.Amount > MaxSafe-s.PaidCents {
		return CashResult{}, ErrRange
	}
	var used int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_bags WHERE tenant_id=? AND device_id=? AND withdrawal_operation_id=?`, a.TenantID, d.DeviceID, in.WithdrawalOperationID).Scan(&used); e != nil {
		return CashResult{}, e
	}
	if used != 0 {
		return CashResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	sourceAt, _ := time.Parse(time.RFC3339Nano, src.CreatedAt)
	recordAt, _ := time.Parse(time.RFC3339Nano, now)
	if recordAt.Before(sourceAt) {
		return CashResult{}, ErrInconsistent
	}
	v := CashPayment{ID: in.PaymentID, PayableID: id, InstallmentID: in.InstallmentID, WithdrawalOperationID: in.WithdrawalOperationID, AmountCents: src.Amount, ActorID: a.IdentityID, Recipient: in.Recipient, Reference: in.Reference, CreatedAt: now, OperationID: in.OperationID}
	if e = checked(ctx, tx, `INSERT INTO payable_cash_payments VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, v.ID, id, v.InstallmentID, v.WithdrawalOperationID, v.AmountCents, a.IdentityID, v.Recipient, v.Reference, now, in.OperationID); e != nil {
		return CashResult{}, e
	}
	return finishCash(ctx, tx, a, d, v, in.OperationID, "pay", h, now)
}
func ReturnCash(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in CashReturnInput) (CashResult, error) {
	if db == nil || contracts == nil || !key(id) || !key(in.OperationID) || !key(in.SupplyOperationID) || !text(in.Reason, 500) {
		return CashResult{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return CashResult{}, e
	}
	defer tx.Rollback()
	if e = contracts.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.Finance); e != nil {
		return CashResult{}, e
	}
	h := hash(struct {
		ID    string
		Input CashReturnInput
	}{id, in})
	if r, ok, e := replayCash(ctx, tx, a, d, in.OperationID, h); ok || e != nil {
		return r, e
	}
	v, e := loadCash(ctx, tx, a, d, id)
	if e != nil {
		return CashResult{}, e
	}
	p, e := load(ctx, tx, a, d, v.PayableID)
	if e != nil {
		return CashResult{}, e
	}
	if _, _, _, e = cashForDocument(ctx, tx, a, d, p); e != nil {
		return CashResult{}, e
	}
	if v.Return != nil {
		return CashResult{}, ErrConflict
	}
	src, e := sourceCash(ctx, tx, a, d, in.SupplyOperationID)
	if e != nil {
		return CashResult{}, e
	}
	if src.ActorID != a.IdentityID {
		return CashResult{}, identity.ErrDenied
	}
	if src.Kind != "supply" || src.Amount != v.AmountCents {
		return CashResult{}, ErrConflict
	}
	withdrawn, e := sourceCash(ctx, tx, a, d, v.WithdrawalOperationID)
	if e != nil {
		return CashResult{}, e
	}
	st, _ := time.Parse(time.RFC3339Nano, src.CreatedAt)
	wt, _ := time.Parse(time.RFC3339Nano, withdrawn.CreatedAt)
	if st.Before(wt) {
		return CashResult{}, ErrConflict
	}
	var used int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_vault_entries WHERE tenant_id=? AND device_id=? AND cash_movement_id=?`, a.TenantID, d.DeviceID, src.MovementID).Scan(&used)
	if e != nil {
		return CashResult{}, e
	}
	if used != 0 {
		return CashResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	sourceAt, _ := time.Parse(time.RFC3339Nano, src.CreatedAt)
	recordAt, _ := time.Parse(time.RFC3339Nano, now)
	if recordAt.Before(sourceAt) {
		return CashResult{}, ErrInconsistent
	}
	ret := CashReturn{in.SupplyOperationID, a.IdentityID, in.Reason, now, in.OperationID}
	v.Return = &ret
	if e = checked(ctx, tx, `INSERT INTO payable_cash_returns VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, id, ret.SupplyOperationID, a.IdentityID, ret.Reason, now, in.OperationID); e != nil {
		return CashResult{}, e
	}
	return finishCash(ctx, tx, a, d, v, in.OperationID, "return", h, now)
}
