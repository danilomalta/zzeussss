package sale

import (
	"context"
	"database/sql"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
)

// The store uses the same DB. Permission, device and POS entitlement are
// checked in the transaction of sale, payments, cash, stock and outbox.
func CompleteWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in Input, token ...string) (Result, error) {
	return complete(ctx, db, actor, device, in, func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		if err := validateCheckoutSessionTx(ctx, tx, actor, device, token, len(in.TaxQuoteOperations) > 0); err != nil {
			return err
		}
		if len(in.TaxQuoteOperations) > 0 {
			if err := contracts.RequireTx(ctx, tx, actor, device, identity.Sell, modules.Fiscal); err != nil {
				return err
			}
		}
		return contracts.RequireTx(ctx, tx, actor, device, identity.Sell, modules.POS)
	})
}

// HTTP always supplies its authenticated session. Legacy trusted command callers
// may omit it where no tax quote requires it; no HTTP field can select this path.
func validateCheckoutSessionTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, tokens []string, required bool) error {
	if len(tokens) > 1 || required && len(tokens) != 1 {
		return ErrInvalid
	}
	if len(tokens) == 1 {
		return fiscal.ValidateSaleSessionTx(ctx, tx, actor, device, tokens[0])
	}
	return nil
}
