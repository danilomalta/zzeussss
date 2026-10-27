package fleet

import (
	"context"
	"database/sql"
	"strconv"
	"time"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type NoticeAck struct {
	NoticeID             int64  `json:"notice_id"`
	ClientAcknowledgedAt string `json:"client_acknowledged_at"`
}
type Notice struct {
	ID           int64  `json:"id"`
	RequestID    string `json:"request_id"`
	Revision     int64  `json:"revision"`
	RecipientID  string `json:"recipient_id"`
	CreatedAt    string `json:"created_at"`
	Acknowledged bool   `json:"acknowledged"`
}
type NoticePage struct {
	Items         []Notice `json:"items"`
	NextCursor    int64    `json:"next_cursor"`
	Limit         int      `json:"limit"`
	PushDelivered bool     `json:"push_delivered"`
}

func visibleNotice(ctx context.Context, tx *sql.Tx, a identity.Scope, manager bool, id int64) (bool, error) {
	var recipient, requester string
	e := tx.QueryRowContext(ctx, `SELECT COALESCE(n.recipient_id,''),r.created_by FROM fleet_notices n JOIN fleet_records r ON r.tenant_id=n.tenant_id AND r.store_id=n.store_id AND r.kind='request' AND r.id=n.request_id WHERE n.tenant_id=? AND n.store_id=? AND n.id=?`, a.TenantID, a.StoreID, id).Scan(&recipient, &requester)
	if e == sql.ErrNoRows {
		return false, ErrNotFound
	}
	return manager || recipient == "" || recipient == a.IdentityID || requester == a.IdentityID, e
}
func Notices(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, after int64, limit int) (NoticePage, error) {
	out := NoticePage{Items: []Notice{}, NextCursor: after, Limit: limit}
	if db == nil || after < 0 || limit < 1 || limit > 100 {
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
	rows, e := tx.QueryContext(ctx, `SELECT n.id,n.request_id,n.revision,COALESCE(n.recipient_id,''),n.created_at,EXISTS(SELECT 1 FROM fleet_notice_acks k WHERE k.tenant_id=n.tenant_id AND k.store_id=n.store_id AND k.notice_id=n.id AND k.identity_id=? AND k.device_id=?) FROM fleet_notices n JOIN fleet_records r ON r.tenant_id=n.tenant_id AND r.store_id=n.store_id AND r.kind='request' AND r.id=n.request_id WHERE n.tenant_id=? AND n.store_id=? AND n.id>? AND (? OR n.recipient_id IS NULL OR n.recipient_id=? OR r.created_by=?) ORDER BY n.id LIMIT ?`, a.IdentityID, d.DeviceID, a.TenantID, a.StoreID, after, manager, a.IdentityID, a.IdentityID, limit)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var v Notice
		if e = rows.Scan(&v.ID, &v.RequestID, &v.Revision, &v.RecipientID, &v.CreatedAt, &v.Acknowledged); e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, v)
		out.NextCursor = v.ID
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func AcknowledgeNotice(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id int64, in Input) (Result, error) {
	if id < 1 || in.ID != strconv.FormatInt(id, 10) || in.ExpectedRevision != 0 || in.Action != "ack" || decode(in.Data, &struct{}{}) != nil {
		return Result{}, ErrInvalid
	}
	in, raw, hash, e := prepare("notice_ack", in)
	if e != nil {
		return Result{}, e
	}
	tx, manager, e := begin(ctx, db, c, a, d, false, false)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if out, exists, e := replay(ctx, tx, a, d, in, hash); exists {
		if e == nil {
			e = tx.Commit()
		}
		return out, e
	}
	visible, e := visibleNotice(ctx, tx, a, manager, id)
	if e != nil {
		return Result{}, e
	}
	if !visible {
		return Result{}, identity.ErrDenied
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	out := Result{OperationID: in.OperationID, Ack: &NoticeAck{id, now}}
	if e = operation(ctx, tx, a, d, in, raw, hash, false, out, now); e != nil {
		return out, e
	}
	if e = one(ctx, tx, `INSERT INTO fleet_notice_acks VALUES(?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, id, a.IdentityID, d.DeviceID, in.OperationID, now); e != nil {
		return out, e
	}
	return out, tx.Commit()
}
