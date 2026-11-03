package cashcustody

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"titansystem-backend/internal/localdb/identity"
)

func Read(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Bag, error) {
	if db == nil || !pathID(id) {
		return Bag{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Bag{}, err
	}
	defer tx.Rollback()
	b, err := ReadTx(ctx, tx, a, d, id)
	if err != nil {
		return Bag{}, err
	}
	if err = tx.Commit(); err != nil {
		return Bag{}, err
	}
	return b, nil
}

// ReadTx validates the bag within a caller-owned transaction and never commits.
func ReadTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Bag, error) {
	if tx == nil || !pathID(id) {
		return Bag{}, ErrInvalid
	}
	if err := identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash); err != nil {
		return Bag{}, err
	}
	b, err := load(ctx, tx, a, d, id)
	if err != nil {
		return Bag{}, err
	}
	if err = validateHistory(b); err != nil {
		return Bag{}, err
	}
	return visible(b, a.IdentityID), nil
}

// Receipt returns the original operation snapshot, not the bag's latest state.
func Receipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Result, error) {
	if db == nil || !pathID(id) {
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash); err != nil {
		return Result{}, err
	}
	var body string
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM cash_bag_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND actor_id=?`, a.TenantID, a.StoreID, d.DeviceID, id, a.IdentityID).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	var r Result
	if err = json.Unmarshal([]byte(body), &r); err != nil {
		return r, ErrInconsistent
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return r, nil
}
func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in ListInput) (Page, error) {
	if db == nil || in.Limit < 1 || in.Limit > 100 || in.Offset < 0 || in.Offset > 1000000000 || (in.State != "" && !validState(in.State)) || (in.SessionID != "" && !validID(in.SessionID)) {
		return Page{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash); err != nil {
		return Page{}, err
	}
	q := ` FROM cash_bags WHERE tenant_id=? AND store_id=? AND device_id=?`
	args := []any{a.TenantID, a.StoreID, d.DeviceID}
	if in.State != "" {
		q += ` AND state=?`
		args = append(args, in.State)
	}
	if in.SessionID != "" {
		q += ` AND session_id=?`
		args = append(args, in.SessionID)
	}
	p := Page{Items: make([]Bag, 0), Limit: in.Limit, Offset: in.Offset}
	if err = tx.QueryRowContext(ctx, `SELECT count(*)`+q, args...).Scan(&p.TotalCount); err != nil {
		return p, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id`+q+` ORDER BY created_at,id LIMIT ? OFFSET ?`, append(args, in.Limit, in.Offset)...)
	if err != nil {
		return p, err
	}
	ids := make([]string, 0)
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
		b, err := load(ctx, tx, a, d, id)
		if err != nil {
			return Page{}, err
		}
		if err = validateHistory(b); err != nil {
			return Page{}, err
		}
		p.Items = append(p.Items, visible(b, a.IdentityID))
	}
	p.HasMore = int64(in.Offset+len(p.Items)) < p.TotalCount
	if err = tx.Commit(); err != nil {
		return Page{}, err
	}
	return p, nil
}
