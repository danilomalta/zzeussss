package sale

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var ErrCancellationBalance = errors.New("saldo insuficiente ou excedido para estorno")

type CancelInput struct {
	OperationID string `json:"operation_id"`
	SaleID      string `json:"sale_id"`
	Reason      string `json:"reason"`
}

type CancelResult struct {
	SaleID        string `json:"sale_id"`
	OperationID   string `json:"operation_id"`
	RefundedCents int64  `json:"refunded_cents"`
	CancelledAt   string `json:"cancelled_at"`
	Repeated      bool   `json:"repeated"`
}

// CancelWithContract accepts only a complete cash refund in the original open
// drawer, on the sale's original device. All authorization and effects share a
// transaction. The caller must supply an authenticated session, never body IDs.
func CancelWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in CancelInput) (CancelResult, error) {
	if db == nil || !cancelID(in.OperationID) || !cancelID(in.SaleID) || !utf8.ValidString(in.Reason) ||
		strings.TrimSpace(in.Reason) == "" || utf8.RuneCountInString(in.Reason) > 500 || len(in.Reason) > 2000 || strings.ContainsRune(in.Reason, 0) {
		return CancelResult{}, ErrInvalid
	}
	encoded, err := json.Marshal(in)
	if err != nil {
		return CancelResult{}, err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(encoded))
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CancelResult{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, actor, device, identity.CancelSale, modules.POS); err != nil {
		return CancelResult{}, err
	}
	var priorSale, priorHash, priorActor string
	var prior CancelResult
	err = tx.QueryRowContext(ctx, `SELECT sale_id,request_hash,actor_identity_id,refunded_cents,cancelled_at
		FROM sale_cancellations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`,
		actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID).
		Scan(&priorSale, &priorHash, &priorActor, &prior.RefundedCents, &prior.CancelledAt)
	if err == nil {
		if priorSale != in.SaleID || priorHash != hash || priorActor != actor.IdentityID {
			return CancelResult{}, ErrConflict
		}
		prior.SaleID, prior.OperationID, prior.Repeated = in.SaleID, in.OperationID, true
		if err = tx.Commit(); err != nil {
			return CancelResult{}, err
		}
		return prior, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CancelResult{}, err
	}
	var reused int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sale_operations WHERE tenant_id=? AND device_id=? AND operation_id=?`,
		actor.TenantID, device.DeviceID, in.OperationID).Scan(&reused); err != nil {
		return CancelResult{}, err
	}
	if reused != 0 {
		return CancelResult{}, ErrConflict
	}
	var sessionID, status string
	var total int64
	err = tx.QueryRowContext(ctx, `SELECT cash_session_id,status,total_cents FROM sales
		WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, actor.TenantID, actor.StoreID, device.DeviceID, in.SaleID).
		Scan(&sessionID, &status, &total)
	if errors.Is(err, sql.ErrNoRows) {
		return CancelResult{}, ErrNotFound
	}
	if err != nil {
		return CancelResult{}, err
	}
	if status != "committed" || total <= 0 {
		return CancelResult{}, ErrConflict
	}
	var opening int64
	err = tx.QueryRowContext(ctx, `SELECT opening_cents FROM cash_sessions
		WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND closed_at IS NULL`,
		actor.TenantID, actor.StoreID, device.DeviceID, sessionID).Scan(&opening)
	if errors.Is(err, sql.ErrNoRows) {
		return CancelResult{}, ErrCashClosed
	}
	if err != nil {
		return CancelResult{}, err
	}
	var payments, invalidPayments int
	var paid, cash, movements int64
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(amount_cents),0),
		COALESCE(SUM(CASE WHEN method='cash' AND status='confirmed' THEN 0 ELSE 1 END),0)
		FROM sale_payments WHERE tenant_id=? AND sale_id=?`, actor.TenantID, in.SaleID).
		Scan(&payments, &paid, &invalidPayments); err != nil {
		return CancelResult{}, err
	}
	if payments == 0 || invalidPayments != 0 || paid != total {
		return CancelResult{}, ErrPaymentUnverified
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM cash_movements
		WHERE tenant_id=? AND store_id=? AND cash_session_id=? AND reason=?`,
		actor.TenantID, actor.StoreID, sessionID, "venda:"+in.SaleID).Scan(&cash); err != nil {
		return CancelResult{}, err
	}
	if cash != total {
		return CancelResult{}, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM cash_movements
		WHERE tenant_id=? AND store_id=? AND cash_session_id=?`, actor.TenantID, actor.StoreID, sessionID).
		Scan(&movements); err != nil {
		return CancelResult{}, err
	}
	if movements > 0 && opening > math.MaxInt64-movements {
		return CancelResult{}, ErrCancellationBalance
	}
	if opening+movements < total {
		return CancelResult{}, ErrCancellationBalance
	}
	type returnItem struct {
		itemID, productID, locationID, movementID string
		quantity                                  int64
	}
	rows, err := tx.QueryContext(ctx, `SELECT i.id,i.product_id,m.location_id,m.id,i.quantity_milli,m.quantity_milli
		FROM sale_items i JOIN sale_item_stock x ON x.tenant_id=i.tenant_id AND x.sale_id=i.sale_id AND x.item_id=i.id
		JOIN stock_movements m ON m.id=x.movement_id AND m.tenant_id=i.tenant_id AND m.product_id=i.product_id
		WHERE i.tenant_id=? AND i.sale_id=? AND m.store_id=? AND m.device_id=? ORDER BY i.id`,
		actor.TenantID, in.SaleID, actor.StoreID, device.DeviceID)
	if err != nil {
		return CancelResult{}, err
	}
	items := make([]returnItem, 0)
	for rows.Next() {
		var item returnItem
		var consumed int64
		if err = rows.Scan(&item.itemID, &item.productID, &item.locationID, &item.movementID, &item.quantity, &consumed); err != nil {
			_ = rows.Close()
			return CancelResult{}, err
		}
		if item.quantity <= 0 || consumed != -item.quantity {
			_ = rows.Close()
			return CancelResult{}, ErrConflict
		}
		items = append(items, item)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return CancelResult{}, err
	}
	if closeErr != nil {
		return CancelResult{}, closeErr
	}
	var itemCount int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sale_items WHERE tenant_id=? AND sale_id=?`, actor.TenantID, in.SaleID).Scan(&itemCount); err != nil {
		return CancelResult{}, err
	}
	if len(items) == 0 || len(items) > 500 || len(items) != itemCount {
		return CancelResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	cashID, err := localdb.NewID()
	if err != nil {
		return CancelResult{}, err
	}
	if err = cancelWrite(ctx, tx, 1, `INSERT INTO cash_movements(id,tenant_id,store_id,cash_session_id,amount_cents,reason,created_at)
		VALUES (?,?,?,?,?,?,?)`, cashID, actor.TenantID, actor.StoreID, sessionID, -total, "cancelamento:"+in.SaleID, now); err != nil {
		return CancelResult{}, err
	}
	if err = cancelWrite(ctx, tx, 1, `INSERT INTO sale_cancellations
		(tenant_id,store_id,device_id,operation_id,sale_id,actor_identity_id,reason,request_hash,refunded_cents,cash_movement_id,cancelled_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.SaleID, actor.IdentityID, in.Reason, hash, total, cashID, now); err != nil {
		return CancelResult{}, err
	}
	for _, item := range items {
		var balance int64
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements
			WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, actor.TenantID, actor.StoreID, item.productID, item.locationID).
			Scan(&balance); err != nil {
			return CancelResult{}, err
		}
		if balance < 0 || balance > math.MaxInt64-item.quantity {
			return CancelResult{}, ErrCancellationBalance
		}
		id, err := localdb.NewID()
		if err != nil {
			return CancelResult{}, err
		}
		if err = cancelWrite(ctx, tx, 1, `INSERT INTO stock_movements
			(id,tenant_id,store_id,device_id,product_id,location_id,quantity_milli,reason,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			id, actor.TenantID, actor.StoreID, device.DeviceID, item.productID, item.locationID, item.quantity, "cancelamento:"+in.SaleID, now); err != nil {
			return CancelResult{}, err
		}
		if err = cancelWrite(ctx, tx, 1, `INSERT INTO sale_cancellation_stock VALUES (?,?,?,?,?)`, actor.TenantID, in.SaleID, item.itemID, item.movementID, id); err != nil {
			return CancelResult{}, err
		}
	}
	if err = cancelWrite(ctx, tx, int64(payments), `UPDATE sale_payments SET status='reversed' WHERE tenant_id=? AND sale_id=?`, actor.TenantID, in.SaleID); err != nil {
		return CancelResult{}, err
	}
	if err = cancelWrite(ctx, tx, 1, `UPDATE sales SET status='cancelled' WHERE tenant_id=? AND id=? AND status='committed'`, actor.TenantID, in.SaleID); err != nil {
		return CancelResult{}, err
	}
	payload, err := json.Marshal(struct {
		TenantID      string `json:"tenant_id"`
		StoreID       string `json:"store_id"`
		DeviceID      string `json:"device_id"`
		ActorID       string `json:"actor_id"`
		SaleID        string `json:"sale_id"`
		SessionID     string `json:"cash_session_id"`
		RefundedCents int64  `json:"refunded_cents"`
		Reason        string `json:"reason"`
		CancelledAt   string `json:"cancelled_at"`
	}{actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, in.SaleID, sessionID, total, in.Reason, now})
	if err != nil {
		return CancelResult{}, err
	}
	id, err := localdb.NewID()
	if err != nil {
		return CancelResult{}, err
	}
	if err = cancelWrite(ctx, tx, 1, `INSERT INTO outbox
		(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
		VALUES (?,?,?,?,?,?,'sale.cancelled',1,?,?)`, id, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.SaleID, string(payload), now); err != nil {
		return CancelResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return CancelResult{}, err
	}
	return CancelResult{SaleID: in.SaleID, OperationID: in.OperationID, RefundedCents: total, CancelledAt: now}, nil
}

func cancelID(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value && !strings.ContainsRune(value, 0)
}

// Triggers that silently ignore a write must also cause a full rollback.
func cancelWrite(ctx context.Context, tx *sql.Tx, expected int64, query string, args ...any) error {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != expected {
		return ErrConflict
	}
	return nil
}
