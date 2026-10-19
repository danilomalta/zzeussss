package cashvault

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/identity"
)

// InspectTx validates a vault in the caller's transaction without committing.
func InspectTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Vault, error) {
	if tx == nil || !key(id) {
		return Vault{}, ErrInvalid
	}
	if err := identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash); err != nil {
		return Vault{}, err
	}
	v, _, err := load(ctx, tx, a, d, id)
	return v, err
}
func requireUnfrozen(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) error {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_vault_counts WHERE tenant_id=? AND store_id=? AND device_id=? AND vault_id=? AND state IN ('counting','submitted')`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return ErrConflict
	}
	return nil
}
