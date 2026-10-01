package sale

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"titansystem-backend/internal/localdb/identity"
)

type ReceiptItem struct {
	ItemID         string `json:"item_id"`
	ProductID      string `json:"product_id"`
	LocationID     string `json:"location_id"`
	QuantityMilli  int64  `json:"quantity_milli"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	TotalCents     int64  `json:"total_cents"`
}

type ReceiptPayment struct {
	PaymentID   string `json:"payment_id"`
	Method      string `json:"method"`
	Status      string `json:"status"`
	AmountCents int64  `json:"amount_cents"`
}

// Receipt is an operational local record, not an authorized fiscal document.
// Item names and unit labels were not snapshotted by the original schema;
// this API returns IDs and the stored quantities/prices without inventing them.
type Receipt struct {
	SaleID           string           `json:"sale_id"`
	OperationID      string           `json:"operation_id"`
	CashSessionID    string           `json:"cash_session_id"`
	Status           string           `json:"status"`
	CommittedAt      string           `json:"committed_at"`
	TotalCents       int64            `json:"total_cents"`
	FiscalAuthorized bool             `json:"fiscal_authorized"`
	Items            []ReceiptItem    `json:"items"`
	Payments         []ReceiptPayment `json:"payments"`
}

// Read only returns a sale from this company/store/device and human actor.
// No current license is required for authorized access to existing records.
func Read(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, saleID string) (Receipt, error) {
	if db == nil || strings.TrimSpace(saleID) == "" || len(saleID) > 128 {
		return Receipt{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Receipt{}, err
	}
	defer tx.Rollback()
	if err = authorizeLegacy(ctx, tx, actor, device); err != nil {
		return Receipt{}, err
	}
	result := Receipt{Items: make([]ReceiptItem, 0), Payments: make([]ReceiptPayment, 0)}
	err = tx.QueryRowContext(ctx, `SELECT s.id,o.operation_id,s.cash_session_id,s.status,s.committed_at,s.total_cents
		FROM sales s JOIN sale_operations o ON o.tenant_id=s.tenant_id AND o.sale_id=s.id
		WHERE s.tenant_id=? AND s.store_id=? AND s.device_id=? AND s.id=?
		AND o.store_id=? AND o.device_id=? AND o.actor_identity_id=?`,
		actor.TenantID, actor.StoreID, device.DeviceID, saleID, actor.StoreID, device.DeviceID, actor.IdentityID).
		Scan(&result.SaleID, &result.OperationID, &result.CashSessionID, &result.Status, &result.CommittedAt, &result.TotalCents)
	if errors.Is(err, sql.ErrNoRows) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT i.id,i.product_id,m.location_id,i.quantity_milli,i.unit_price_cents,i.total_cents
		FROM sale_items i JOIN sale_item_stock x ON x.tenant_id=i.tenant_id AND x.sale_id=i.sale_id AND x.item_id=i.id
		JOIN stock_movements m ON m.id=x.movement_id AND m.tenant_id=i.tenant_id
		WHERE i.tenant_id=? AND i.sale_id=? AND m.store_id=? AND m.device_id=? ORDER BY i.id`,
		actor.TenantID, saleID, actor.StoreID, device.DeviceID)
	if err != nil {
		return Receipt{}, err
	}
	for rows.Next() {
		var item ReceiptItem
		if err = rows.Scan(&item.ItemID, &item.ProductID, &item.LocationID, &item.QuantityMilli, &item.UnitPriceCents, &item.TotalCents); err != nil {
			_ = rows.Close()
			return Receipt{}, err
		}
		result.Items = append(result.Items, item)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return Receipt{}, err
	}
	if closeErr != nil {
		return Receipt{}, closeErr
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,method,status,amount_cents FROM sale_payments WHERE tenant_id=? AND sale_id=? ORDER BY id`, actor.TenantID, saleID)
	if err != nil {
		return Receipt{}, err
	}
	for rows.Next() {
		var payment ReceiptPayment
		if err = rows.Scan(&payment.PaymentID, &payment.Method, &payment.Status, &payment.AmountCents); err != nil {
			_ = rows.Close()
			return Receipt{}, err
		}
		result.Payments = append(result.Payments, payment)
	}
	err = rows.Err()
	closeErr = rows.Close()
	if err != nil {
		return Receipt{}, err
	}
	if closeErr != nil {
		return Receipt{}, closeErr
	}
	if err = tx.Commit(); err != nil {
		return Receipt{}, err
	}
	return result, nil
}
