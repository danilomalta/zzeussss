package fiscal

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type ProfilePage struct {
	Rows      []Profile `json:"rows"`
	HasMore   bool      `json:"has_more"`
	NextAfter string    `json:"next_after"`
}

// ListProfiles discovers only the latest immutable revisions in this owner scope.
// A saved draft is never evidence of an installed provider or fiscal authority.
func ListProfiles(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, after string, limit int) (ProfilePage, error) {
	out := ProfilePage{Rows: []Profile{}}
	if db == nil || license == nil {
		return out, entitlementstore.ErrNotInstalled
	}
	if limit < 1 || limit > 50 || after != "" && !idPattern.MatchString(after) {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return out, err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.ViewAccounting, modules.Fiscal); err != nil {
		return out, err
	}
	var role string
	if err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
		return out, err
	}
	if role != "owner" {
		return out, identity.ErrDenied
	}
	rows, err := tx.QueryContext(ctx, `SELECT profile_id FROM fiscal_profiles WHERE tenant_id=? AND store_id=? AND profile_id>? GROUP BY profile_id ORDER BY profile_id LIMIT ?`, a.TenantID, a.StoreID, after, limit+1)
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(ids) > limit {
		out.HasMore = true
		ids = ids[:limit]
		out.NextAfter = ids[len(ids)-1]
	}
	for _, id := range ids {
		p, err := profileTx(ctx, tx, a, id, 0)
		if err != nil {
			return ProfilePage{}, err
		}
		out.Rows = append(out.Rows, p)
	}
	return out, tx.Commit()
}
