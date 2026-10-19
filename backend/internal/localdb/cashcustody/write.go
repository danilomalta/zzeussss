package cashcustody

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func hash(v any) string { body, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(body)) }
func previous(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, requestHash string) (Result, bool, error) {
	var who, saved, body string
	err := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash,result_json FROM cash_bag_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&who, &saved, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if who != a.IdentityID {
		return Result{}, false, ErrNotFound
	}
	if saved != requestHash {
		return Result{}, false, ErrConflict
	}
	var r Result
	if err = json.Unmarshal([]byte(body), &r); err != nil {
		return r, false, ErrInconsistent
	}
	r.Repeated = true
	return r, true, nil
}
func finish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, b Bag, e Event, requestHash string) (Result, error) {
	b.History = append(b.History, e)
	if err := validateHistory(b); err != nil {
		return Result{}, err
	}
	r := Result{OperationID: e.OperationID, Bag: visible(b, a.IdentityID)}
	body, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	if err = checked(ctx, tx, `INSERT INTO cash_bag_operations(tenant_id,store_id,device_id,operation_id,bag_id,actor_id,action,from_state,to_state,reason,request_hash,result_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, e.OperationID, b.ID, a.IdentityID, e.Action, e.FromState, e.ToState, e.Reason, requestHash, string(body), e.CreatedAt); err != nil {
		return Result{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return Result{}, err
	}
	payload, err := json.Marshal(struct {
		Bag   Bag   `json:"bag"`
		Event Event `json:"event"`
	}{b, e})
	if err != nil {
		return Result{}, err
	}
	if err = checked(ctx, tx, `INSERT INTO cash_bag_outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,1,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, e.OperationID, b.ID, "cash.bag."+e.Action, string(payload), e.CreatedAt); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return r, nil
}
func validateHistory(b Bag) error {
	if len(b.History) < 1 || len(b.History) > 3 {
		return ErrInconsistent
	}
	state := ""
	for i, e := range b.History {
		if e.FromState != state {
			return ErrInconsistent
		}
		valid := false
		switch e.Action {
		case "create":
			valid = i == 0 && state == "" && e.ToState == "prepared" && e.ActorID == b.SenderID
		case "dispatch":
			valid = state == "prepared" && e.ToState == "in_transit" && e.ActorID == b.SenderID
		case "cancel":
			valid = state == "prepared" && e.ToState == "cancelled" && e.ActorID == b.SenderID
		case "receive":
			valid = state == "in_transit" && e.ToState == "received" && e.ActorID != b.SenderID && e.ActorID != b.OperatorID && b.ReceiverID != nil && *b.ReceiverID == e.ActorID
		case "reject":
			valid = state == "in_transit" && e.ToState == "rejected" && e.ActorID != b.SenderID && e.ActorID != b.OperatorID
		}
		if !valid {
			return ErrInconsistent
		}
		state = e.ToState
	}
	if state != b.State || b.History[len(b.History)-1].CreatedAt != b.UpdatedAt {
		return ErrInconsistent
	}
	return nil
}

func Create(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in CreateInput) (Result, error) {
	if db == nil || !pathID(in.OperationID) || !pathID(in.BagID) || !validID(in.WithdrawalOperationID) || !validID(in.SealID) {
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
	requestHash := hash(struct {
		Action string
		Input  CreateInput
	}{"create", in})
	if r, ok, err := previous(ctx, tx, a, d, in.OperationID, requestHash); ok || err != nil {
		if err == nil {
			err = tx.Commit()
		}
		return r, err
	}
	o, err := source(ctx, tx, a, d, in.WithdrawalOperationID)
	if err != nil {
		return Result{}, err
	}
	if o.ActorID != a.IdentityID {
		return Result{}, ErrNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = checked(ctx, tx, `INSERT INTO cash_bags(tenant_id,store_id,device_id,id,withdrawal_operation_id,movement_id,session_id,operator_id,sender_id,seal_id,declared_cents,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,'prepared',?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.BagID, in.WithdrawalOperationID, o.MovementID, o.SessionID, o.OperatorID, a.IdentityID, in.SealID, o.Amount, now, now); err != nil {
		return Result{}, err
	}
	b, err := load(ctx, tx, a, d, in.BagID)
	if err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, a, d, b, Event{OperationID: in.OperationID, ActorID: a.IdentityID, Action: "create", ToState: "prepared", CreatedAt: now}, requestHash)
}

func Transition(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, action string, in ActionInput) (Result, error) {
	if db == nil || !pathID(id) || !pathID(in.OperationID) {
		return Result{}, ErrInvalid
	}
	switch action {
	case "dispatch":
		if in.SealID != "" || in.Reason != "" || in.CountedCents != 0 {
			return Result{}, ErrInvalid
		}
	case "cancel", "reject":
		if !validReason(in.Reason) || in.SealID != "" || in.CountedCents != 0 {
			return Result{}, ErrInvalid
		}
	case "receive":
		if !validID(in.SealID) || in.Reason != "" || in.CountedCents < 0 || in.CountedCents > maxSafe {
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
	requestHash := hash(struct {
		ID, Action string
		Input      ActionInput
	}{id, action, in})
	if r, ok, err := previous(ctx, tx, a, d, in.OperationID, requestHash); ok || err != nil {
		if err == nil {
			err = tx.Commit()
		}
		return r, err
	}
	b, err := load(ctx, tx, a, d, id)
	if err != nil {
		return Result{}, err
	}
	if err = validateHistory(b); err != nil {
		return Result{}, err
	}
	before := b.State
	after := ""
	switch action {
	case "dispatch", "cancel":
		if a.IdentityID != b.SenderID {
			return Result{}, identity.ErrDenied
		}
		if before != "prepared" {
			return Result{}, ErrConflict
		}
		after = "in_transit"
		if action == "cancel" {
			after = "cancelled"
		}
	case "receive", "reject":
		if a.IdentityID == b.SenderID || a.IdentityID == b.OperatorID {
			return Result{}, identity.ErrDenied
		}
		if before != "in_transit" {
			return Result{}, ErrConflict
		}
		after = "received"
		if action == "reject" {
			after = "rejected"
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if action == "receive" {
		if in.SealID != b.SealID {
			return Result{}, ErrConflict
		}
		if err = checked(ctx, tx, `UPDATE cash_bags SET state='received',counted_cents=?,difference_cents=?,receiver_id=?,received_at=?,updated_at=? WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND state='in_transit'`, in.CountedCents, in.CountedCents-*b.DeclaredCents, a.IdentityID, now, now, a.TenantID, a.StoreID, d.DeviceID, id); err != nil {
			return Result{}, err
		}
	} else {
		if err = checked(ctx, tx, `UPDATE cash_bags SET state=?,updated_at=? WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND state=?`, after, now, a.TenantID, a.StoreID, d.DeviceID, id, before); err != nil {
			return Result{}, err
		}
	}
	b, err = load(ctx, tx, a, d, id)
	if err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, a, d, b, Event{OperationID: in.OperationID, ActorID: a.IdentityID, Action: action, FromState: before, ToState: after, Reason: in.Reason, CreatedAt: now}, requestHash)
}
