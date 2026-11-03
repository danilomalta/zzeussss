package cashvault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"

	"titansystem-backend/internal/localdb/cashcustody"
	"titansystem-backend/internal/localdb/identity"
)

func rawEntry(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Entry, error) {
	var e Entry
	var actor, reason sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT e.id,e.vault_id,e.operation_id,e.kind,e.delta_cents,e.bag_id,e.cash_session_id,e.cash_movement_id,o.actor_id,o.reason,e.created_at FROM cash_vault_entries e LEFT JOIN cash_vault_operations o ON o.tenant_id=e.tenant_id AND o.store_id=e.store_id AND o.device_id=e.device_id AND o.operation_id=e.operation_id WHERE e.tenant_id=? AND e.store_id=? AND e.device_id=? AND e.id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&e.ID, &e.VaultID, &e.OperationID, &e.Kind, &e.DeltaCents, &e.BagID, &e.CashSessionID, &e.CashMovementID, &actor, &reason, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrInconsistent
	}
	if err != nil {
		return e, err
	}
	if !actor.Valid {
		return e, ErrInconsistent
	}
	e.ActorID = actor.String
	e.Reason = reason.String
	return e, nil
}
func operation(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Result, string, error) {
	var r Result
	var body, actor, kind, at, reason string
	err := tx.QueryRowContext(ctx, `SELECT result_json,actor_id,kind,created_at,reason FROM cash_vault_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&body, &actor, &kind, &at, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return r, "", ErrNotFound
	}
	if err != nil {
		return r, "", err
	}
	if json.Unmarshal([]byte(body), &r) != nil || r.OperationID != id || r.Kind != kind || r.CreatedAt != at || r.Reason != reason || r.Repeated {
		return r, "", ErrInconsistent
	}
	valid := false
	switch kind {
	case "create":
		valid = len(r.Entries) == 0 && len(r.Vaults) == 1 && r.Vaults[0].BalanceCents == 0 && r.Vaults[0].State == "active" && r.Vaults[0].CreationOperationID == id
	case "archive":
		valid = len(r.Entries) == 0 && len(r.Vaults) == 1 && r.Vaults[0].BalanceCents == 0 && r.Vaults[0].State == "archived"
	case "deposit":
		valid = len(r.Entries) == 1 && len(r.Vaults) == 1 && r.Entries[0].Kind == "deposit" && r.BagID != nil && r.Entries[0].BagID != nil && *r.BagID == *r.Entries[0].BagID && r.Entries[0].DeltaCents >= 0
	case "supply":
		valid = len(r.Entries) == 1 && len(r.Vaults) == 1 && r.Entries[0].Kind == "cash_supply" && r.Entries[0].DeltaCents < 0 && r.CashMovementID != nil
	case "transfer":
		valid = len(r.Entries) == 2 && len(r.Vaults) == 2 && r.Entries[0].Kind == "transfer_out" && r.Entries[1].Kind == "transfer_in" && r.Entries[0].DeltaCents < 0 && r.Entries[1].DeltaCents > 0 && r.Entries[0].DeltaCents == -r.Entries[1].DeltaCents && r.Vaults[0].ID != r.Vaults[1].ID
	}
	if !valid {
		return r, "", ErrInconsistent
	}
	for _, v := range r.Vaults {
		if v.LastOperationID != id || v.UpdatedAt != at || v.BalanceCents < 0 || v.BalanceCents > maxSafe {
			return r, "", ErrInconsistent
		}
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_vault_entries WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&count); err != nil {
		return r, "", err
	}
	if count != len(r.Entries) {
		return r, "", ErrInconsistent
	}
	for i, e := range r.Entries {
		if e.OperationID != id || e.ActorID != actor || e.Reason != reason || e.CreatedAt != at || e.VaultID != r.Vaults[i].ID {
			return r, "", ErrInconsistent
		}
		actual, err := rawEntry(ctx, tx, a, d, e.ID)
		if err != nil {
			return r, "", err
		}
		if !reflect.DeepEqual(actual, e) {
			return r, "", ErrInconsistent
		}
	}
	var eventType, eventBody string
	if err = tx.QueryRowContext(ctx, `SELECT event_type,payload_json FROM cash_vault_events WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&eventType, &eventBody); errors.Is(err, sql.ErrNoRows) {
		return r, "", ErrInconsistent
	} else if err != nil {
		return r, "", err
	}
	if eventType != "cash.vault."+kind || eventBody != body {
		return r, "", ErrInconsistent
	}
	return r, actor, nil
}

