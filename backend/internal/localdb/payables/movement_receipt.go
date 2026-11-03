package payables

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
	"unicode/utf8"
)

// AuditedMovementReceipt recovers an existing source, never creating or linking
// money. It accepts the legacy movement ID alphabet and returns only the actor's
// own receipt after validating the same audit/journal used by supplier payments.
func AuditedMovementReceipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, operation string) (register.MovementResult, error) {
	if operation == "" || operation != strings.TrimSpace(operation) || len(operation) > 128 || !utf8.ValidString(operation) || strings.ContainsRune(operation, 0) {
		return register.MovementResult{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return register.MovementResult{}, e
	}
	defer tx.Rollback()
	var actor string
	e = tx.QueryRowContext(ctx, `SELECT actor_identity_id FROM cash_adjustments WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, operation).Scan(&actor)
	if errors.Is(e, sql.ErrNoRows) {
		return register.MovementResult{}, ErrNotFound
	}
	if e != nil {
		return register.MovementResult{}, e
	}
	if actor != a.IdentityID {
		return register.MovementResult{}, ErrNotFound
	}
	source, e := sourceCash(ctx, tx, a, d, operation)
	if e != nil {
		return register.MovementResult{}, e
	}
	var reason string
	e = tx.QueryRowContext(ctx, `SELECT reason FROM cash_adjustments WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, operation).Scan(&reason)
	if e != nil {
		return register.MovementResult{}, e
	}
	if strings.TrimSpace(reason) == "" || !utf8.ValidString(reason) || utf8.RuneCountInString(reason) > 500 || strings.ContainsRune(reason, 0) {
		return register.MovementResult{}, ErrInconsistent
	}
	result := register.MovementResult{SessionID: source.SessionID, OperationID: operation, Kind: source.Kind, AmountCents: source.Amount, Reason: reason, MovementID: source.MovementID, CreatedAt: source.CreatedAt}
	return result, tx.Commit()
}
