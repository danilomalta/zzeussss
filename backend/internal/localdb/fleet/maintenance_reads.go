package fleet

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

func MaintenanceGet(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, id string) (MaintenanceRecord, error) {
	if db == nil || !maintenanceKind(kind) || !ID(id) {
		return MaintenanceRecord{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return MaintenanceRecord{}, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return MaintenanceRecord{}, e
	}
	out, e := maintenanceLoad(ctx, tx, a, kind, id)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func MaintenanceOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (MaintenanceRecord, error) {
	if db == nil || !ID(id) {
		return MaintenanceRecord{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return MaintenanceRecord{}, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return MaintenanceRecord{}, e
	}
	out, e := maintenanceReceipt(ctx, tx, a, d.DeviceID, id, true)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func MaintenanceList(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, state string, limit, offset int) (MaintenancePage, error) {
	out := MaintenancePage{Items: []MaintenanceRecord{}, Limit: limit, Offset: offset}
	if db == nil || !maintenanceKind(kind) || !oneOf(state, "", "active", "inactive") || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return out, e
	}
	where := `tenant_id=? AND store_id=? AND kind=?`
	args := []any{a.TenantID, a.StoreID, kind}
	if state != "" {
		where += ` AND state=?`
		args = append(args, state)
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM maintenance_registry WHERE `+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM maintenance_registry WHERE `+where+` ORDER BY id LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		r, e := maintenanceLoad(ctx, tx, a, kind, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, r)
	}
	return out, tx.Commit()
}
func MaintenanceRecordHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, id string, limit, offset int) (MaintenanceHistory, error) {
	out := MaintenanceHistory{Items: []MaintenanceHistoryItem{}, Limit: limit, Offset: offset}
	if db == nil || !maintenanceKind(kind) || !ID(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return out, e
	}
	current, e := maintenanceLoad(ctx, tx, a, kind, id)
	if e != nil {
		return out, e
	}
	out.Total = current.Revision
	rows, e := tx.QueryContext(ctx, `SELECT device_id,operation_id,request_json FROM maintenance_registry_operations WHERE tenant_id=? AND store_id=? AND kind=? AND id=? ORDER BY revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, kind, id, limit, offset)
	if e != nil {
		return out, e
	}
	type v struct{ device, op, raw string }
	entries := []v{}
	for rows.Next() {
		var x v
		if e = rows.Scan(&x.device, &x.op, &x.raw); e != nil {
			rows.Close()
			return out, e
		}
		entries = append(entries, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, x := range entries {
		r, e := maintenanceReceipt(ctx, tx, a, x.device, x.op, false)
		if e != nil {
			return out, e
		}
		var input struct {
			Input Input `json:"input"`
		}
		if json.Unmarshal([]byte(x.raw), &input) != nil {
			return out, ErrConflict
		}
		out.Items = append(out.Items, MaintenanceHistoryItem{input.Input.Action, input.Input.Reason, r})
	}
	return out, tx.Commit()
}

type MaintenanceCandidate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func MaintenanceCandidates(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) ([]MaintenanceCandidate, error) {
	out := []MaintenanceCandidate{}
	if db == nil {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	manager, e := authorize(ctx, tx, a, d, false)
	if e != nil {
		return out, e
	}
	if !manager {
		return out, identity.ErrDenied
	}
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM memberships WHERE tenant_id=?`, a.TenantID).Scan(&count); e != nil {
		return out, e
	}
	if count > 1000 {
		return out, ErrWorkshopLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT i.id,i.display_name,m.role FROM identities i JOIN memberships m ON m.identity_id=i.id WHERE m.tenant_id=? AND m.status='active' AND(m.role='owner' OR EXISTS(SELECT 1 FROM membership_stores s WHERE s.tenant_id=m.tenant_id AND s.identity_id=i.id AND s.store_id=?)) ORDER BY i.id`, a.TenantID, a.StoreID)
	if e != nil {
		return out, e
	}
	all := []MaintenanceCandidate{}
	for rows.Next() {
		var x MaintenanceCandidate
		if e = rows.Scan(&x.ID, &x.Name, &x.Role); e != nil {
			rows.Close()
			return out, e
		}
		all = append(all, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, x := range all {
		target := a
		target.IdentityID = x.ID
		if _, e = authorize(ctx, tx, target, d, false); e != nil {
			if errors.Is(e, identity.ErrDenied) {
				continue
			}
			return out, e
		}
		out = append(out, x)
	}
	return out, tx.Commit()
}
