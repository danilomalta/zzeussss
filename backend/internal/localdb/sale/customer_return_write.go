package sale

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// returnDrawerTx checks physical funds in the selected local open drawer.
// Cross-shift authorization and provenance are checked separately.
func returnDrawerTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string, refund int64) error {
	var opening int64
	err := tx.QueryRowContext(ctx, `SELECT opening_cents FROM cash_sessions WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND closed_at IS NULL`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&opening)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCashClosed
	}
	if err != nil {
		return err
	}
	balance := big.NewInt(opening)
	rows, err := tx.QueryContext(ctx, `SELECT amount_cents FROM cash_movements WHERE tenant_id=? AND store_id=? AND cash_session_id=?`, a.TenantID, a.StoreID, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var n int64
		if err = rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		balance.Add(balance, big.NewInt(n))
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if balance.Cmp(big.NewInt(refund)) < 0 || balance.Cmp(big.NewInt(checkoutMaxSafe)) > 0 {
		return ErrCancellationBalance
	}
	return nil
}

func CreateCustomerReturn(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in CustomerReturnInput) (CustomerReturn, error) {
	if db == nil || !validCustomerReturn(in) {
		return CustomerReturn{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CustomerReturn{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, a, d, identity.CancelSale, modules.POS); err != nil {
		return CustomerReturn{}, err
	}
	if in.RefundCashSessionID != "" {
		if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash); err != nil {
			return CustomerReturn{}, err
		}
	}
	hash := planHash(in)
	var priorSale, priorActor, priorHash string
	err = tx.QueryRowContext(ctx, `SELECT sale_id,actor_identity_id,request_hash FROM customer_returns WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID).Scan(&priorSale, &priorActor, &priorHash)
	if err == nil {
		if priorSale != in.SaleID || priorActor != a.IdentityID || priorHash != hash {
			return CustomerReturn{}, ErrConflict
		}
		h, e := readCustomerReturnsTx(ctx, tx, a, d, in.SaleID)
		if e != nil {
			return CustomerReturn{}, e
		}
		for _, r := range h.Returns {
			if r.OperationID == in.OperationID {
				r.Repeated = true
				return r, tx.Commit()
			}
		}
		return CustomerReturn{}, ErrConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CustomerReturn{}, err
	}
	var used int
	err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM sale_operations WHERE tenant_id=? AND device_id=? AND operation_id=?)+(SELECT count(*) FROM sale_cancellations WHERE tenant_id=? AND device_id=? AND operation_id=?)`, a.TenantID, d.DeviceID, in.OperationID, a.TenantID, d.DeviceID, in.OperationID).Scan(&used)
	if err != nil {
		return CustomerReturn{}, err
	}
	if used != 0 {
		return CustomerReturn{}, ErrConflict
	}
	h, err := readCustomerReturnsTx(ctx, tx, a, d, in.SaleID)
	if err != nil {
		return CustomerReturn{}, err
	}
	if len(h.Returns) >= 1000 {
		return CustomerReturn{}, ErrReturnRange
	}
	q, err := quoteReturn(h, in.Items)
	if err != nil {
		return CustomerReturn{}, err
	}
	if q.RefundCents != in.ExpectedRefundCents {
		return CustomerReturn{}, ErrConflict
	}
	if err = checkReturnUnitsTx(ctx, tx, a, h, in.Items); err != nil {
		return CustomerReturn{}, err
	}
	drawer, e := resolveReturnDrawerTx(ctx, tx, a, d, h.CashSessionID, in.RefundCashSessionID)
	if e != nil {
		return CustomerReturn{}, e
	}
	if err = returnDrawerTx(ctx, tx, a, d, drawer, q.RefundCents); err != nil {
		return CustomerReturn{}, err
	}
	id, err := localdb.NewID()
	if err != nil {
		return CustomerReturn{}, err
	}
	cashID, err := localdb.NewID()
	if err != nil {
		return CustomerReturn{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	reason := "devolucao-cliente:" + id
	if err = cancelWrite(ctx, tx, 1, `INSERT INTO cash_movements VALUES(?,?,?,?,?,?,?)`, cashID, a.TenantID, a.StoreID, drawer, -q.RefundCents, reason, now); err != nil {
		return CustomerReturn{}, err
	}
	encoded, err := json.Marshal(in)
	if err != nil {
		return CustomerReturn{}, err
	}
	if err = cancelWrite(ctx, tx, 1, `INSERT INTO customer_returns(tenant_id,store_id,device_id,return_id,operation_id,sale_id,cash_session_id,actor_identity_id,reason,refund_cents,cash_movement_id,request_hash,input_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, id, in.OperationID, in.SaleID, drawer, a.IdentityID, in.Reason, q.RefundCents, cashID, hash, string(encoded), now); err != nil {
		return CustomerReturn{}, err
	}
	if drawer != h.CashSessionID {
		if err = cancelWrite(ctx, tx, 1, `INSERT INTO customer_return_cross_shifts(tenant_id,store_id,device_id,return_id,original_cash_session_id,refund_cash_session_id,actor_identity_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, id, h.CashSessionID, drawer, a.IdentityID, now); err != nil {
			return CustomerReturn{}, err
		}
	}
	for _, l := range q.Items {
		var moved any
		if l.Disposition == "restock" {
			var balance int64
			err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, a.TenantID, a.StoreID, l.ProductID, l.LocationID).Scan(&balance)
			if err != nil {
				return CustomerReturn{}, err
			}
			if balance < 0 || balance > checkoutMaxSafe-l.QuantityMilli {
				return CustomerReturn{}, ErrCancellationBalance
			}
			mid, e := localdb.NewID()
			if e != nil {
				return CustomerReturn{}, e
			}
			moved = mid
			if err = cancelWrite(ctx, tx, 1, `INSERT INTO stock_movements VALUES(?,?,?,?,?,?,?,?,?)`, mid, a.TenantID, a.StoreID, d.DeviceID, l.ProductID, l.LocationID, l.QuantityMilli, reason, now); err != nil {
				return CustomerReturn{}, err
			}
		}
		if err = cancelWrite(ctx, tx, 1, `INSERT INTO customer_return_items VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, id, in.SaleID, l.SaleItemID, l.ProductID, l.LocationID, l.QuantityMilli, l.RefundCents, l.Disposition, l.OriginalMovementID, moved); err != nil {
			return CustomerReturn{}, err
		}
	}
	if err = cashjournal.CaptureTx(ctx, tx, a, d, "movement", cashID); err != nil {
		return CustomerReturn{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return CustomerReturn{}, err
	}
	payload, err := json.Marshal(struct {
		ReturnID       string              `json:"return_id"`
		ActorID        string              `json:"actor_id"`
		CashMovementID string              `json:"cash_movement_id"`
		Input          CustomerReturnInput `json:"input"`
	}{id, a.IdentityID, cashID, in})
	if err != nil {
		return CustomerReturn{}, err
	}
	if err = cancelWrite(ctx, tx, 1, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'sale.customer_returned',1,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, id, string(payload), now); err != nil {
		return CustomerReturn{}, err
	}
	h, err = readCustomerReturnsTx(ctx, tx, a, d, in.SaleID)
	if err != nil {
		return CustomerReturn{}, err
	}
	for _, r := range h.Returns {
		if r.ID == id {
			return r, tx.Commit()
		}
	}
	return CustomerReturn{}, ErrConflict
}

// Unit products cannot be returned as fractions of a unit. This check is for a
// new physical receipt only; historical reads never depend on today's catalog.
func checkReturnUnitsTx(ctx context.Context, tx *sql.Tx, a identity.Scope, h CustomerReturnHistory, items []ReturnItemInput) error {
	for _, in := range items {
		for _, l := range h.Items {
			if l.SaleItemID == in.SaleItemID {
				var unit string
				if err := tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, l.ProductID).Scan(&unit); err != nil {
					return err
				}
				if unit == "unit" && in.QuantityMilli%1000 != 0 {
					return ErrInvalid
				}
			}
		}
	}
	return nil
}
