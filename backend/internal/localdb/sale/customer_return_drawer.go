package sale

import (
	"context"
	"database/sql"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

// No implicit current-drawer selection: legacy calls retain the original turn.
func resolveReturnDrawerTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, original, requested string) (string, error) {
	if requested == "" {
		return original, nil
	}
	if !planID(requested) {
		return "", ErrInvalid
	}
	if e := identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash); e != nil {
		return "", e
	}
	var owner string
	e := tx.QueryRowContext(ctx, `SELECT c.operator_id FROM cash_sessions c JOIN cash_session_operators o ON o.tenant_id=c.tenant_id AND o.store_id=c.store_id AND o.session_id=c.id AND o.identity_id=c.operator_id WHERE c.tenant_id=? AND c.store_id=? AND c.device_id=? AND c.id=? AND c.closed_at IS NULL`, a.TenantID, a.StoreID, d.DeviceID, requested).Scan(&owner)
	if errors.Is(e, sql.ErrNoRows) {
		return "", ErrCashClosed
	}
	if e != nil {
		return "", e
	}
	if owner != a.IdentityID {
		return "", ErrConflict
	}
	if requested != original {
		var count int
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_sessions c JOIN cash_closures z ON z.tenant_id=c.tenant_id AND z.store_id=c.store_id AND z.session_id=c.id AND z.closed_at=c.closed_at AND z.identity_id=c.operator_id WHERE c.tenant_id=? AND c.store_id=? AND c.device_id=? AND c.id=? AND c.closed_at IS NOT NULL`, a.TenantID, a.StoreID, d.DeviceID, original).Scan(&count)
		if e != nil {
			return "", e
		}
		if count != 1 {
			return "", ErrConflict
		}
	}
	return requested, nil
}
