package backup

import (
	"context"
	"database/sql"
	"time"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

// A snapshot cannot know revocations made after it was taken. Do not resurrect
// network trust silently. Only the disposable recovery copy is changed; original
// message/decision/operation rows and the source installation remain untouched.
func blockRestoredCompanyTrade(ctx context.Context, db *sql.DB, d identity.DeviceContext) error {
	var count int
	if e := db.QueryRowContext(ctx, `SELECT count(*) FROM company_trade_peers WHERE tenant_id=? AND store_id=?`, d.TenantID, d.StoreID).Scan(&count); e != nil {
		return e
	}
	if count == 0 {
		return nil
	}
	generation, e := localdb.NewID()
	if e != nil {
		return e
	}
	_, e = db.ExecContext(ctx, `INSERT INTO company_trade_recovery VALUES(?,?,?,1,'','',?) ON CONFLICT(tenant_id,store_id) DO UPDATE SET generation=excluded.generation,required=1,approved_by='',verification_reference='',updated_at=excluded.updated_at`, d.TenantID, d.StoreID, generation, time.Now().UTC().Format(time.RFC3339Nano))
	if e != nil {
		return e
	}
	var saved string
	var required int
	if e = db.QueryRowContext(ctx, `SELECT generation,required FROM company_trade_recovery WHERE tenant_id=? AND store_id=?`, d.TenantID, d.StoreID).Scan(&saved, &required); e != nil {
		return e
	}
	if saved != generation || required != 1 {
		return ErrBackup
	}
	return nil
}
