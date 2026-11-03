package supplycycle

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
)

type TradeMessageRecord struct {
	PeerID    string        `json:"peer_id"`
	Direction string        `json:"direction"`
	Message   TradeMessage  `json:"message"`
	Envelope  TradeEnvelope `json:"envelope"`
}
type TradeMessagePage struct {
	Items   []TradeMessageRecord `json:"items"`
	Total   int                  `json:"total"`
	Limit   int                  `json:"limit"`
	Offset  int                  `json:"offset"`
	HasMore bool                 `json:"has_more"`
}
type TradePeerPage struct {
	Items   []TradePeer `json:"items"`
	Total   int         `json:"total"`
	Limit   int         `json:"limit"`
	Offset  int         `json:"offset"`
	HasMore bool        `json:"has_more"`
}

func ReadTradeMessage(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (TradeMessageRecord, error) {
	var out TradeMessageRecord
	if !key(id) {
		return out, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	m, env, p, direction, e := tradeMessageTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out = TradeMessageRecord{PeerID: p.ID, Direction: direction, Message: m, Envelope: env}
	return out, tx.Commit()
}
func ListTradeMessages(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, direction, kind string, limit, offset int) (TradeMessagePage, error) {
	out := TradeMessagePage{Items: []TradeMessageRecord{}, Limit: limit, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000 || (direction != "" && direction != "sent" && direction != "received") || (kind != "" && kind != "order" && kind != "decision") {
		return out, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	filter := ` WHERE tenant_id=? AND store_id=? AND (?='' OR direction=?) AND (?='' OR kind=?)`
	args := []any{a.TenantID, a.StoreID, direction, direction, kind, kind}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM company_trade_messages`+filter, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	pageArgs := append(append([]any{}, args...), limit, offset)
	rows, e := tx.QueryContext(ctx, `SELECT id FROM company_trade_messages`+filter+` ORDER BY id LIMIT ? OFFSET ?`, pageArgs...)
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
		m, env, p, direction, e := tradeMessageTx(ctx, tx, a, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, TradeMessageRecord{PeerID: p.ID, Direction: direction, Message: m, Envelope: env})
	}
	out.HasMore = offset+len(out.Items) < out.Total
	return out, tx.Commit()
}
func ListTradePeers(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, limit, offset int) (TradePeerPage, error) {
	out := TradePeerPage{Items: []TradePeer{}, Limit: limit, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM company_trade_peers WHERE tenant_id=? AND store_id=?`, a.TenantID, a.StoreID).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM company_trade_peers WHERE tenant_id=? AND store_id=? ORDER BY id LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, limit, offset)
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
		p, e := tradePeerTx(ctx, tx, a, id, false)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, p)
	}
	out.HasMore = offset+len(out.Items) < out.Total
	return out, tx.Commit()
}
