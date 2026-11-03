package sale

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// CompleteCreditTx is called only inside the receivable's immediate transaction.
// It creates no payment, no cash movement and never invokes a fiscal emitter.
func CompleteCreditTx(ctx context.Context, tx *sql.Tx, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in Input) (Result, error) {
	if len(in.Payments) != 1 || in.Payments[0].Method != "receivable" {
		return Result{}, ErrInvalid
	}
	return completeTxMode(ctx, tx, a, d, in, func(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) error {
		return c.RequireTx(ctx, tx, a, d, identity.Sell, modules.POS)
	}, nil, true)
}
