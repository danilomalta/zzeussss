package payables

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func hash(v any) string { b, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(b)) }
func replay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id, h string) (Result, bool, error) {
	var who, saved string
	e := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash FROM supplier_payable_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &saved)
	if errors.Is(e, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	if e != nil {
		return Result{}, false, e
	}
	if who != a.IdentityID {
		return Result{}, false, ErrNotFound
	}
	if saved != h {
		return Result{}, false, ErrConflict
	}
	r, _, e := operation(ctx, tx, a, d, id)
	if e != nil {
		return r, false, e
	}
	if _, e = load(ctx, tx, a, d, r.Payable.ID); e != nil {
		return Result{}, false, e
	}
	r.Repeated = true
	return r, true, nil
}
func finish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, p Payable, e Event, h string) (Result, error) {
	p.History = append(p.History, e)
	if err := valid(p); err != nil {
		return Result{}, err
	}
	r := Result{OperationID: e.OperationID, Payable: p}
	b, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	if err = checked(ctx, tx, `INSERT INTO supplier_payable_operations VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, e.OperationID, p.ID, a.IdentityID, e.Action, e.FromState, e.ToState, e.Reason, h, string(b), e.CreatedAt); err != nil {
		return Result{}, err
	}
	if err = checked(ctx, tx, `INSERT INTO supplier_payable_events VALUES(?,?,?,?,?,1,?,?)`, a.TenantID, a.StoreID, d.DeviceID, e.OperationID, "supplier.payable."+e.Action, string(b), e.CreatedAt); err != nil {
		return Result{}, err
	}
	return r, tx.Commit()
}
func Create(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in CreateInput) (Result, error) {
	if db == nil || contracts == nil || !key(in.OperationID) || !key(in.PayableID) || !text(in.SupplierID, 128) || !key(in.DocumentReference) || !text(in.Description, 500) || in.Currency != "BRL" || !schedule(in.Installments, in.TotalCents, in.IssuedOn) {
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.Finance); err != nil {
		return Result{}, err
	}
	h := hash(in)
	if r, ok, e := replay(ctx, tx, a, d, in.OperationID, h); ok || e != nil {
		return r, e
	}
	var name, status string
	err = tx.QueryRowContext(ctx, `SELECT name,status FROM purchase_suppliers WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.SupplierID).Scan(&name, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	if status != "active" {
		return Result{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	p := Payable{ID: in.PayableID, SupplierID: in.SupplierID, SupplierName: name, DocumentReference: in.DocumentReference, Description: in.Description, IssuedOn: in.IssuedOn, Currency: in.Currency, TotalCents: in.TotalCents, CreatorID: a.IdentityID, State: "pending", CreatedAt: now, UpdatedAt: now, LastOperationID: in.OperationID, Installments: append([]Installment{}, in.Installments...), History: []Event{}}
	if err = checked(ctx, tx, `INSERT INTO supplier_payables VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, p.ID, p.SupplierID, p.SupplierName, p.DocumentReference, p.Description, p.IssuedOn, p.Currency, p.TotalCents, p.CreatorID, p.State, now, now, in.OperationID); err != nil {
		return Result{}, err
	}
	for i, v := range p.Installments {
		if err = checked(ctx, tx, `INSERT INTO supplier_payable_installments VALUES(?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, p.ID, v.ID, i+1, v.DueOn, v.AmountCents); err != nil {
			return Result{}, err
		}
	}
	return finish(ctx, tx, a, d, p, Event{in.OperationID, a.IdentityID, "create", "", "pending", p.Description, now}, h)
}
func Transition(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, action string, in ActionInput) (Result, error) {
	if db == nil || contracts == nil || !key(id) || !key(in.OperationID) || !text(in.Reason, 500) || (action != "approve" && action != "reject" && action != "cancel") {
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.Finance); err != nil {
		return Result{}, err
	}
	h := hash(struct {
		ID, Action string
		Input      ActionInput
	}{id, action, in})
	if r, ok, e := replay(ctx, tx, a, d, in.OperationID, h); ok || e != nil {
		return r, e
	}
	p, err := load(ctx, tx, a, d, id)
	if err != nil {
		return Result{}, err
	}
	before := p.State
	after := ""
	switch action {
	case "approve", "reject":
		if a.IdentityID == p.CreatorID {
			return Result{}, identity.ErrDenied
		}
		if before != "pending" {
			return Result{}, ErrConflict
		}
		after = "approved"
		if action == "reject" {
			after = "rejected"
		}
	case "cancel":
		if before != "pending" && before != "approved" {
			return Result{}, ErrConflict
		}
		if before == "pending" && a.IdentityID != p.CreatorID || before == "approved" && a.IdentityID == p.CreatorID {
			return Result{}, identity.ErrDenied
		}
		after = "cancelled"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	p.State = after
	p.UpdatedAt = now
	p.LastOperationID = in.OperationID
	if err = checked(ctx, tx, `UPDATE supplier_payables SET state=?,updated_at=?,last_operation_id=? WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND state=?`, after, now, in.OperationID, a.TenantID, a.StoreID, d.DeviceID, id, before); err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, a, d, p, Event{in.OperationID, a.IdentityID, action, before, after, in.Reason, now}, h)
}
