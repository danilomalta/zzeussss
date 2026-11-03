package cashvault

import (
	"context"
	"database/sql"
	"errors"

	"titansystem-backend/internal/localdb/cashcustody"
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
func Get(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Vault, error) {
	if !key(id) {
		return Vault{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return Vault{}, err
	}
	defer tx.Rollback()
	v, _, err := load(ctx, tx, a, d, id)
	if err != nil {
		return Vault{}, err
	}
	if err = tx.Commit(); err != nil {
		return Vault{}, err
	}
	return v, nil
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
	r, actor, err := operation(ctx, tx, a, d, id)
	if err != nil {
		return Result{}, err
	}
	if actor != a.IdentityID {
		return Result{}, ErrNotFound
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return r, nil
}
func bounds(limit, offset int) bool {
	return limit >= 1 && limit <= 100 && offset >= 0 && offset <= 1000000000
}
func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, limit, offset int) (Page, error) {
	if !bounds(limit, offset) {
		return Page{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback()
	p := Page{Items: []Vault{}, Limit: limit, Offset: offset}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_vaults WHERE tenant_id=? AND store_id=? AND device_id=?`, a.TenantID, a.StoreID, d.DeviceID).Scan(&p.TotalCount); err != nil {
		return p, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM cash_vaults WHERE tenant_id=? AND store_id=? AND device_id=? ORDER BY created_at,id LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, d.DeviceID, limit, offset)
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
		v, _, err := load(ctx, tx, a, d, id)
		if err != nil {
			return Page{}, err
		}
		p.Items = append(p.Items, v)
	}
	p.HasMore = int64(offset+len(p.Items)) < p.TotalCount
	if err = tx.Commit(); err != nil {
		return Page{}, err
	}
	return p, nil
}
func ReadStatement(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (Statement, error) {
	if !key(id) || !bounds(limit, offset) {
		return Statement{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return Statement{}, err
	}
	defer tx.Rollback()
	v, all, err := load(ctx, tx, a, d, id)
	if err != nil {
		return Statement{}, err
	}
	p := Statement{Vault: v, Items: []Entry{}, Limit: limit, Offset: offset, TotalCount: int64(len(all))}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	if offset < len(all) {
		p.Items = all[offset:end]
	}
	p.HasMore = int64(offset+len(p.Items)) < p.TotalCount
	if err = tx.Commit(); err != nil {
		return Statement{}, err
	}
	return p, nil
}

type PendingPage struct {
	Items      []cashcustody.Bag `json:"items"`
	TotalCount int64             `json:"total_count"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
	HasMore    bool              `json:"has_more"`
}

// Pending shows only received bags belonging to the current receiver. It never
// reveals declarations for bags that are awaiting a count by another manager.
func Pending(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, limit, offset int) (PendingPage, error) {
	if !bounds(limit, offset) {
		return PendingPage{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return PendingPage{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM cash_bags WHERE tenant_id=? AND store_id=? AND device_id=? AND receiver_id=? AND state='received' ORDER BY created_at,id`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID)
	if err != nil {
		return PendingPage{}, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return PendingPage{}, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return PendingPage{}, err
	}
	all := []cashcustody.Bag{}
	verified := map[string]bool{}
	for _, id := range ids {
		b, err := cashcustody.ReadTx(ctx, tx, a, d, id)
		if err != nil {
			return PendingPage{}, err
		}
		var vault, entryID string
		err = tx.QueryRowContext(ctx, `SELECT vault_id,entry_id FROM cash_vault_deposits WHERE tenant_id=? AND store_id=? AND device_id=? AND bag_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&vault, &entryID)
		if errors.Is(err, sql.ErrNoRows) {
			all = append(all, b)
			continue
		}
		if err != nil {
			return PendingPage{}, err
		}
		e, err := rawEntry(ctx, tx, a, d, entryID)
		if err != nil {
			return PendingPage{}, err
		}
		if e.BagID == nil || *e.BagID != id || e.VaultID != vault {
			return PendingPage{}, ErrInconsistent
		}
		if !verified[vault] {
			if _, _, err = load(ctx, tx, a, d, vault); err != nil {
				return PendingPage{}, err
			}
			verified[vault] = true
		}
	}
	p := PendingPage{Items: []cashcustody.Bag{}, TotalCount: int64(len(all)), Limit: limit, Offset: offset}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	if offset < len(all) {
		p.Items = all[offset:end]
	}
	p.HasMore = int64(offset+len(p.Items)) < p.TotalCount
	if err = tx.Commit(); err != nil {
		return PendingPage{}, err
	}
	return p, nil
}
