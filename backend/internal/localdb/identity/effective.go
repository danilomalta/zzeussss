package identity

import (
	"context"
	"database/sql"
)

type policyReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// KnownPermission prevents arbitrary strings from becoming authorizations.
func KnownPermission(p Permission) bool {
	switch p {
	case ViewCatalog, Sell, ManageStock, ReviewDiscount, ManageStaff, ViewAccounting, ManageProduction, ViewOrders, ManageReplenishment, ManageCash, CancelSale:
		return true
	}
	return false
}

// Effective checks the store-specific policy. Denial wins over a personal or
// department grant AND over the legacy role. The owner cannot be locked out.
// A department is a group of people, not a row-level business data boundary.
func effective(ctx context.Context, q policyReader, scope Scope, role string, p Permission) (bool, error) {
	if !KnownPermission(p) {
		return false, nil
	}
	if role == "owner" {
		return true, nil
	}
	var denies, grants int
	err := q.QueryRowContext(ctx, `SELECT
 COALESCE(sum(effect='deny'),0),COALESCE(sum(effect='allow'),0) FROM access_rules r
 WHERE r.tenant_id=? AND r.store_id=? AND r.permission=? AND
 ((r.target_kind='member' AND r.target_id=?) OR
 (r.target_kind='department' AND EXISTS(SELECT 1 FROM access_department_members dm
 JOIN access_departments d ON d.tenant_id=dm.tenant_id AND d.store_id=dm.store_id AND d.id=dm.department_id
 WHERE dm.tenant_id=r.tenant_id AND dm.store_id=r.store_id AND dm.identity_id=?
 AND dm.department_id=r.target_id AND (d.status='active' OR r.effect='deny'))))`, scope.TenantID, scope.StoreID, string(p), scope.IdentityID, scope.IdentityID).Scan(&denies, &grants)
	if err != nil {
		return false, err
	}
	if denies > 0 {
		return false, nil
	}
	return grants > 0 || allowed(role, p), nil
}
