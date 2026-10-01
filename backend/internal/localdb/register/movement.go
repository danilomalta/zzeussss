package register

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var ErrFunds = errors.New("saldo do caixa insuficiente ou fora do limite")

type MovementInput struct {
	SessionID   string `json:"session_id"`
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
}

type MovementResult struct {
	SessionID   string `json:"session_id"`
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
	MovementID  string `json:"movement_id"`
	CreatedAt   string `json:"created_at"`
	Repeated    bool   `json:"repeated"`
}

// MoveWithContract changes cash and its audit/outbox together. It has no
// permission-only fallback and never reveals the expected drawer balance.
func MoveWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in MovementInput) (MovementResult, error) {
	var empty MovementResult
	validID := func(id string) bool {
		return id != "" && id == strings.TrimSpace(id) && len(id) <= 128 && !strings.ContainsRune(id, 0) && utf8.ValidString(id)
	}
	if db == nil || !validID(in.SessionID) || !validID(in.OperationID) || in.AmountCents <= 0 ||
		(in.Kind != "withdrawal" && in.Kind != "supply") || !utf8.ValidString(in.Reason) ||
		strings.TrimSpace(in.Reason) == "" || utf8.RuneCountInString(in.Reason) > 500 || strings.ContainsRune(in.Reason, 0) {
		return empty, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, actor, device, identity.ManageCash, modules.POS); err != nil {
		return empty, err
	}
	var previous MovementResult
	var previousActor string
	err = tx.QueryRowContext(ctx, `SELECT session_id,operation_id,kind,amount_cents,reason,movement_id,created_at,actor_identity_id
  FROM cash_adjustments WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`,
		actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID).Scan(&previous.SessionID, &previous.OperationID, &previous.Kind, &previous.AmountCents, &previous.Reason, &previous.MovementID, &previous.CreatedAt, &previousActor)
	if err == nil {
		if previousActor != actor.IdentityID || previous.SessionID != in.SessionID || previous.Kind != in.Kind || previous.AmountCents != in.AmountCents || previous.Reason != in.Reason {
			return empty, ErrConflict
		}
		if err = tx.Commit(); err != nil {
			return empty, err
		}
		previous.Repeated = true
		return previous, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return empty, err
	}
	var opening int64
	var closed sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT opening_cents,closed_at FROM cash_sessions WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, actor.TenantID, actor.StoreID, device.DeviceID, in.SessionID).Scan(&opening, &closed)
	if errors.Is(err, sql.ErrNoRows) {
		return empty, ErrClosed
	}
	if err != nil {
		return empty, err
	}
	if closed.Valid {
		return empty, ErrClosed
	}
	// Summing individual movements also detects overflow before SQLite SUM fails.
	rows, err := tx.QueryContext(ctx, `SELECT amount_cents FROM cash_movements WHERE tenant_id=? AND store_id=? AND cash_session_id=? ORDER BY created_at,id`, actor.TenantID, actor.StoreID, in.SessionID)
	if err != nil {
		return empty, err
	}
	balance := opening
	for rows.Next() {
		var delta int64
		if err = rows.Scan(&delta); err != nil {
			rows.Close()
			return empty, err
		}
		if (delta > 0 && balance > math.MaxInt64-delta) || (delta < 0 && balance < math.MinInt64-delta) {
			rows.Close()
			return empty, ErrFunds
		}
		balance += delta
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return empty, err
	}
	if balance < 0 || (in.Kind == "withdrawal" && in.AmountCents > balance) || (in.Kind == "supply" && balance > math.MaxInt64-in.AmountCents) {
		return empty, ErrFunds
	}
	id, err := localdb.NewID()
	if err != nil {
		return empty, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	signed := in.AmountCents
	if in.Kind == "withdrawal" {
		signed = -signed
	}
	write := func(query string, args ...any) error {
		r, e := tx.ExecContext(ctx, query, args...)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return ErrConflict
		}
		return nil
	}
	if err = write(`INSERT INTO cash_movements(id,tenant_id,store_id,cash_session_id,amount_cents,reason,created_at) VALUES(?,?,?,?,?,?,?)`, id, actor.TenantID, actor.StoreID, in.SessionID, signed, in.Kind+":"+in.Reason, now); err != nil {
		return empty, err
	}
	if err = write(`INSERT INTO cash_adjustments(tenant_id,store_id,device_id,operation_id,session_id,actor_identity_id,kind,amount_cents,reason,movement_id,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.SessionID, actor.IdentityID, in.Kind, in.AmountCents, in.Reason, id, now); err != nil {
		return empty, err
	}
	payload, err := json.Marshal(struct {
		TenantID   string        `json:"tenant_id"`
		StoreID    string        `json:"store_id"`
		DeviceID   string        `json:"device_id"`
		ActorID    string        `json:"actor_id"`
		Input      MovementInput `json:"data"`
		MovementID string        `json:"movement_id"`
		CreatedAt  string        `json:"created_at"`
	}{actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, in, id, now})
	if err != nil {
		return empty, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return empty, err
	}
	if err = write(`INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'cash.movement',1,?,?)`, eventID, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.SessionID, string(payload), now); err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return MovementResult{SessionID: in.SessionID, OperationID: in.OperationID, Kind: in.Kind, AmountCents: in.AmountCents, Reason: in.Reason, MovementID: id, CreatedAt: now}, nil
}
