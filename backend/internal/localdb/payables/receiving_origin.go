package payables

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/identity"
)

// CheckReceivingOriginTx is a write gate for new approvals and payments.
// Original receipts/replays, returns and cancellation stay recoverable.
// Legacy documents without a receiving origin retain their existing rules.
func CheckReceivingOriginTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) error {
	var n int
	if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM payable_receiving_origins WHERE tenant_id=? AND store_id=? AND device_id=? AND payable_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&n); e != nil {
		return e
	}
	if n == 0 {
		return nil
	}
	var valid int
	e := tx.QueryRowContext(ctx, `SELECT count(*) FROM payable_receiving_origins o
 JOIN supplier_payables p ON p.tenant_id=o.tenant_id AND p.store_id=o.store_id AND p.device_id=o.device_id AND p.id=o.payable_id
 JOIN purchase_receipts r ON r.tenant_id=o.tenant_id AND r.store_id=o.store_id AND r.id=o.receipt_id AND r.order_id=o.order_id AND r.supplier_id=o.supplier_id AND r.request_json=o.receipt_request_json
 JOIN purchase_deliveries d ON d.tenant_id=o.tenant_id AND d.store_id=o.store_id AND d.id=o.delivery_id AND d.receipt_id=r.id AND d.state='received'
 JOIN stock_operations s ON s.id=r.stock_operation_id AND s.tenant_id=r.tenant_id AND s.store_id=r.store_id AND s.device_id=r.device_id AND s.kind='entry' AND s.product_id=r.product_id AND s.quantity_milli=r.accepted_milli AND s.to_location_id=r.location_id AND s.from_location_id IS NULL AND s.actor_identity_id=r.actor_id AND s.reason=r.reason AND s.occurred_at=r.created_at
 JOIN stock_operation_movements l ON l.operation_id=s.id AND l.movement_id=r.movement_id
 JOIN stock_movements m ON m.id=r.movement_id AND m.tenant_id=r.tenant_id AND m.store_id=r.store_id AND m.quantity_milli=r.accepted_milli AND m.product_id=r.product_id AND m.location_id=r.location_id AND m.device_id=r.device_id AND m.reason=r.reason AND m.created_at=r.created_at
 JOIN purchase_receiving_events ev ON ev.tenant_id=r.tenant_id AND ev.store_id=r.store_id AND ev.device_id=r.device_id AND ev.operation_id=r.operation_id AND ev.kind='received' AND ev.order_id=r.order_id AND ev.actor_id=r.actor_id AND ev.request_json=r.request_json AND ev.created_at=r.created_at
 WHERE o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.payable_id=? AND o.amount_cents=p.total_cents AND o.supplier_id=p.supplier_id
 AND NOT EXISTS(SELECT 1 FROM purchase_receipt_voids v WHERE v.tenant_id=r.tenant_id AND v.store_id=r.store_id AND v.receipt_id=r.id)`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&valid)
	if e != nil {
		return e
	}
	if valid != 1 {
		return ErrConflict
	}
	return nil
}
