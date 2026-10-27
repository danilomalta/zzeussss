package dockagenda

import (
	"context"
	"database/sql"
	"strings"
	"titansystem-backend/internal/localdb/identity"
)

// OrderSnapshotTx participates in the caller's authorized read snapshot.
// It never commits/rolls back the caller's transaction and never writes.
func OrderSnapshotTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string, max int) ([]Appointment, error) {
	// Preserve legacy purchase-order identifiers, which permit interior spaces.
	if tx == nil || len(id) < 1 || len(id) > 128 || strings.TrimSpace(id) != id || strings.ContainsAny(id, "\x00\r\n") || max < 1 || max > 5000 {
		return nil, ErrInvalid
	}
	if e := authorize(ctx, tx, a, d); e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY starts_unix,id LIMIT ?`, a.TenantID, a.StoreID, id, max+1)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Appointment{}
	for rows.Next() {
		if len(out) >= max {
			return nil, ErrInsightLimit
		}
		v, e := scanAppointment(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