func sourceEntry(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, e Entry) error {
	switch e.Kind {
	case "deposit":
		if e.BagID == nil {
			return ErrInconsistent
		}
		b, err := cashcustody.ReadTx(ctx, tx, a, d, *e.BagID)
		if err != nil {
			return ErrInconsistent
		}
		if b.State != "received" || b.CountedCents == nil || *b.CountedCents != e.DeltaCents || b.ReceiverID == nil || *b.ReceiverID != e.ActorID {
			return ErrInconsistent
		}
		var vault, id string
		if err = tx.QueryRowContext(ctx, `SELECT vault_id,entry_id FROM cash_vault_deposits WHERE tenant_id=? AND store_id=? AND device_id=? AND bag_id=?`, a.TenantID, a.StoreID, d.DeviceID, *e.BagID).Scan(&vault, &id); err != nil || vault != e.VaultID || id != e.ID {
			return ErrInconsistent
		}
	case "cash_supply":
		if e.CashSessionID == nil || e.CashMovementID == nil {
			return ErrInconsistent
		}
		var amount, posted int64
		var actor, kind, session, movement, journalActor, journalOp, adjustedAt, movedAt, postedAt, reason, cashReason string
		err := tx.QueryRowContext(ctx, `SELECT x.amount_cents,x.actor_identity_id,x.kind,x.session_id,x.movement_id,j.cash_delta_cents,j.actor_identity_id,j.operation_id,x.created_at,m.created_at,j.occurred_at,x.reason,m.reason FROM cash_adjustments x JOIN cash_movements m ON m.tenant_id=x.tenant_id AND m.store_id=x.store_id AND m.cash_session_id=x.session_id AND m.id=x.movement_id JOIN cash_journal_entries j ON j.tenant_id=x.tenant_id AND j.store_id=x.store_id AND j.device_id=x.device_id AND j.entry_id='movement:'||x.movement_id AND j.category='supply' AND j.cash_delta_cents=m.amount_cents WHERE x.tenant_id=? AND x.store_id=? AND x.device_id=? AND x.operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, e.OperationID).Scan(&amount, &actor, &kind, &session, &movement, &posted, &journalActor, &journalOp, &adjustedAt, &movedAt, &postedAt, &reason, &cashReason)
		if err != nil || kind != "supply" || amount != -e.DeltaCents || posted != amount || actor != e.ActorID || journalActor != actor || journalOp != e.OperationID || session != *e.CashSessionID || movement != *e.CashMovementID || adjustedAt != movedAt || movedAt != postedAt || reason != e.Reason || cashReason != "supply:"+e.Reason {
			return ErrInconsistent
		}
	case "transfer_in", "transfer_out": // The immutable operation validates both legs.
	default:
		return ErrInconsistent
	}
	return nil
}

func load(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Vault, []Entry, error) {
	var v Vault
	err := tx.QueryRowContext(ctx, `SELECT id,name,state,balance_cents,created_by,created_at,updated_at,creation_operation_id,last_operation_id FROM cash_vaults WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&v.ID, &v.Name, &v.State, &v.BalanceCents, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt, &v.CreationOperationID, &v.LastOperationID)
	if errors.Is(err, sql.ErrNoRows) {
		return v, nil, ErrNotFound
	}
	if err != nil {
		return v, nil, err
	}
	creation, who, err := operation(ctx, tx, a, d, v.CreationOperationID)
	if err != nil {
		return v, nil, ErrInconsistent
	}
	cv := creation.Vaults[0]
	if creation.Kind != "create" || cv.ID != v.ID || cv.Name != v.Name || cv.CreatedBy != v.CreatedBy || cv.CreatedAt != v.CreatedAt || who != v.CreatedBy {
		return v, nil, ErrInconsistent
	}
	last, _, err := operation(ctx, tx, a, d, v.LastOperationID)
	if err != nil {
		return v, nil, ErrInconsistent
	}
	found := false
	for _, after := range last.Vaults {
		if reflect.DeepEqual(after, v) {
			found = true
		}
	}
	if !found {
		return v, nil, ErrInconsistent
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM cash_vault_entries WHERE tenant_id=? AND store_id=? AND device_id=? AND vault_id=? ORDER BY sequence`, a.TenantID, a.StoreID, d.DeviceID, id)
	if err != nil {
		return v, nil, err
	}
	ids := []string{}
	for rows.Next() {
		var entryID string
		if err = rows.Scan(&entryID); err != nil {
			rows.Close()
			return v, nil, err
		}
		ids = append(ids, entryID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, nil, err
	}
	items := make([]Entry, 0, len(ids))
	balance := new(big.Int)
	seen := map[string]bool{}
	for _, entryID := range ids {
		e, err := rawEntry(ctx, tx, a, d, entryID)
		if err != nil {
			return v, nil, err
		}
		if !seen[e.OperationID] {
			if _, _, err = operation(ctx, tx, a, d, e.OperationID); err != nil {
				return v, nil, err
			}
			seen[e.OperationID] = true
		}
		if err = sourceEntry(ctx, tx, a, d, e); err != nil {
			return v, nil, err
		}
		balance.Add(balance, big.NewInt(e.DeltaCents))
		items = append(items, e)
	}
	if !balance.IsInt64() || balance.Cmp(big.NewInt(maxSafe)) > 0 {
		return v, nil, ErrRange
	}
	if balance.Sign() < 0 || balance.Int64() != v.BalanceCents {
		return v, nil, ErrInconsistent
	}
	return v, items, nil
}
