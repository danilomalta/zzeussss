package register

import (
	"context"
	"database/sql"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// Permission, device and POS contract share the transaction of cash and outbox.
// The store must use the same database. The caller resolves the human session.
func OpenWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in OpenInput) (Result, error) {
	return open(ctx, db, actor, device, in, contracted(contracts))
}

func CloseWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in CloseInput) (Result, error) {
	return close(ctx, db, actor, device, in, contracted(contracts))
}

func contracted(contracts *entitlementstore.Store) authorization {
	return func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return contracts.RequireTx(ctx, tx, actor, device, identity.Sell, modules.POS)
	}
}

type CurrentSession struct {
	SessionID string `json:"session_id"`
	OpenedAt  string `json:"opened_at"`
}

// Current returns only the actor's open turn on this device, without the
// expected balance. Authorized reads remain possible without a valid license.
func Current(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext) (*CurrentSession, error) {
	if db == nil {
		return nil, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = authorizeLegacy(ctx, tx, actor, device); err != nil {
		return nil, err
	}
	var result CurrentSession
	err = tx.QueryRowContext(ctx, `SELECT c.id,c.opened_at FROM cash_sessions c
		JOIN cash_session_operators b ON b.tenant_id=c.tenant_id AND b.session_id=c.id
		WHERE c.tenant_id=? AND c.store_id=? AND c.device_id=? AND b.identity_id=? AND c.closed_at IS NULL`,
		actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID).Scan(&result.SessionID, &result.OpenedAt)
	if err == sql.ErrNoRows {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &result, nil
}
