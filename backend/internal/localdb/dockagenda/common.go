package dockagenda

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"unicode/utf8"
)

func valid(s string, max int, spaces bool) bool {
	if len(s) < 1 || len(s) > max || strings.TrimSpace(s) != s || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 || (!spaces && r == ' ') {
			return false
		}
	}
	return true
}
func validID(s string) bool { return valid(s, 128, false) }
func window(from, to string) (time.Time, time.Time, error) {
	a, e := time.Parse(time.RFC3339Nano, from)
	b, f := time.Parse(time.RFC3339Nano, to)
	if e != nil || f != nil || a.Year() < 2000 || b.Year() > 2200 || a.Nanosecond() != 0 || b.Nanosecond() != 0 || b.Sub(a) < 5*time.Minute || b.Sub(a) > 8*time.Hour {
		return a, b, ErrInvalid
	}
	return a.UTC(), b.UTC(), nil
}
func authorize(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) error {
	for _, p := range []identity.Permission{identity.ManageStock, identity.ViewOrders} {
		if e := identity.CanOperateTx(ctx, tx, a, d, p); e != nil {
			return e
		}
	}
	return nil
}
func manager(ctx context.Context, tx *sql.Tx, a identity.Scope) error {
	var role string
	e := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role)
	if errors.Is(e, sql.ErrNoRows) {
		return identity.ErrDenied
	}
	if e != nil {
		return e
	}
	if role != "owner" && role != "manager" {
		return identity.ErrDenied
	}
	return nil
}
func execOne(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	result, e := tx.ExecContext(ctx, q, args...)
	if e != nil {
		if strings.Contains(e.Error(), "dock_agenda_overlap") {
			return ErrOverlap
		}
		if strings.Contains(e.Error(), "UNIQUE constraint failed: dock_agenda_") {
			return ErrConflict
		}
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func dockTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Dock, error) {
	var r Dock
	e := tx.QueryRowContext(ctx, `SELECT id,name,enabled,revision,created_at,updated_at FROM dock_agenda_resources WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&r.ID, &r.Name, &r.Enabled, &r.Revision, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if e != nil {
		return r, e
	}
	if !validID(r.ID) || !valid(r.Name, 120, true) || r.Revision < 1 || r.Revision > MaxExact {
		return r, ErrConflict
	}
	return r, nil
}

const columns = `id,dock_id,order_id,supplier_id,starts_unix,ends_unix,status,revision,created_by,created_at,updated_at,arrived_at,unloading_at,completed_at`

func scanAppointment(s scanner) (Appointment, error) {
	var r Appointment
	var from, to int64
	e := s.Scan(&r.ID, &r.DockID, &r.OrderID, &r.SupplierID, &from, &to, &r.Status, &r.Revision, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &r.ArrivedAt, &r.UnloadingAt, &r.CompletedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if e != nil {
		return r, e
	}
	r.StartsAt = time.Unix(from, 0).UTC().Format(time.RFC3339)
	r.EndsAt = time.Unix(to, 0).UTC().Format(time.RFC3339)
	if _, _, e = window(r.StartsAt, r.EndsAt); e != nil || !validID(r.ID) || !validID(r.DockID) || !validID(r.OrderID) || !validID(r.SupplierID) || r.Revision < 1 || r.Revision > MaxExact {
		return r, ErrConflict
	}
	switch r.Status {
	case "reserved", "arrived", "unloading", "completed", "cancelled", "no_show":
	default:
		return r, ErrConflict
	}
	for _, value := range []*string{&r.CreatedAt, &r.UpdatedAt, r.ArrivedAt, r.UnloadingAt, r.CompletedAt} {
		if value != nil {
			if _, err := time.Parse(time.RFC3339Nano, *value); err != nil {
				return r, ErrConflict
			}
		}
	}
	if (r.Status == "reserved" || r.Status == "no_show") && (r.ArrivedAt != nil || r.UnloadingAt != nil || r.CompletedAt != nil) {
		return r, ErrConflict
	}
	if r.Status == "arrived" && (r.ArrivedAt == nil || r.UnloadingAt != nil || r.CompletedAt != nil) {
		return r, ErrConflict
	}
	if r.Status == "unloading" && (r.ArrivedAt == nil || r.UnloadingAt == nil || r.CompletedAt != nil) {
		return r, ErrConflict
	}
	if r.Status == "completed" && (r.ArrivedAt == nil || r.UnloadingAt == nil || r.CompletedAt == nil) {
		return r, ErrConflict
	}
	if r.Status == "cancelled" && (r.UnloadingAt != nil || r.CompletedAt != nil) {
		return r, ErrConflict
	}
	return r, nil
}
func appointmentTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Appointment, error) {
	return scanAppointment(tx.QueryRowContext(ctx, `SELECT `+columns+` FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id))
}
func orderTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (string, error) {
	var supplier, status string
	var cancelled bool
	e := tx.QueryRowContext(ctx, `SELECT o.supplier_id,s.status,EXISTS(SELECT 1 FROM purchase_order_cancellations c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND c.order_id=o.id) FROM purchase_orders o JOIN purchase_suppliers s ON s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=o.supplier_id WHERE o.tenant_id=? AND o.store_id=? AND o.id=?`, a.TenantID, a.StoreID, id).Scan(&supplier, &status, &cancelled)
	if errors.Is(e, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if e != nil {
		return "", e
	}
	if status != "active" || cancelled {
		return "", ErrOrder
	}
	return supplier, nil
}
func write(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, op, kind, id, action string, in any, fn func(*sql.Tx, string) (Result, error)) (Result, error) {
	if db == nil || !validID(op) || !validID(id) {
		return Result{}, ErrInvalid
	}
	payload, e := json.Marshal(struct {
		Kind   string `json:"kind"`
		ID     string `json:"id"`
		Action string `json:"action"`
		Input  any    `json:"input"`
	}{kind, id, action, in})
	if e != nil {
		return Result{}, e
	}
	h := sha256.Sum256(payload)
	hash := hex.EncodeToString(h[:])
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if e = authorize(ctx, tx, a, d); e != nil {
		return Result{}, e
	}
	if kind == "dock" {
		if e = manager(ctx, tx, a); e != nil {
			return Result{}, e
		}
	}
	if e = contracts.RequireTx(ctx, tx, a, d, identity.ViewOrders, modules.Logistics); e != nil {
		return Result{}, e
	}
	var actor, savedHash, raw string
	e = tx.QueryRowContext(ctx, `SELECT actor_id,request_hash,response_json FROM dock_agenda_receipts WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&actor, &savedHash, &raw)
	if e == nil {
		if actor != a.IdentityID || savedHash != hash {
			return Result{}, ErrConflict
		}
		var r Result
		if e = json.Unmarshal([]byte(raw), &r); e != nil {
			return r, e
		}
		r.Repeated = true
		return r, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Result{}, e
	}
	now := stamp()
	r, e := fn(tx, now)
	if e != nil {
		return r, e
	}
	r.OperationID = op
	r.EntityKind = kind
	r.EntityID = id
	r.Action = action
	r.OccurredAt = now
	body, e := json.Marshal(r)
	if e != nil {
		return r, e
	}
	e = execOne(ctx, tx, `INSERT INTO dock_agenda_receipts VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID, hash, string(payload), string(body), now)
	if e != nil {
		return r, e
	}
	e = execOne(ctx, tx, `INSERT INTO dock_agenda_events VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, kind, id, r.Revision, action, d.DeviceID, op, a.IdentityID, now)
	if e != nil {
		return r, e
	}
	event, e := localdb.NewID()
	if e != nil {
		return r, e
	}
	// Local outbox evidence is not a transmission or supplier acknowledgement.
	envelope, e := json.Marshal(struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		DeviceID string `json:"device_id"`
		ActorID  string `json:"actor_id"`
		Result   Result `json:"result"`
	}{a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, r})
	if e != nil {
		return r, e
	}
	e = execOne(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'logistics.dock_agenda',1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, op, id, string(envelope), now)
	if e != nil {
		return r, e
	}
	return r, tx.Commit()
}
