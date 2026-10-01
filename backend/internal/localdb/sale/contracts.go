package sale

import (
	"context"
	"database/sql"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// The store uses the same DB. Permission, device and POS entitlement are
// checked in the transaction of sale, payments, cash, stock and outbox.
func CompleteWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in Input) (Result, error) {
	return complete(ctx, db, actor, device, in, func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return contracts.RequireTx(ctx, tx, actor, device, identity.Sell, modules.POS)
	})
}
