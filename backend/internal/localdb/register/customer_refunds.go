package register

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"

	"titansystem-backend/internal/localdb/identity"
)

// Validate the cash provenance inside the closed-report snapshot. Refunds are
// separate from the original payment's historical confirmed/reversed status.
func customerRefundTotalTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, saleID string) (int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT r.return_id,r.operation_id,r.refund_cents,r.cash_session_id,r.created_at,r.actor_identity_id,r.reason,r.input_json,r.request_hash,m.amount_cents,m.reason,m.created_at,j.cash_delta_cents,j.category,j.operation_id,j.actor_identity_id,(SELECT sum(i.refund_cents) FROM customer_return_items i WHERE i.tenant_id=r.tenant_id AND i.return_id=r.return_id),(SELECT count(*) FROM outbox o WHERE o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.device_id=r.device_id AND o.aggregate_id=r.return_id AND o.operation_id=r.operation_id AND o.event_type='sale.customer_returned') FROM customer_returns r JOIN sales s ON s.tenant_id=r.tenant_id AND s.id=r.sale_id AND s.cash_session_id=r.cash_session_id AND s.status='committed' LEFT JOIN cash_movements m ON m.tenant_id=r.tenant_id AND m.store_id=r.store_id AND m.cash_session_id=r.cash_session_id AND m.id=r.cash_movement_id LEFT JOIN cash_journal_entries j ON j.tenant_id=r.tenant_id AND j.store_id=r.store_id AND j.device_id=r.device_id AND j.session_id=r.cash_session_id AND j.entry_id='movement:'||r.cash_movement_id WHERE r.tenant_id=? AND r.store_id=? AND r.device_id=? AND r.sale_id=?`, a.TenantID, a.StoreID, d.DeviceID, saleID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var total int64
	for rows.Next() {
		var id, op, session, at, actor, reason, input, hash string
		var refund int64
		var delta, posted, lines sql.NullInt64
		var mr, mt, category, jo, ja sql.NullString
		var events int
		if err = rows.Scan(&id, &op, &refund, &session, &at, &actor, &reason, &input, &hash, &delta, &mr, &mt, &posted, &category, &jo, &ja, &lines, &events); err != nil {
			return 0, err
		}
		if !delta.Valid || !posted.Valid || !lines.Valid || !mr.Valid || !mt.Valid || !category.Valid || !jo.Valid || !ja.Valid || refund <= 0 || delta.Int64 != -refund || posted.Int64 != delta.Int64 || lines.Int64 != refund || mr.String != "devolucao-cliente:"+id || mt.String != at || category.String != "sale_reversal" || jo.String != op || ja.String != actor || events != 1 || fmt.Sprintf("%x", sha256.Sum256([]byte(input))) != hash {
			return 0, ErrReportInconsistent
		}
		var in struct {
			RefundSession string `json:"refund_cash_session_id"`
			OperationID   string `json:"operation_id"`
			SaleID        string `json:"sale_id"`
			Reason        string `json:"reason"`
			Expected      int64  `json:"expected_refund_cents"`
		}
		if json.Unmarshal([]byte(input), &in) != nil || in.OperationID != op || in.SaleID != saleID || in.Reason != reason || in.Expected != refund || (in.RefundSession != "" && in.RefundSession != session) {
			return 0, ErrReportInconsistent
		}
		if err = reportAdd(&total, refund); err != nil {
			return 0, err
		}
	}
	return total, rows.Err()
}

func priorSessionRefundTotalTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, refundSession string) (int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT r.return_id,r.operation_id,r.refund_cents,r.cash_session_id,r.created_at,r.actor_identity_id,r.reason,r.input_json,r.request_hash,m.amount_cents,m.reason,m.created_at,j.cash_delta_cents,j.category,j.operation_id,j.actor_identity_id,(SELECT sum(i.refund_cents) FROM customer_return_items i WHERE i.tenant_id=r.tenant_id AND i.return_id=r.return_id),(SELECT count(*) FROM outbox o WHERE o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.device_id=r.device_id AND o.aggregate_id=r.return_id AND o.operation_id=r.operation_id AND o.event_type='sale.customer_returned'),s.id,s.cash_session_id,x.original_cash_session_id,x.refund_cash_session_id,x.actor_identity_id,x.created_at FROM customer_returns r JOIN sales s ON s.tenant_id=r.tenant_id AND s.id=r.sale_id AND s.cash_session_id<>r.cash_session_id AND s.status='committed' LEFT JOIN customer_return_cross_shifts x ON x.tenant_id=r.tenant_id AND x.return_id=r.return_id LEFT JOIN cash_movements m ON m.tenant_id=r.tenant_id AND m.store_id=r.store_id AND m.cash_session_id=r.cash_session_id AND m.id=r.cash_movement_id LEFT JOIN cash_journal_entries j ON j.tenant_id=r.tenant_id AND j.store_id=r.store_id AND j.device_id=r.device_id AND j.session_id=r.cash_session_id AND j.entry_id='movement:'||r.cash_movement_id WHERE r.tenant_id=? AND r.store_id=? AND r.device_id=? AND r.cash_session_id=?`, a.TenantID, a.StoreID, d.DeviceID, refundSession)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var total int64
	for rows.Next() {
		var id, op, session, at, actor, reason, input, hash string
		var refund int64
		var delta, posted, lines sql.NullInt64
		var mr, mt, category, jo, ja sql.NullString
		var events int
		var sourceSale, sourceSession string
		var origin, dest, contextActor, contextStamp sql.NullString
		if err = rows.Scan(&id, &op, &refund, &session, &at, &actor, &reason, &input, &hash, &delta, &mr, &mt, &posted, &category, &jo, &ja, &lines, &events, &sourceSale, &sourceSession, &origin, &dest, &contextActor, &contextStamp); err != nil {
			return 0, err
		}
		if !delta.Valid || !posted.Valid || !lines.Valid || !mr.Valid || !mt.Valid || !category.Valid || !jo.Valid || !ja.Valid || refund <= 0 || delta.Int64 != -refund || posted.Int64 != delta.Int64 || lines.Int64 != refund || mr.String != "devolucao-cliente:"+id || mt.String != at || category.String != "sale_reversal" || jo.String != op || ja.String != actor || events != 1 || fmt.Sprintf("%x", sha256.Sum256([]byte(input))) != hash {
			return 0, ErrReportInconsistent
		}
		var in struct {
			RefundSession string `json:"refund_cash_session_id"`
			OperationID   string `json:"operation_id"`
			SaleID        string `json:"sale_id"`
			Reason        string `json:"reason"`
			Expected      int64  `json:"expected_refund_cents"`
		}
		if json.Unmarshal([]byte(input), &in) != nil || in.OperationID != op || in.SaleID != sourceSale || in.Reason != reason || in.Expected != refund || in.RefundSession != session {
			return 0, ErrReportInconsistent
		}
		if !origin.Valid || !dest.Valid || !contextActor.Valid || !contextStamp.Valid || origin.String != sourceSession || dest.String != session || contextActor.String != actor || contextStamp.String != at || sourceSession == session {
			return 0, ErrReportInconsistent
		}
		if err = reportAdd(&total, refund); err != nil {
			return 0, err
		}
	}
	return total, rows.Err()
}
