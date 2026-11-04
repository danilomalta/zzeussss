package retail

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitewrite"
)

type Resolution struct {
	OperationID string `json:"operation_id"`
	Applied     bool   `json:"applied"`
	Retired     bool   `json:"retired"`
}

// Reconcile reads the original result, or retires an absent command atomically.
// It never replays a request, changes points/cash or discards an applied effect.
func (s Service) Reconcile(ctx context.Context, op string, confirmed bool) (Resolution, error) {
	out := Resolution{OperationID: op}
	if s.DB == nil || s.Contracts == nil {
		return out, ErrInvalid
	}
	if !ID(op) || !confirmed {
		return out, ErrInvalid
	}
	e := sqlitewrite.Run(ctx, s.DB, ErrConflict, func(tx *sql.Tx) error {
		out = Resolution{OperationID: op}
		if e := s.AuthorizeRecovery(ctx, tx); e != nil {
			return e
		}
		var store, device, actor string
		e := tx.QueryRowContext(ctx, `SELECT store_id,device_id,actor_id FROM retail_operations WHERE tenant_id=? AND operation_id=?`, s.Actor.TenantID, op).Scan(&store, &device, &actor)
		if e == nil {
			if store != s.Actor.StoreID || device != s.Device.DeviceID || actor != s.Actor.IdentityID {
				return identity.ErrDenied
			}
			out.Applied = true
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		e = tx.QueryRowContext(ctx, `SELECT store_id,device_id,actor_id FROM retail_operation_fences WHERE tenant_id=? AND operation_id=? AND store_id=? AND device_id=? AND actor_id=?`, s.Actor.TenantID, op, s.Actor.StoreID, s.Device.DeviceID, s.Actor.IdentityID).Scan(&store, &device, &actor)
		if e == nil {
			if store != s.Actor.StoreID || device != s.Device.DeviceID || actor != s.Actor.IdentityID {
				return identity.ErrDenied
			}
			out.Retired = true
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO retail_operation_fences VALUES(?,?,?,?,?,?)`, s.Actor.TenantID, op, s.Actor.StoreID, s.Device.DeviceID, s.Actor.IdentityID, s.now().Format(time.RFC3339Nano))
		out.Retired = e == nil
		return e
	})
	return out, e
}
