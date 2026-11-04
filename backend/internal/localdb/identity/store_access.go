package identity

import (
	"context"
	"database/sql"
	"errors"
)

// CanAccessStoreTx proves only the human's target-store authorization, not a
// session/device. Cross-store callers must first validate the real source session.
func CanAccessStoreTx(ctx context.Context, tx *sql.Tx, a Scope, p Permission) error {
	if tx == nil || a.TenantID == "" || a.StoreID == "" || a.IdentityID == "" {
		return ErrDenied
	}
	var role, status string
	e := tx.QueryRowContext(ctx, `SELECT m.role,m.status FROM memberships m JOIN stores s ON s.tenant_id=m.tenant_id WHERE m.tenant_id=? AND m.identity_id=? AND s.id=?`, a.TenantID, a.IdentityID, a.StoreID).Scan(&role, &status)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrDenied
	}
	if e != nil {
		return e
	}
	if status != "active" {
		return ErrDenied
	}
	if role != "owner" {
		var n int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM membership_stores WHERE tenant_id=? AND identity_id=? AND store_id=?`, a.TenantID, a.IdentityID, a.StoreID).Scan(&n); e != nil {
			return e
		}
		if n != 1 {
			return ErrDenied
		}
	}
	allowed, e := effective(ctx, tx, a, role, p)
	if e != nil {
		return e
	}
	if !allowed {
		return ErrDenied
	}
	return nil
}
