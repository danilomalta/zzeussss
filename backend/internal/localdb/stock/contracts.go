package stock

import (
	"context"
	"database/sql"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// RecordWithContract verifies permission, device and inventory entitlement in
// the transaction that writes operation, movements and outbox. Caller must
// resolve the human session and device proof first. Store and db share a database.
func RecordWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in Input) (Result, error) {
	return record(ctx, db, actor, device, in, func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return contracts.RequireTx(ctx, tx, actor, device, identity.ManageStock, modules.Inventory)
	})
}

// RecordInTxWithContract writes the existing movement/outbox protocol inside a
// caller-owned transaction. It never commits or rolls back that transaction.
func RecordInTxWithContract(ctx context.Context, tx *sql.Tx, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in Input) (Result, error) {
	if tx == nil || contracts == nil {
		return Result{}, ErrInvalidOperation
	}
	return recordTx(ctx, tx, actor, device, in, func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return contracts.RequireTx(ctx, tx, actor, device, identity.ManageStock, modules.Inventory)
	})
}
