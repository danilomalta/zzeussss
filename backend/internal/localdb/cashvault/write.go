package cashvault

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
	"titansystem-backend/internal/localdb/cashcustody"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
)

func hash(v any) string { b, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(b)) }
func replay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id, h string) (Result, bool, error) {
	var who, saved, body string
	err := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash,result_json FROM cash_vault_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &saved, &body)
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
	var r Result
	if json.Unmarshal([]byte(body), &r) != nil {
		return r, false, ErrInconsistent
	}
	r.Repeated = true
	return r, true, nil
}
func finish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, r Result, h string, before []Vault) (Result, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return Result{}, err
	}
	if err = checked(ctx, tx, `INSERT INTO cash_vault_operations(tenant_id,store_id,device_id,operation_id,actor_id,kind,reason,request_hash,result_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, a.IdentityID, r.Kind, r.Reason, h, string(body), r.CreatedAt); err != nil {
		return Result{}, err
	}
	for _, e := range r.Entries {
		if err = checked(ctx, tx, `INSERT INTO cash_vault_entries(tenant_id,store_id,device_id,id,vault_id,operation_id,kind,delta_cents,bag_id,cash_session_id,cash_movement_id,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, e.ID, e.VaultID, e.OperationID, e.Kind, e.DeltaCents, e.BagID, e.CashSessionID, e.CashMovementID, e.CreatedAt); err != nil {
			return Result{}, err
		}
		if e.Kind == "deposit" {
			if err = checked(ctx, tx, `INSERT INTO cash_vault_deposits(tenant_id,store_id,device_id,bag_id,vault_id,entry_id) VALUES(?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, *e.BagID, e.VaultID, e.ID); err != nil {
				return Result{}, err
			}
		}
	}
	for i, previous := range before {
		after := r.Vaults[i]
		if err = checked(ctx, tx, `UPDATE cash_vaults SET balance_cents=?,state=?,updated_at=?,last_operation_id=? WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND balance_cents=? AND state='active' AND last_operation_id=?`, after.BalanceCents, after.State, after.UpdatedAt, r.OperationID, a.TenantID, a.StoreID, d.DeviceID, after.ID, previous.BalanceCents, previous.LastOperationID); err != nil {
			return Result{}, err
		}
	}
	if err = checked(ctx, tx, `INSERT INTO cash_vault_events(tenant_id,store_id,device_id,operation_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,1,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, "cash.vault."+r.Kind, string(body), r.CreatedAt); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return r, nil
}
func Create(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in CreateInput) (Result, error) {
	if db == nil || !key(in.OperationID) || !key(in.VaultID) || !text(in.Name, 100) {
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
		Kind  string
		Input CreateInput
	}{"create", in})
	if r, ok, err := replay(ctx, tx, a, d, in.OperationID, h); ok || err != nil {
		if err == nil {
			err = tx.Commit()
		}
		return r, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	v := Vault{ID: in.VaultID, Name: in.Name, State: "active", CreatedBy: a.IdentityID, CreatedAt: now, UpdatedAt: now, CreationOperationID: in.OperationID, LastOperationID: in.OperationID}
	if err = checked(ctx, tx, `INSERT INTO cash_vaults(tenant_id,store_id,device_id,id,name,state,balance_cents,created_by,created_at,updated_at,creation_operation_id,last_operation_id) VALUES(?,?,?,?,?,'active',0,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, v.ID, v.Name, a.IdentityID, now, now, in.OperationID, in.OperationID); err != nil {
		return Result{}, err
	}
	r := Result{OperationID: in.OperationID, Kind: "create", CreatedAt: now, Vaults: []Vault{v}, Entries: []Entry{}}
	return finish(ctx, tx, a, d, r, h, nil)
}

func Apply(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, kind string, in Input) (Result, error) {
	if db == nil || !key(id) || !key(in.OperationID) {
		return Result{}, ErrInvalid
	}
	switch kind {
	case "deposit":
		if !key(in.BagID) || in.AmountCents != 0 || in.DestinationID != "" || in.CashSessionID != "" || in.Reason != "" {
			return Result{}, ErrInvalid
		}
	case "transfer":
		if !key(in.DestinationID) || in.DestinationID == id || in.AmountCents < 1 || in.AmountCents > maxSafe || !text(in.Reason, 500) || in.BagID != "" || in.CashSessionID != "" {
			return Result{}, ErrInvalid
		}
	case "supply":
		if !text(in.CashSessionID, 128) || len(in.CashSessionID) > 128 || in.AmountCents < 1 || in.AmountCents > maxSafe || !text(in.Reason, 500) || in.BagID != "" || in.DestinationID != "" {
			return Result{}, ErrInvalid
		}
	case "archive":
		if !text(in.Reason, 500) || in.AmountCents != 0 || in.DestinationID != "" || in.CashSessionID != "" || in.BagID != "" {
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
		VaultID, Kind string
		Input         Input
	}{id, kind, in})
	if r, ok, err := replay(ctx, tx, a, d, in.OperationID, h); ok || err != nil {
		if err == nil {
			err = tx.Commit()
		}
		return r, err
	}
	v, _, err := load(ctx, tx, a, d, id)
	if err != nil {
		return Result{}, err
	}
	if v.State != "active" {
		return Result{}, ErrConflict
	}
	if err = requireUnfrozen(ctx, tx, a, d, id); err != nil {
		return Result{}, err
	}
	before := []Vault{v}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r := Result{OperationID: in.OperationID, Kind: kind, Reason: in.Reason, CreatedAt: now, Vaults: []Vault{v}, Entries: []Entry{}}
	entry := func(vaultID, k string, delta int64) (Entry, error) {
		eid, err := localdb.NewID()
		return Entry{ID: eid, VaultID: vaultID, OperationID: in.OperationID, Kind: k, DeltaCents: delta, ActorID: a.IdentityID, Reason: in.Reason, CreatedAt: now}, err
	}
	switch kind {
	case "deposit":
		b, err := cashcustody.ReadTx(ctx, tx, a, d, in.BagID)
		if err != nil {
			return Result{}, err
		}
		if b.State != "received" || b.CountedCents == nil || b.ReceiverID == nil {
			return Result{}, ErrConflict
		}
		if *b.ReceiverID != a.IdentityID {
			return Result{}, identity.ErrDenied
		}
		if *b.CountedCents < 0 || *b.CountedCents > maxSafe-v.BalanceCents {
			return Result{}, ErrRange
		}
		e, err := entry(id, "deposit", *b.CountedCents)
		if err != nil {
			return Result{}, err
		}
		e.BagID = &in.BagID
		r.Entries = append(r.Entries, e)
		r.BagID = &in.BagID
		r.Vaults[0].BalanceCents += *b.CountedCents
	case "transfer":
		if v.BalanceCents < in.AmountCents {
			return Result{}, ErrConflict
		}
		to, _, err := load(ctx, tx, a, d, in.DestinationID)
		if err != nil {
			return Result{}, err
		}
		if to.State != "active" {
			return Result{}, ErrConflict
		}
		if err = requireUnfrozen(ctx, tx, a, d, to.ID); err != nil {
			return Result{}, err
		}
		if to.BalanceCents > maxSafe-in.AmountCents {
			return Result{}, ErrRange
		}
		out, err := entry(id, "transfer_out", -in.AmountCents)
		if err != nil {
			return Result{}, err
		}
		credit, err := entry(to.ID, "transfer_in", in.AmountCents)
		if err != nil {
			return Result{}, err
		}
		r.Entries = append(r.Entries, out, credit)
		before = append(before, to)
		r.Vaults = append(r.Vaults, to)
		r.Vaults[0].BalanceCents -= in.AmountCents
		r.Vaults[1].BalanceCents += in.AmountCents
	case "supply":
		if v.BalanceCents < in.AmountCents {
			return Result{}, ErrConflict
		}
		var existing int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_adjustments WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&existing); err != nil {
			return Result{}, err
		}
		if existing != 0 {
			return Result{}, ErrConflict
		}
		m, err := register.MoveTxWithContract(ctx, tx, contracts, a, d, register.MovementInput{SessionID: in.CashSessionID, OperationID: in.OperationID, Kind: "supply", AmountCents: in.AmountCents, Reason: in.Reason})
		if err != nil {
			return Result{}, err
		}
		if m.Repeated {
			return Result{}, ErrConflict
		}
		e, err := entry(id, "cash_supply", -in.AmountCents)
		if err != nil {
			return Result{}, err
		}
		e.CashSessionID = &in.CashSessionID
		e.CashMovementID = &m.MovementID
		r.Entries = append(r.Entries, e)
		r.CashMovementID = &m.MovementID
		r.Vaults[0].BalanceCents -= in.AmountCents
	case "archive":
		if v.BalanceCents != 0 {
			return Result{}, ErrConflict
		}
		r.Vaults[0].State = "archived"
	}
	for i := range r.Vaults {
		r.Vaults[i].UpdatedAt = now
		r.Vaults[i].LastOperationID = in.OperationID
	}
	return finish(ctx, tx, a, d, r, h, before)
}
