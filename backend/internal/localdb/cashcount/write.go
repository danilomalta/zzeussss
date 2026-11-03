package cashcount

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/cashvault"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func hash(v any) string { b, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(b)) }
func replay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id, h string) (Result, bool, error) {
	var who, saved string
	err := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash FROM cash_vault_count_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &saved)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if who != a.IdentityID {
		return Result{}, false, ErrNotFound
	}
	if saved != h {
		return Result{}, false, ErrConflict
	}
	r, _, err := readOperation(ctx, tx, a, d, id)
	if err != nil {
		return r, false, err
	}
	r.Count = visible(r.Count)
	r.Repeated = true
	return r, true, nil
}
func finish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, c Count, e Event, h string) (Result, error) {
	c.History = append(c.History, e)
	if err := valid(c); err != nil {
		return Result{}, err
	}
	r := Result{OperationID: e.OperationID, Count: c}
	b, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	if err = checked(ctx, tx, `INSERT INTO cash_vault_count_operations(tenant_id,store_id,device_id,operation_id,count_id,actor_id,action,from_state,to_state,reason,request_hash,result_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, e.OperationID, c.ID, a.IdentityID, e.Action, e.FromState, e.ToState, e.Reason, h, string(b), e.CreatedAt); err != nil {
		return Result{}, err
	}
	if err = checked(ctx, tx, `INSERT INTO cash_vault_count_events(tenant_id,store_id,device_id,operation_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,1,?,?)`, a.TenantID, a.StoreID, d.DeviceID, e.OperationID, "cash.vault.count."+e.Action, string(b), e.CreatedAt); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	r.Count = visible(c)
	return r, nil
}
func Create(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in CreateInput) (Result, error) {
	if db == nil || !key(in.OperationID) || !key(in.CountID) || !key(in.VaultID) || !text(in.CounterID, 128) || !text(in.Reason, 500) {
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.POS); err != nil {
		return Result{}, err
	}
	h := hash(in)
	if r, ok, err := replay(ctx, tx, a, d, in.OperationID, h); ok || err != nil {
		return r, err
	}
	if in.CounterID == a.IdentityID {
		return Result{}, identity.ErrDenied
	}
	counter := a
	counter.IdentityID = in.CounterID
	if err = identity.CanOperateTx(ctx, tx, counter, d, identity.ManageCash); err != nil {
		return Result{}, err
	}
	v, err := cashvault.InspectTx(ctx, tx, a, d, in.VaultID)
	if err != nil {
		return Result{}, err
	}
	if v.State != "active" {
		return Result{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	c := Count{ID: in.CountID, VaultID: in.VaultID, CreatorID: a.IdentityID, CounterID: in.CounterID, Reason: in.Reason, State: "counting", ExpectedCents: &v.BalanceCents, VaultOperationID: v.LastOperationID, CreatedAt: now, UpdatedAt: now, LastOperationID: in.OperationID, History: []Event{}}
	if err = checked(ctx, tx, `INSERT INTO cash_vault_counts(tenant_id,store_id,device_id,id,vault_id,creator_id,counter_id,reason,expected_cents,vault_operation_id,state,created_at,updated_at,last_operation_id) VALUES(?,?,?,?,?,?,?,?,?,?,'counting',?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, c.ID, c.VaultID, c.CreatorID, c.CounterID, c.Reason, *c.ExpectedCents, c.VaultOperationID, now, now, in.OperationID); err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, a, d, c, Event{OperationID: in.OperationID, ActorID: a.IdentityID, Action: "create", ToState: "counting", Reason: in.Reason, CreatedAt: now}, h)
}
func Transition(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, action string, in ActionInput) (Result, error) {
	if db == nil || !key(id) || !key(in.OperationID) {
		return Result{}, ErrInvalid
	}
	switch action {
	case "submit":
		if in.CountedCents < 0 || in.CountedCents > maxSafe || in.Reason != "" {
			return Result{}, ErrInvalid
		}
	case "accept", "reject", "cancel":
		if !text(in.Reason, 500) || in.CountedCents != 0 {
			return Result{}, ErrInvalid
		}
	default:
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.POS); err != nil {
		return Result{}, err
	}
	h := hash(struct {
		ID, Action string
		Input      ActionInput
	}{id, action, in})
	if r, ok, err := replay(ctx, tx, a, d, in.OperationID, h); ok || err != nil {
		return r, err
	}
	c, err := load(ctx, tx, a, d, id)
	if err != nil {
		return Result{}, err
	}
	before := c.State
	after := ""
	switch action {
	case "submit":
		if a.IdentityID != c.CounterID {
			return Result{}, identity.ErrDenied
		}
		if before != "counting" {
			return Result{}, ErrConflict
		}
		after = "submitted"
		c.CountedCents = &in.CountedCents
		diff := in.CountedCents - *c.ExpectedCents
		c.DifferenceCents = &diff
	case "cancel":
		if a.IdentityID != c.CreatorID {
			return Result{}, identity.ErrDenied
		}
		if before != "counting" {
			return Result{}, ErrConflict
		}
		after = "cancelled"
	case "accept", "reject":
		if a.IdentityID == c.CounterID {
			return Result{}, identity.ErrDenied
		}
		if before != "submitted" {
			return Result{}, ErrConflict
		}
		after = "accepted"
		if action == "reject" {
			after = "rejected"
		}
		c.ReviewerID = &a.IdentityID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	c.State = after
	c.UpdatedAt = now
	c.LastOperationID = in.OperationID
	if err = checked(ctx, tx, `UPDATE cash_vault_counts SET state=?,counted_cents=?,difference_cents=?,reviewer_id=?,updated_at=?,last_operation_id=? WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND state=?`, after, c.CountedCents, c.DifferenceCents, c.ReviewerID, now, in.OperationID, a.TenantID, a.StoreID, d.DeviceID, id, before); err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, a, d, c, Event{in.OperationID, a.IdentityID, action, before, after, in.Reason, now}, h)
}
