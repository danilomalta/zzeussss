package stocktaking

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

// Historical reads intentionally require current permission/device, but do not
// demand renewal of a module contract merely to recover an existing receipt.
func Get(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Detail, error) {
	if db == nil || !validID(id) {
		return Detail{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, a, d); err != nil {
		return Detail{}, err
	}
	s, err := load(ctx, tx, a, id)
	if err != nil {
		return Detail{}, err
	}
	lines, err := linesTx(ctx, tx, a, id)
	if err != nil {
		return Detail{}, err
	}
	return Detail{s, lines}, tx.Commit()
}
func Receipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, op string) (Result, error) {
	if db == nil || !validID(op) {
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, a, d); err != nil {
		return Result{}, err
	}
	var actor, body string
	err = tx.QueryRowContext(ctx, `SELECT actor_id,response_json FROM reviewed_inventory_receipts WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&actor, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	if err != nil {
		return Result{}, err
	}
	if actor != a.IdentityID {
		return Result{}, ErrNotFound
	}
	var r Result
	if err = json.Unmarshal([]byte(body), &r); err != nil {
		return r, err
	}
	return r, tx.Commit()
}
func pageOK(limit, offset int) bool {
	return limit >= 1 && limit <= 100 && offset >= 0 && offset <= 1000000
}
func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, status string, limit, offset int) (Page[Session], error) {
	out := Page[Session]{Items: []Session{}, Limit: limit, Offset: offset}
	if db == nil || !pageOK(limit, offset) {
		return out, ErrInvalid
	}
	switch status {
	case "", "open", "submitted", "approved", "rejected", "cancelled":
	default:
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, a, d); err != nil {
		return out, err
	}
	where := `tenant_id=? AND store_id=?`
	args := []any{a.TenantID, a.StoreID}
	if status != "" {
		where += ` AND status=?`
		args = append(args, status)
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM reviewed_inventory_sessions WHERE `+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	params := append(append([]any{}, args...), limit, offset)
	rows, err := tx.QueryContext(ctx, `SELECT id,location_id,title,status,revision,created_by,created_at,updated_at FROM reviewed_inventory_sessions WHERE `+where+` ORDER BY created_at DESC,id LIMIT ? OFFSET ?`, params...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var s Session
		if err = rows.Scan(&s.ID, &s.LocationID, &s.Title, &s.Status, &s.Revision, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func History(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (Page[Event], error) {
	out := Page[Event]{Items: []Event{}, Limit: limit, Offset: offset}
	if db == nil || !validID(id) || !pageOK(limit, offset) {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, a, d); err != nil {
		return out, err
	}
	if _, err = load(ctx, tx, a, id); err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM reviewed_inventory_events WHERE tenant_id=? AND store_id=? AND session_id=?`, a.TenantID, a.StoreID, id).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.revision,e.operation_id,e.actor_id,e.device_id,e.action,e.reason,e.occurred_at,r.request_json,r.response_json FROM reviewed_inventory_events e JOIN reviewed_inventory_receipts r ON r.tenant_id=e.tenant_id AND r.store_id=e.store_id AND r.device_id=e.device_id AND r.operation_id=e.operation_id WHERE e.tenant_id=? AND e.store_id=? AND e.session_id=? ORDER BY e.revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, id, limit, offset)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var e Event
		var request, result string
		if err = rows.Scan(&e.Revision, &e.OperationID, &e.ActorID, &e.DeviceID, &e.Action, &e.Reason, &e.OccurredAt, &request, &result); err != nil {
			rows.Close()
			return out, err
		}
		e.Request = json.RawMessage(request)
		e.Result = json.RawMessage(result)
		out.Items = append(out.Items, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
