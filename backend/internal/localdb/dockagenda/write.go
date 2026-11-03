package dockagenda

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func SaveDock(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in DockInput) (Result, error) {
	if !validID(in.DockID) || !valid(in.Name, 120, true) || in.ExpectedRevision < 0 || in.ExpectedRevision >= MaxExact {
		return Result{}, ErrInvalid
	}
	return write(ctx, db, c, a, d, in.OperationID, "dock", in.DockID, "save_dock", in, func(tx *sql.Tx, now string) (Result, error) {
		if e := manager(ctx, tx, a); e != nil {
			return Result{}, e
		}
		old, e := dockTx(ctx, tx, a, in.DockID)
		if e != nil && !errors.Is(e, ErrNotFound) {
			return Result{}, e
		}
		if errors.Is(e, ErrNotFound) {
			if in.ExpectedRevision != 0 {
				return Result{}, ErrConflict
			}
			e = execOne(ctx, tx, `INSERT INTO dock_agenda_resources VALUES(?,?,?,?,?,1,?,?)`, a.TenantID, a.StoreID, in.DockID, in.Name, in.Enabled, now, now)
		} else {
			if old.Revision != in.ExpectedRevision {
				return Result{}, ErrConflict
			}
			if !in.Enabled {
				var active bool
				e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND dock_id=? AND status IN('reserved','arrived','unloading'))`, a.TenantID, a.StoreID, in.DockID).Scan(&active)
				if e != nil {
					return Result{}, e
				}
				if active {
					return Result{}, ErrConflict
				}
			}
			e = execOne(ctx, tx, `UPDATE dock_agenda_resources SET name=?,enabled=?,revision=revision+1,updated_at=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=?`, in.Name, in.Enabled, now, a.TenantID, a.StoreID, in.DockID, in.ExpectedRevision)
		}
		if e != nil {
			return Result{}, e
		}
		r, e := dockTx(ctx, tx, a, in.DockID)
		return Result{Revision: r.Revision, Dock: &r}, e
	})
}
func ensureSlot(ctx context.Context, tx *sql.Tx, a identity.Scope, dock, id, order string, from, to int64) error {
	resource, e := dockTx(ctx, tx, a, dock)
	if e != nil {
		return e
	}
	if !resource.Enabled {
		return ErrConflict
	}
	var occupied bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND id<>? AND status IN('reserved','arrived','unloading') AND ((dock_id=? AND starts_unix<? AND ends_unix>?) OR order_id=?))`, a.TenantID, a.StoreID, id, dock, to, from, order).Scan(&occupied)
	if e != nil {
		return e
	}
	if occupied {
		return ErrOverlap
	}
	return nil
}
func Book(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in BookInput) (Result, error) {
	from, to, e := window(in.StartsAt, in.EndsAt)
	if e != nil || !validID(in.ID) || !validID(in.DockID) || !validID(in.OrderID) || !valid(in.Reason, 255, true) {
		return Result{}, ErrInvalid
	}
	in.StartsAt = from.Format(time.RFC3339)
	in.EndsAt = to.Format(time.RFC3339)
	return write(ctx, db, c, a, d, in.OperationID, "appointment", in.ID, "reserve", in, func(tx *sql.Tx, now string) (Result, error) {
		supplier, e := orderTx(ctx, tx, a, in.OrderID)
		if e != nil {
			return Result{}, e
		}
		if e = ensureSlot(ctx, tx, a, in.DockID, in.ID, in.OrderID, from.Unix(), to.Unix()); e != nil {
			return Result{}, e
		}
		e = execOne(ctx, tx, `INSERT INTO dock_agenda_appointments(tenant_id,store_id,id,dock_id,order_id,supplier_id,starts_unix,ends_unix,status,revision,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,'reserved',1,?,?,?)`, a.TenantID, a.StoreID, in.ID, in.DockID, in.OrderID, supplier, from.Unix(), to.Unix(), a.IdentityID, now, now)
		if e != nil {
			return Result{}, e
		}
		r, e := appointmentTx(ctx, tx, a, in.ID)
		return Result{Revision: r.Revision, Appointment: &r}, e
	})
}
func Act(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in ActionInput) (Result, error) {
	if !validID(id) || in.ExpectedRevision < 1 || in.ExpectedRevision >= MaxExact || !valid(in.Reason, 255, true) {
		return Result{}, ErrInvalid
	}
	var from, to time.Time
	if in.Action == "reschedule" {
		var e error
		from, to, e = window(in.StartsAt, in.EndsAt)
		if e != nil || !validID(in.DockID) {
			return Result{}, ErrInvalid
		}
		in.StartsAt = from.Format(time.RFC3339)
		in.EndsAt = to.Format(time.RFC3339)
	} else if in.DockID != "" || in.StartsAt != "" || in.EndsAt != "" {
		return Result{}, ErrInvalid
	}
	switch in.Action {
	case "reschedule", "arrive", "start", "complete", "cancel", "no_show":
	default:
		return Result{}, ErrInvalid
	}
	return write(ctx, db, c, a, d, in.OperationID, "appointment", id, in.Action, in, func(tx *sql.Tx, now string) (Result, error) {
		r, e := appointmentTx(ctx, tx, a, id)
		if e != nil {
			return Result{}, e
		}
		if r.Revision != in.ExpectedRevision {
			return Result{}, ErrConflict
		}
		beforeTime, parseError := time.Parse(time.RFC3339Nano, r.UpdatedAt)
		nowTime, _ := time.Parse(time.RFC3339Nano, now)
		if parseError != nil || nowTime.Before(beforeTime) {
			return Result{}, ErrConflict
		}
		next := r.Status
		dock := r.DockID
		start, _ := time.Parse(time.RFC3339, r.StartsAt)
		end, _ := time.Parse(time.RFC3339, r.EndsAt)
		arrived, unloading, completed := r.ArrivedAt, r.UnloadingAt, r.CompletedAt
		switch in.Action {
		case "reschedule":
			if r.Status != "reserved" {
				return Result{}, ErrConflict
			}
			dock = in.DockID
			start = from
			end = to
			if e = ensureSlot(ctx, tx, a, dock, id, r.OrderID, start.Unix(), end.Unix()); e != nil {
				return Result{}, e
			}
		case "arrive":
			if r.Status != "reserved" {
				return Result{}, ErrConflict
			}
			next = "arrived"
			arrived = &now
		case "start":
			if r.Status != "arrived" {
				return Result{}, ErrConflict
			}
			var active bool
			e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND dock_id=? AND status='unloading' AND id<>?)`, a.TenantID, a.StoreID, r.DockID, id).Scan(&active)
			if e != nil {
				return Result{}, e
			}
			if active {
				return Result{}, ErrOverlap
			}
			next = "unloading"
			unloading = &now
		case "complete":
			if r.Status != "unloading" {
				return Result{}, ErrConflict
			}
			next = "completed"
			completed = &now
		case "cancel":
			if r.Status != "reserved" && r.Status != "arrived" {
				return Result{}, ErrConflict
			}
			next = "cancelled"
		case "no_show":
			if r.Status != "reserved" || time.Now().Unix() < end.Unix() {
				return Result{}, ErrConflict
			}
			next = "no_show"
		}
		if in.Action == "reschedule" || in.Action == "arrive" || in.Action == "start" {
			supplier, e := orderTx(ctx, tx, a, r.OrderID)
			if e != nil {
				return Result{}, e
			}
			if supplier != r.SupplierID {
				return Result{}, ErrConflict
			}
		}
		e = execOne(ctx, tx, `UPDATE dock_agenda_appointments SET dock_id=?,starts_unix=?,ends_unix=?,status=?,revision=revision+1,updated_at=?,arrived_at=?,unloading_at=?,completed_at=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=?`, dock, start.Unix(), end.Unix(), next, now, arrived, unloading, completed, a.TenantID, a.StoreID, id, in.ExpectedRevision)
		if e != nil {
			return Result{}, e
		}
		after, e := appointmentTx(ctx, tx, a, id)
		return Result{Revision: after.Revision, Appointment: &after}, e
	})
}
