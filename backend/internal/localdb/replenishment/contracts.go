package replenishment

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

const MaxExact int64 = 9007199254740991

type replenishmentAuthorization func(context.Context, *sql.Tx, identity.Scope, identity.DeviceContext) error

func checkedExec(ctx context.Context, tx *sql.Tx, q string, args ...any) (sql.Result, error) {
	r, e := tx.ExecContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return nil, e
	}
	if n != 1 {
		return nil, ErrConflict
	}
	return r, nil
}
func contracted(store *entitlementstore.Store, permission identity.Permission) replenishmentAuthorization {
	return func(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) error {
		return store.RequireTx(ctx, tx, a, d, permission, modules.Orders)
	}
}
func SetPolicyWithContract(ctx context.Context, db *sql.DB, store *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in PolicyInput) (PolicyResult, error) {
	if len(in.OperationID) > 128 || len(in.ProductID) > 128 || in.MinimumMilli < 0 || in.TargetMilli > MaxExact {
		return PolicyResult{}, ErrInvalid
	}
	auth := contracted(store, identity.ManageReplenishment)
	return setPolicy(ctx, db, a, d, in, func(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) error {
		if e := auth(ctx, tx, a, d); e != nil {
			return e
		}
		var unit string
		e := tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, in.ProductID).Scan(&unit)
		if e == sql.ErrNoRows {
			return ErrInvalid
		}
		if e != nil {
			return e
		}
		if unit == "unit" && (in.MinimumMilli%1000 != 0 || in.TargetMilli%1000 != 0) {
			return ErrInvalid
		}
		return nil
	})
}
func SuggestWithContract(ctx context.Context, db *sql.DB, store *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in SuggestInput) (SuggestResult, error) {
	if len(in.OperationID) > 128 || len(in.ProductID) > 128 {
		return SuggestResult{}, ErrInvalid
	}
	return suggest(ctx, db, a, d, in, contracted(store, identity.ManageStock))
}
func ReviewWithContract(ctx context.Context, db *sql.DB, store *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in ReviewInput) (ReviewResult, error) {
	if len(in.OperationID) > 128 || len(in.SuggestionID) > 128 {
		return ReviewResult{}, ErrInvalid
	}
	return review(ctx, db, a, d, in, contracted(store, identity.ManageReplenishment))
}
