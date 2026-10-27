package cashcount

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/identity"
)

func readTx(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (*sql.Tx, error) {
	if db == nil {
		return nil, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}
func Get(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Count, error) {
	if !key(id) {
		return Count{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return Count{}, err
	}
	defer tx.Rollback()
	c, err := load(ctx, tx, a, d, id)
	if err != nil {
		return Count{}, err
	}
	if err = tx.Commit(); err != nil {
		return Count{}, err
	}
	return visible(c), nil
}
func Receipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Result, error) {
	if !key(id) {
		return Result{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	var actor string
	if err = tx.QueryRowContext(ctx, `SELECT actor_id FROM cash_vault_count_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&actor); err == sql.ErrNoRows {
		return Result{}, ErrNotFound
	} else if err != nil {
		return Result{}, err
	}
	if actor != a.IdentityID {
		return Result{}, ErrNotFound
	}
	r, _, err := readOperation(ctx, tx, a, d, id)
	if err != nil {
		return r, err
	}
	r.Count = visible(r.Count)
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return r, nil
}
func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, limit, offset int) (Page, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return Page{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback()
	p := Page{Items: []Count{}, Limit: limit, Offset: offset}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_vault_counts WHERE tenant_id=? AND store_id=? AND device_id=?`, a.TenantID, a.StoreID, d.DeviceID).Scan(&p.TotalCount); err != nil {
		return p, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM cash_vault_counts WHERE tenant_id=? AND store_id=? AND device_id=? ORDER BY rowid LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, d.DeviceID, limit, offset)
	if err != nil {
		return p, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return p, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, err
	}
	for _, id := range ids {
		c, err := load(ctx, tx, a, d, id)
		if err != nil {
			return Page{}, err
		}
		p.Items = append(p.Items, visible(c))
	}
	p.HasMore = int64(offset+len(p.Items)) < p.TotalCount
	if err = tx.Commit(); err != nil {
		return Page{}, err
	}
	return p, nil
}
