package sale

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"titansystem-backend/internal/localdb/identity"
)

func ReadCustomerReturns(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, saleID string) (CustomerReturnHistory, error) {
	if db == nil || !planID(saleID) {
		return CustomerReturnHistory{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CustomerReturnHistory{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.CancelSale); err != nil {
		return CustomerReturnHistory{}, err
	}
	h, err := readCustomerReturnsTx(ctx, tx, actor, device, saleID)
	if err != nil {
		return h, err
	}
	return h, tx.Commit()
}

func QuoteCustomerReturn(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, saleID string, items []ReturnItemInput) (CustomerReturnQuote, error) {
	if db == nil || !planID(saleID) || !validReturnItems(items) {
		return CustomerReturnQuote{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CustomerReturnQuote{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.CancelSale); err != nil {
		return CustomerReturnQuote{}, err
	}
	h, err := readCustomerReturnsTx(ctx, tx, actor, device, saleID)
	if err != nil {
		return CustomerReturnQuote{}, err
	}
	if err = checkReturnUnitsTx(ctx, tx, actor, h, items); err != nil {
		return CustomerReturnQuote{}, err
	}
	q, err := quoteReturn(h, items)
	if err != nil {
		return q, err
	}
	if err = returnDrawerTx(ctx, tx, actor, device, h.CashSessionID, q.RefundCents); err != nil {
		return q, err
	}
	return q, tx.Commit()
}

// Existing records remain readable with current membership permissions even
// after the original shift closes or the POS license expires.
func ReadCustomerReturnOperation(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, operation string) (CustomerReturn, error) {
	if db == nil || !proofKey(operation) {
		return CustomerReturn{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CustomerReturn{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.CancelSale); err != nil {
		return CustomerReturn{}, err
	}
	var id string
	err = tx.QueryRowContext(ctx, `SELECT sale_id FROM customer_returns WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, actor.TenantID, actor.StoreID, device.DeviceID, operation).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return CustomerReturn{}, ErrNotFound
	}
	if err != nil {
		return CustomerReturn{}, err
	}
	h, err := readCustomerReturnsTx(ctx, tx, actor, device, id)
	if err != nil {
		return CustomerReturn{}, err
	}
	for _, r := range h.Returns {
		if r.OperationID == operation {
			return r, tx.Commit()
		}
	}
	return CustomerReturn{}, ErrConflict
}

func readCustomerReturnsTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (CustomerReturnHistory, error) {
	h := CustomerReturnHistory{SaleID: id, Items: []ReturnBalanceLine{}, Returns: []CustomerReturn{}}
	err := tx.QueryRowContext(ctx, `SELECT status,cash_session_id,total_cents FROM sales WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&h.SaleStatus, &h.CashSessionID, &h.OriginalTotalCents)
	if errors.Is(err, sql.ErrNoRows) {
		return h, ErrNotFound
	}
	if err != nil {
		return h, err
	}
	if (h.SaleStatus != "committed" && h.SaleStatus != "cancelled") || h.OriginalTotalCents <= 0 || h.OriginalTotalCents > checkoutMaxSafe {
		return h, ErrConflict
	}
	var paid, totalParts, badParts int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0),count(*),COALESCE(SUM(CASE WHEN method='cash' AND amount_cents>0 AND status=? THEN 0 ELSE 1 END),0) FROM sale_payments WHERE tenant_id=? AND sale_id=?`, map[bool]string{true: "reversed", false: "confirmed"}[h.SaleStatus == "cancelled"], a.TenantID, id).Scan(&paid, &totalParts, &badParts)
	if err != nil {
		return h, err
	}
	if paid != h.OriginalTotalCents || totalParts == 0 || badParts != 0 {
		return h, ErrPaymentUnverified
	}
	var sourceCount int
	var collected int64
	err = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(sum(amount_cents),0) FROM cash_movements WHERE tenant_id=? AND store_id=? AND cash_session_id=? AND reason=?`, a.TenantID, a.StoreID, h.CashSessionID, "venda:"+id).Scan(&sourceCount, &collected)
	if err != nil {
		return h, err
	}
	if sourceCount != 1 || collected != h.OriginalTotalCents {
		return h, ErrConflict
	}
	rows, err := tx.QueryContext(ctx, `SELECT i.id,i.product_id,m.location_id,i.quantity_milli,i.total_cents,m.id,m.quantity_milli,m.reason FROM sale_items i JOIN sale_item_stock x ON x.tenant_id=i.tenant_id AND x.sale_id=i.sale_id AND x.item_id=i.id JOIN stock_movements m ON m.id=x.movement_id AND m.tenant_id=i.tenant_id AND m.product_id=i.product_id WHERE i.tenant_id=? AND i.sale_id=? AND m.store_id=? AND m.device_id=? ORDER BY i.id`, a.TenantID, id, a.StoreID, d.DeviceID)
	if err != nil {
		return h, err
	}
	var original int64
	for rows.Next() {
		var l ReturnBalanceLine
		var consumed int64
		var reason string
		if err = rows.Scan(&l.SaleItemID, &l.ProductID, &l.LocationID, &l.SoldMilli, &l.OriginalCents, &l.originalMovement, &consumed, &reason); err != nil {
			rows.Close()
			return h, err
		}
		if l.SoldMilli <= 0 || l.SoldMilli > checkoutMaxSafe || consumed != -l.SoldMilli || reason != "venda:"+id || l.OriginalCents < 0 || l.OriginalCents > checkoutMaxSafe-original {
			rows.Close()
			return h, ErrConflict
		}
		l.RemainingMilli = l.SoldMilli
		l.RemainingCents = l.OriginalCents
		original += l.OriginalCents
		h.Items = append(h.Items, l)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return h, err
	}
	if closeErr != nil {
		return h, closeErr
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM sale_items WHERE tenant_id=? AND sale_id=?`, a.TenantID, id).Scan(&count); err != nil {
		return h, err
	}
	if count < 1 || count > 500 || count != len(h.Items) || original != h.OriginalTotalCents {
		return h, ErrConflict
	}
	type stored struct {
		r           CustomerReturn
		input, hash string
	}
	storedReturns := []stored{}
	rows, err = tx.QueryContext(ctx, `SELECT return_id,operation_id,cash_session_id,actor_identity_id,reason,refund_cents,cash_movement_id,created_at,input_json,request_hash FROM customer_returns WHERE tenant_id=? AND store_id=? AND device_id=? AND sale_id=? ORDER BY sequence LIMIT 1001`, a.TenantID, a.StoreID, d.DeviceID, id)
	if err != nil {
		return h, err
	}
	for rows.Next() {
		var s stored
		s.r.SaleID = id
		if err = rows.Scan(&s.r.ID, &s.r.OperationID, &s.r.CashSessionID, &s.r.ActorID, &s.r.Reason, &s.r.RefundCents, &s.r.CashMovementID, &s.r.CreatedAt, &s.input, &s.hash); err != nil {
			rows.Close()
			return h, err
		}
		storedReturns = append(storedReturns, s)
	}
	err = rows.Err()
	closeErr = rows.Close()
	if err != nil {
		return h, err
	}
	if closeErr != nil {
		return h, closeErr
	}
	if len(storedReturns) > 1000 {
		return h, ErrReturnRange
	}
	var historicalLines int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM customer_return_items WHERE tenant_id=? AND sale_id=?`, a.TenantID, id).Scan(&historicalLines)
	if err != nil {
		return h, err
	}
	if historicalLines > 10000 {
		return h, ErrReturnRange
	}
	if len(storedReturns) > 0 && h.SaleStatus != "committed" {
		return h, ErrConflict
	}
	for _, s := range storedReturns {
		var in CustomerReturnInput
		if json.Unmarshal([]byte(s.input), &in) != nil || !validCustomerReturn(in) || planHash(in) != s.hash || in.OperationID != s.r.OperationID || in.SaleID != id || in.Reason != s.r.Reason || in.ExpectedRefundCents != s.r.RefundCents || s.r.CashSessionID != h.CashSessionID {
			return h, ErrConflict
		}
		q, e := quoteReturn(h, in.Items)
		if e != nil {
			return h, ErrConflict
		}
		if q.RefundCents != s.r.RefundCents {
			return h, ErrConflict
		}
		for _, l := range q.Items {
			var qty, cents int64
			var disposition, origin, product, location string
			var moved sql.NullString
			e = tx.QueryRowContext(ctx, `SELECT quantity_milli,refund_cents,disposition,original_movement_id,product_id,location_id,return_movement_id FROM customer_return_items WHERE tenant_id=? AND return_id=? AND sale_item_id=?`, a.TenantID, s.r.ID, l.SaleItemID).Scan(&qty, &cents, &disposition, &origin, &product, &location, &moved)
			if e != nil {
				return h, ErrConflict
			}
			if qty != l.QuantityMilli || cents != l.RefundCents || disposition != l.Disposition || origin != l.OriginalMovementID || product != l.ProductID || location != l.LocationID || moved.Valid != (l.Disposition == "restock") {
				return h, ErrConflict
			}
			if moved.Valid {
				var amount int64
				var reason, stamp string
				e = tx.QueryRowContext(ctx, `SELECT quantity_milli,reason,created_at FROM stock_movements WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND product_id=? AND location_id=?`, a.TenantID, a.StoreID, d.DeviceID, moved.String, l.ProductID, l.LocationID).Scan(&amount, &reason, &stamp)
				if e != nil || amount != l.QuantityMilli || reason != "devolucao-cliente:"+s.r.ID || stamp != s.r.CreatedAt {
					return h, ErrConflict
				}
				v := moved.String
				l.ReturnMovementID = &v
			}
			s.r.Items = append(s.r.Items, l)
			for i := range h.Items {
				b := &h.Items[i]
				if b.SaleItemID == l.SaleItemID {
					b.ReturnedMilli += l.QuantityMilli
					b.RefundedCents += l.RefundCents
					b.RemainingMilli -= l.QuantityMilli
					b.RemainingCents -= l.RefundCents
				}
			}
		}
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM customer_return_items WHERE tenant_id=? AND return_id=?`, a.TenantID, s.r.ID).Scan(&count); e != nil {
			return h, e
		}
		if count != len(q.Items) {
			return h, ErrConflict
		}
		var delta, posted int64
		var reason, stamp, category, operation, who string
		e = tx.QueryRowContext(ctx, `SELECT m.amount_cents,m.reason,m.created_at,j.cash_delta_cents,j.category,j.operation_id,j.actor_identity_id FROM cash_movements m JOIN cash_journal_entries j ON j.tenant_id=m.tenant_id AND j.entry_id='movement:'||m.id AND j.store_id=m.store_id AND j.session_id=m.cash_session_id AND j.device_id=? WHERE m.tenant_id=? AND m.store_id=? AND m.cash_session_id=? AND m.id=?`, d.DeviceID, a.TenantID, a.StoreID, h.CashSessionID, s.r.CashMovementID).Scan(&delta, &reason, &stamp, &posted, &category, &operation, &who)
		if e != nil || delta != -s.r.RefundCents || posted != delta || reason != "devolucao-cliente:"+s.r.ID || stamp != s.r.CreatedAt || category != "sale_reversal" || operation != s.r.OperationID || who != s.r.ActorID {
			return h, ErrConflict
		}
		var events int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND operation_id=? AND event_type='sale.customer_returned'`, a.TenantID, a.StoreID, d.DeviceID, s.r.ID, s.r.OperationID).Scan(&events); e != nil {
			return h, e
		}
		if events != 1 {
			return h, ErrConflict
		}
		h.RefundedCents += s.r.RefundCents
		h.Returns = append(h.Returns, s.r)
	}
	h.RemainingCents = h.OriginalTotalCents - h.RefundedCents
	if h.RemainingCents < 0 {
		return h, ErrConflict
	}
	if h.SaleStatus == "cancelled" {
		var cancelled int64
		if err = tx.QueryRowContext(ctx, `SELECT refunded_cents FROM sale_cancellations WHERE tenant_id=? AND store_id=? AND device_id=? AND sale_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&cancelled); err != nil {
			return h, ErrConflict
		}
		if cancelled != h.OriginalTotalCents {
			return h, ErrConflict
		}
		h.CancelledCents = cancelled
		h.RemainingCents = 0
		for i := range h.Items {
			h.Items[i].CancelledMilli = h.Items[i].SoldMilli
			h.Items[i].CancelledCents = h.Items[i].OriginalCents
			h.Items[i].RemainingMilli = 0
			h.Items[i].RemainingCents = 0
		}
	}
	h.Complete = true
	return h, nil
}
