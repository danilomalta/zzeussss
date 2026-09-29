// Package register registra turnos de caixa no SQLite do dispositivo.
// O chamador deve entregar identidade humana e aparelho já autenticados.
package register

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrInvalid  = errors.New("dados do caixa inválidos")
	ErrConflict = errors.New("operação de caixa conflitante")
	ErrClosed   = errors.New("caixa inexistente ou fechado")
)

type OpenInput struct {
	SessionID    string
	OpeningCents int64
}

type CloseInput struct {
	SessionID     string
	OperationID   string
	DeclaredCents int64
}

type Result struct {
	SessionID       string
	Repeated        bool
	ExpectedCents   int64
	DifferenceCents int64
}

func Open(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in OpenInput) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponível")
	}
	in.SessionID = strings.TrimSpace(in.SessionID)
	if in.SessionID == "" || in.OpeningCents < 0 {
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.Sell); err != nil {
		return Result{}, err
	}
	var tenant, store, deviceID, operator string
	var opening int64
	var closed sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT tenant_id, store_id, device_id, operator_id, opening_cents, closed_at
		FROM cash_sessions WHERE tenant_id = ? AND id = ?`, actor.TenantID, in.SessionID).Scan(&tenant, &store, &deviceID, &operator, &opening, &closed)
	if err == nil {
		var bound string
		if err = tx.QueryRowContext(ctx, `SELECT identity_id FROM cash_session_operators WHERE tenant_id = ? AND session_id = ?`, tenant, in.SessionID).Scan(&bound); err != nil {
			return Result{}, err
		}
		if tenant != actor.TenantID || store != actor.StoreID || deviceID != device.DeviceID || operator != actor.IdentityID || bound != actor.IdentityID || opening != in.OpeningCents {
			return Result{}, ErrConflict
		}
		if closed.Valid {
			return Result{}, ErrClosed
		}
		if err = tx.Commit(); err != nil {
			return Result{}, err
		}
		return Result{SessionID: in.SessionID, Repeated: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	// Projeção de uma identidade já validada para a FK da tabela legada users.
	_, err = tx.ExecContext(ctx, `INSERT INTO users(tenant_id, id, display_name)
		SELECT ?, i.id, i.display_name FROM identities i WHERE i.id = ?
		ON CONFLICT(tenant_id, id) DO NOTHING`, actor.TenantID, actor.IdentityID)
	if err != nil {
		return Result{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO cash_sessions(tenant_id, store_id, device_id, id, operator_id, opened_at, opening_cents)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, actor.TenantID, actor.StoreID, device.DeviceID, in.SessionID, actor.IdentityID, now, in.OpeningCents)
	if err != nil {
		return Result{}, fmt.Errorf("abrir caixa (verifique se já existe turno aberto): %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cash_session_operators VALUES (?, ?, ?, ?)`, actor.TenantID, actor.StoreID, in.SessionID, actor.IdentityID)
	if err != nil {
		return Result{}, err
	}
	if err = event(ctx, tx, actor, device, in.SessionID, in.SessionID, "cash.open", struct {
		SessionID    string `json:"session_id"`
		OpeningCents int64  `json:"opening_cents"`
	}{in.SessionID, in.OpeningCents}, now); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{SessionID: in.SessionID}, nil
}

func Close(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in CloseInput) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponível")
	}
	in.SessionID, in.OperationID = strings.TrimSpace(in.SessionID), strings.TrimSpace(in.OperationID)
	if in.SessionID == "" || in.OperationID == "" || in.DeclaredCents < 0 {
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.Sell); err != nil {
		return Result{}, err
	}
	var store, deviceID, operator string
	var opening int64
	var closed sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT c.store_id, c.device_id, b.identity_id, c.opening_cents, c.closed_at
		FROM cash_sessions c JOIN cash_session_operators b ON b.tenant_id = c.tenant_id AND b.session_id = c.id
		WHERE c.tenant_id = ? AND c.id = ?`, actor.TenantID, in.SessionID).
		Scan(&store, &deviceID, &operator, &opening, &closed)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrClosed
	}
	if err != nil {
		return Result{}, err
	}
	if store != actor.StoreID || deviceID != device.DeviceID || operator != actor.IdentityID {
		return Result{}, identity.ErrDenied
	}
	if closed.Valid {
		var op, identityID string
		var declared, expected int64
		err = tx.QueryRowContext(ctx, `SELECT operation_id, identity_id, declared_cents, expected_cents
			FROM cash_closures WHERE tenant_id = ? AND session_id = ?`, actor.TenantID, in.SessionID).
			Scan(&op, &identityID, &declared, &expected)
		if err != nil {
			return Result{}, err
		}
		if op != in.OperationID || identityID != actor.IdentityID || declared != in.DeclaredCents {
			return Result{}, ErrConflict
		}
		if err = tx.Commit(); err != nil {
			return Result{}, err
		}
		return Result{SessionID: in.SessionID, Repeated: true, ExpectedCents: expected, DifferenceCents: declared - expected}, nil
	}
	var movement int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM cash_movements
		WHERE tenant_id = ? AND store_id = ? AND cash_session_id = ?`, actor.TenantID, actor.StoreID, in.SessionID).Scan(&movement); err != nil {
		return Result{}, err
	}
	if movement > 0 && opening > math.MaxInt64-movement {
		return Result{}, ErrInvalid
	}
	expected := opening + movement
	if expected < 0 {
		return Result{}, ErrInvalid
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `UPDATE cash_sessions SET closed_at = ? WHERE tenant_id = ? AND store_id = ? AND device_id = ? AND id = ? AND closed_at IS NULL`, now, actor.TenantID, actor.StoreID, device.DeviceID, in.SessionID)
	if err != nil {
		return Result{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cash_closures VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, actor.TenantID, actor.StoreID, in.SessionID, in.OperationID, actor.IdentityID, in.DeclaredCents, expected, now)
	if err != nil {
		return Result{}, err
	}
	if err = event(ctx, tx, actor, device, in.OperationID, in.SessionID, "cash.close", struct {
		SessionID     string `json:"session_id"`
		DeclaredCents int64  `json:"declared_cents"`
		ExpectedCents int64  `json:"expected_cents"`
	}{in.SessionID, in.DeclaredCents, expected}, now); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{SessionID: in.SessionID, ExpectedCents: expected, DifferenceCents: in.DeclaredCents - expected}, nil
}

func event(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, operationID, aggregateID, eventType string, data any, now string) error {
	payload, err := json.Marshal(struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		DeviceID string `json:"device_id"`
		ActorID  string `json:"actor_id"`
		Data     any    `json:"data"`
	}{actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, data})
	if err != nil {
		return err
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox(event_id, tenant_id, store_id, device_id, operation_id, aggregate_id, event_type, schema_version, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`, id, actor.TenantID, actor.StoreID, device.DeviceID, operationID, aggregateID, eventType, string(payload), now)
	return err
}
