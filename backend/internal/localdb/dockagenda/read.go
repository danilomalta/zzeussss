package dockagenda

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

func readTx(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (*sql.Tx, error) {
	if db == nil {
		return nil, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, e
	}
	if e = authorize(ctx, tx, a, d); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func pageOK(limit, offset int) bool {
	return limit >= 1 && limit <= 100 && offset >= 0 && offset <= 1000000
}
func Docks(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, limit, offset int) (Page[Dock], error) {
	out := Page[Dock]{Items: []Dock{}, Limit: limit, Offset: offset}
	if !pageOK(limit, offset) {
		return out, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM dock_agenda_resources WHERE tenant_id=? AND store_id=?`, a.TenantID, a.StoreID).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM dock_agenda_resources WHERE tenant_id=? AND store_id=? ORDER BY id LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, limit, offset)
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
		r, e := dockTx(ctx, tx, a, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, r)
	}
	return out, tx.Commit()
}

type Detail struct {
	Appointment         Appointment `json:"appointment"`
	MeasuredAt          string      `json:"measured_at"`
	OrderCancelled      bool        `json:"order_cancelled"`
	ArrivalDelaySeconds *int64      `json:"arrival_delay_seconds"`
	WaitingSeconds      *int64      `json:"waiting_seconds"`
	UnloadingSeconds    *int64      `json:"unloading_seconds"`
}

func duration(from, to string) (int64, error) {
	a, e := time.Parse(time.RFC3339Nano, from)
	b, f := time.Parse(time.RFC3339Nano, to)
	if e != nil || f != nil || b.Before(a) {
		return 0, ErrConflict
	}
	return int64(b.Sub(a) / time.Second), nil
}
func Get(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Detail, error) {
	var out Detail
	if !validID(id) {
		return out, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	r, e := appointmentTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out.Appointment = r
	out.MeasuredAt = stamp()
	if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_order_cancellations WHERE tenant_id=? AND store_id=? AND order_id=?)`, a.TenantID, a.StoreID, r.OrderID).Scan(&out.OrderCancelled); e != nil {
		return out, e
	}
	if r.ArrivedAt != nil {
		arrived, e := time.Parse(time.RFC3339Nano, *r.ArrivedAt)
		if e != nil {
			return out, ErrConflict
		}
		start, _ := time.Parse(time.RFC3339, r.StartsAt)
		delay := int64(arrived.Sub(start) / time.Second)
		out.ArrivalDelaySeconds = &delay
		end := out.MeasuredAt
		if r.UnloadingAt != nil {
			end = *r.UnloadingAt
		} else if r.Status == "cancelled" {
			end = r.UpdatedAt
		}
		n, e := duration(*r.ArrivedAt, end)
		if e != nil {
			return out, e
		}
		out.WaitingSeconds = &n
	}
	if r.UnloadingAt != nil {
		end := out.MeasuredAt
		if r.CompletedAt != nil {
			end = *r.CompletedAt
		}
		n, e := duration(*r.UnloadingAt, end)
		if e != nil {
			return out, e
		}
		out.UnloadingSeconds = &n
	}
	return out, tx.Commit()
}
func Appointments(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in Window) (Page[Appointment], error) {
	out := Page[Appointment]{Items: []Appointment{}, Limit: in.Limit, Offset: in.Offset}
	from, e := time.Parse(time.RFC3339Nano, in.From)
	to, f := time.Parse(time.RFC3339Nano, in.To)
	if e != nil || f != nil || !from.Before(to) || to.Sub(from) > 31*24*time.Hour || from.Year() < 2000 || to.Year() > 2200 || from.Nanosecond() != 0 || to.Nanosecond() != 0 || !pageOK(in.Limit, in.Offset) || (in.DockID != "" && !validID(in.DockID)) {
		return out, ErrInvalid
	}
	switch in.Status {
	case "", "reserved", "arrived", "unloading", "completed", "cancelled", "no_show":
	default:
		return out, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	where := `tenant_id=? AND store_id=? AND starts_unix<? AND ends_unix>?`
	args := []any{a.TenantID, a.StoreID, to.Unix(), from.Unix()}
	if in.DockID != "" {
		where += ` AND dock_id=?`
		args = append(args, in.DockID)
	}
	if in.Status != "" {
		where += ` AND status=?`
		args = append(args, in.Status)
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM dock_agenda_appointments WHERE `+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	args = append(args, in.Limit, in.Offset)
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM dock_agenda_appointments WHERE `+where+` ORDER BY starts_unix,id LIMIT ? OFFSET ?`, args...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		r, e := scanAppointment(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func Receipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, op string) (Result, error) {
	var r Result
	if !validID(op) {
		return r, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	var raw string
	e = tx.QueryRowContext(ctx, `SELECT response_json FROM dock_agenda_receipts WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND actor_id=?`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal([]byte(raw), &r); e != nil {
		return r, e
	}
	return r, tx.Commit()
}
func History(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, id string, limit, offset int) (Page[Event], error) {
	out := Page[Event]{Items: []Event{}, Limit: limit, Offset: offset}
	if !validID(id) || !pageOK(limit, offset) || (kind != "dock" && kind != "appointment") {
		return out, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if kind == "dock" {
		_, e = dockTx(ctx, tx, a, id)
	} else {
		_, e = appointmentTx(ctx, tx, a, id)
	}
	if e != nil {
		return out, e
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM dock_agenda_events WHERE tenant_id=? AND store_id=? AND entity_kind=? AND entity_id=?`, a.TenantID, a.StoreID, kind, id).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT r.response_json,r.actor_id,r.device_id,r.request_json FROM dock_agenda_events e JOIN dock_agenda_receipts r ON r.tenant_id=e.tenant_id AND r.store_id=e.store_id AND r.device_id=e.device_id AND r.operation_id=e.operation_id WHERE e.tenant_id=? AND e.store_id=? AND e.entity_kind=? AND e.entity_id=? ORDER BY e.revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, kind, id, limit, offset)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var item Event
		var raw string
		if e = rows.Scan(&raw, &item.ActorID, &item.DeviceID, &item.Request); e != nil {
			rows.Close()
			return out, e
		}
		if e = json.Unmarshal([]byte(raw), &item.Result); e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}

type Occupancy struct {
	Dock         Dock         `json:"dock"`
	Unloading    *Appointment `json:"unloading"`
	WaitingCount int64        `json:"waiting_count"`
	MeasuredAt   string       `json:"measured_at"`
}

// Actual occupancy deliberately ignores planned dates: a late vehicle from
// yesterday must remain visible and keep blocking a second unloading today.
func Occupancies(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, limit, offset int) (Page[Occupancy], error) {
	out := Page[Occupancy]{Items: []Occupancy{}, Limit: limit, Offset: offset}
	if !pageOK(limit, offset) {
		return out, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM dock_agenda_resources WHERE tenant_id=? AND store_id=?`, a.TenantID, a.StoreID).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM dock_agenda_resources WHERE tenant_id=? AND store_id=? ORDER BY id LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, limit, offset)
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
	now := stamp()
	for _, id := range ids {
		dock, e := dockTx(ctx, tx, a, id)
		if e != nil {
			return out, e
		}
		r := Occupancy{Dock: dock, MeasuredAt: now}
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND dock_id=? AND status='arrived'`, a.TenantID, a.StoreID, id).Scan(&r.WaitingCount); e != nil {
			return out, e
		}
		var active string
		e = tx.QueryRowContext(ctx, `SELECT id FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND dock_id=? AND status='unloading'`, a.TenantID, a.StoreID, id).Scan(&active)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return out, e
		}
		if e == nil {
			v, e := appointmentTx(ctx, tx, a, active)
			if e != nil {
				return out, e
			}
			r.Unloading = &v
		}
		out.Items = append(out.Items, r)
	}
	return out, tx.Commit()
}
