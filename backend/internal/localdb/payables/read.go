package payables

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/identity"
)

func Get(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Payable, error) {
	if !key(id) {
		return Payable{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return Payable{}, e
	}
	defer tx.Rollback()
	p, e := load(ctx, tx, a, d, id)
	if e != nil {
		return p, e
	}
	_, summary, _, e := cashForDocument(ctx, tx, a, d, p)
	if e != nil {
		return Payable{}, e
	}
	p.Cash = &summary
	return p, tx.Commit()
}
func Receipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Result, error) {
	if !key(id) {
		return Result{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	// Scope and actor are checked before decoding any other actor's saved result.
	var who string
	e = tx.QueryRowContext(ctx, `SELECT actor_id FROM supplier_payable_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who)
	if e == sql.ErrNoRows {
		return Result{}, ErrNotFound
	}
	if e != nil {
		return Result{}, e
	}
	if who != a.IdentityID {
		return Result{}, ErrNotFound
	}
	r, _, e := operation(ctx, tx, a, d, id)
	if e != nil {
		return r, e
	}
	if _, e = load(ctx, tx, a, d, r.Payable.ID); e != nil {
		return Result{}, e
	}
	return r, tx.Commit()
}
func all(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) ([]Payable, error) {
	rows, e := tx.QueryContext(ctx, `SELECT id FROM supplier_payables WHERE tenant_id=? AND store_id=? AND device_id=? ORDER BY created_at,id LIMIT ?`, a.TenantID, a.StoreID, d.DeviceID, MaxDocuments+1)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(ids) > MaxDocuments {
		return nil, ErrRange
	}
	out := []Payable{}
	for _, id := range ids {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		p, e := load(ctx, tx, a, d, id)
		if e != nil {
			return nil, e
		}
		_, summary, _, e := cashForDocument(ctx, tx, a, d, p)
		if e != nil {
			return nil, e
		}
		p.Cash = &summary
		out = append(out, p)
	}
	return out, nil
}
func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, state string, limit, offset int) (Page, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 || (state != "" && state != "pending" && state != "approved" && state != "rejected" && state != "cancelled") {
		return Page{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return Page{}, e
	}
	defer tx.Rollback()
	items, e := all(ctx, tx, a, d)
	if e != nil {
		return Page{}, e
	}
	selected := []Payable{}
	for _, p := range items {
		if state == "" || p.State == state {
			selected = append(selected, p)
		}
	}
	r := Page{Items: []Payable{}, TotalCount: len(selected), Limit: limit, Offset: offset}
	if offset < len(selected) {
		end := offset + limit
		if end > len(selected) {
			end = len(selected)
		}
		r.Items = selected[offset:end]
	}
	r.HasMore = offset+len(r.Items) < r.TotalCount
	return r, tx.Commit()
}
