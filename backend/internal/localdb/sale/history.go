package sale

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/identity"
)

type Summary struct {
	SaleID        string `json:"sale_id"`
	CashSessionID string `json:"cash_session_id"`
	Status        string `json:"status"`
	CommittedAt   string `json:"committed_at"`
	TotalCents    int64  `json:"total_cents"`
}

// History follows Read's scope: this operator, company, store and device.
// Authorized reads remain available after license expiration.
func History(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, limit, offset int) ([]Summary, error) {
	if db == nil || limit < 1 || limit > 50 || offset < 0 {
		return nil, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = authorizeLegacy(ctx, tx, actor, device); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT s.id,s.cash_session_id,s.status,s.committed_at,s.total_cents
 FROM sales s JOIN sale_operations o ON o.tenant_id=s.tenant_id AND o.sale_id=s.id
 WHERE s.tenant_id=? AND s.store_id=? AND s.device_id=? AND o.store_id=? AND o.device_id=? AND o.actor_identity_id=?
 ORDER BY s.committed_at DESC,s.id DESC LIMIT ? OFFSET ?`, actor.TenantID, actor.StoreID, device.DeviceID, actor.StoreID, device.DeviceID, actor.IdentityID, limit, offset)
	if err != nil {
		return nil, err
	}
	items := make([]Summary, 0)
	for rows.Next() {
		var item Summary
		if err = rows.Scan(&item.SaleID, &item.CashSessionID, &item.Status, &item.CommittedAt, &item.TotalCents); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}
