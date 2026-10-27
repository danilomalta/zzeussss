package supplycycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/purchases"
)

type orderData struct {
	Supplier, Product, Unit string
	Planned, Accepted       int64
	Cancelled               bool
}

func orderTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (orderData, error) {
	var o orderData
	var count int
	e := tx.QueryRowContext(ctx, `SELECT o.supplier_id,i.product_id,i.unit,i.quantity_milli,(SELECT count(*) FROM purchase_order_cancellations c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND c.order_id=o.id) FROM purchase_orders o JOIN purchase_order_items i ON i.tenant_id=o.tenant_id AND i.store_id=o.store_id AND i.order_id=o.id WHERE o.tenant_id=? AND o.store_id=? AND o.id=?`, a.TenantID, a.StoreID, id).Scan(&o.Supplier, &o.Product, &o.Unit, &o.Planned, &count)
	if errors.Is(e, sql.ErrNoRows) {
		return o, purchases.ErrNotFound
	}
	if e != nil {
		return o, e
	}
	o.Cancelled = count != 0
	if o.Planned < 1 || o.Planned > purchases.MaxQuantity {
		return o, purchases.ErrConflict
	}
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&count)
	if e != nil {
		return o, e
	}
	if count != 1 {
		return o, purchases.ErrConflict
	}
	var rows int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&rows)
	if e != nil {
		return o, e
	}
	if rows > MaxDeliveries {
		return o, ErrLimit
	}
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(r.accepted_milli),0) FROM purchase_receipts r WHERE r.tenant_id=? AND r.store_id=? AND r.order_id=? AND NOT EXISTS(SELECT 1 FROM purchase_receipt_voids v WHERE v.tenant_id=r.tenant_id AND v.store_id=r.store_id AND v.receipt_id=r.id)`, a.TenantID, a.StoreID, id).Scan(&o.Accepted)
	if e != nil {
		return o, e
	}
	if o.Accepted < 0 || o.Accepted > o.Planned {
		return o, purchases.ErrConflict
	}
	return o, nil
}
func decisionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (*Decision, error) {
	var body string
	var rev int64
	e := tx.QueryRowContext(ctx, `SELECT revision,payload_json FROM purchase_supplier_decisions WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&rev, &body)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var v Decision
	if json.Unmarshal([]byte(body), &v) != nil || v.OrderID != id || v.Revision != rev || v.Revision < 1 || !((v.EvidenceMode == "manual_external" && !v.ExternalVerified) || (v.EvidenceMode == "authenticated_company" && v.ExternalVerified)) {
		return nil, purchases.ErrConflict
	}
	var saved string
	e = tx.QueryRowContext(ctx, `SELECT result_json FROM purchase_cycle_operations WHERE tenant_id=? AND store_id=? AND kind='decision' AND entity_id=? AND revision=?`, a.TenantID, a.StoreID, id, rev).Scan(&saved)
	var r Result
	if e != nil {
		return nil, e
	}
	if json.Unmarshal([]byte(saved), &r) != nil || !reflect.DeepEqual(r.Decision, &v) {
		return nil, purchases.ErrConflict
	}
	if v.ExternalVerified {
		if e := verifiedTradeDecisionTx(ctx, tx, a, v); e != nil {
			return nil, e
		}
	}
	return &v, nil
}
func deliveryTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Delivery, error) {
	var v Delivery
	var body, order, supplier, ref, state string
	var rev, qty int64
	var receipt sql.NullString
	e := tx.QueryRowContext(ctx, `SELECT order_id,supplier_id,delivery_reference,revision,state,planned_milli,receipt_id,payload_json FROM purchase_deliveries WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&order, &supplier, &ref, &rev, &state, &qty, &receipt, &body)
	if errors.Is(e, sql.ErrNoRows) {
		return v, purchases.ErrNotFound
	}
	if e != nil {
		return v, e
	}
	if json.Unmarshal([]byte(body), &v) != nil || v.ID != id || v.OrderID != order || v.SupplierID != supplier || v.DeliveryReference != ref || v.State != state || v.Revision != rev || v.PlannedMilli != qty || v.ReceiptID != receipt.String {
		return v, purchases.ErrConflict
	}
	var saved string
	e = tx.QueryRowContext(ctx, `SELECT result_json FROM purchase_cycle_operations WHERE tenant_id=? AND store_id=? AND kind='delivery' AND entity_id=? AND revision=?`, a.TenantID, a.StoreID, id, rev).Scan(&saved)
	var r Result
	if e != nil {
		return v, e
	}
	if json.Unmarshal([]byte(saved), &r) != nil || !reflect.DeepEqual(r.Delivery, &v) {
		return v, purchases.ErrConflict
	}
	return rejectedDeliveryTx(ctx, tx, a, v, body)
}
func allocatedTx(ctx context.Context, tx *sql.Tx, a identity.Scope, order string) (int64, error) {
	rows, e := tx.QueryContext(ctx, `SELECT id FROM purchase_deliveries WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY id LIMIT ?`, a.TenantID, a.StoreID, order, MaxDeliveries+1)
	if e != nil {
		return 0, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return 0, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, e
	}
	if len(ids) > MaxDeliveries {
		return 0, ErrLimit
	}
	var total int64
	for _, id := range ids {
		v, e := deliveryTx(ctx, tx, a, id)
		if e != nil {
			return 0, e
		}
		if v.State != "cancelled" && v.State != "rejected" {
			qty := v.PlannedMilli
			if v.State == "received" {
				var accepted int64
				var voids int
				e = tx.QueryRowContext(ctx, `SELECT accepted_milli,(SELECT count(*) FROM purchase_receipt_voids x WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.receipt_id=r.id) FROM purchase_receipts r WHERE tenant_id=? AND store_id=? AND id=? AND order_id=?`, a.TenantID, a.StoreID, v.ReceiptID, order).Scan(&accepted, &voids)
				if e != nil {
					return 0, e
				}
				if accepted < 1 || accepted > v.PlannedMilli {
					return 0, purchases.ErrConflict
				}
				qty = accepted
				if voids > 0 {
					qty = 0
				}
			}
			if qty < 0 || qty > purchases.MaxQuantity-total {
				return 0, purchases.ErrConflict
			}
			total += qty
		}
	}
	var legacy int64
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(r.accepted_milli),0) FROM purchase_receipts r WHERE r.tenant_id=? AND r.store_id=? AND r.order_id=? AND NOT EXISTS(SELECT 1 FROM purchase_receipt_voids v WHERE v.tenant_id=r.tenant_id AND v.store_id=r.store_id AND v.receipt_id=r.id) AND NOT EXISTS(SELECT 1 FROM purchase_deliveries d WHERE d.tenant_id=r.tenant_id AND d.store_id=r.store_id AND d.receipt_id=r.id)`, a.TenantID, a.StoreID, order).Scan(&legacy)
	if e != nil {
		return 0, e
	}
	if legacy < 0 || legacy > purchases.MaxQuantity-total {
		return 0, purchases.ErrConflict
	}
	return total + legacy, nil
}
func verifiedOperation(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, body string) (Result, error) {
	var r Result
	if json.Unmarshal([]byte(body), &r) != nil || r.OperationID != op || r.Repeated {
		return r, purchases.ErrConflict
	}
	var n int
	e := tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_cycle_events WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND payload_json=?`, a.TenantID, a.StoreID, d.DeviceID, op, body).Scan(&n)
	if e != nil {
		return r, e
	}
	if n != 1 {
		return r, purchases.ErrConflict
	}
	return r, nil
}
func readTx(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, finance bool) (*sql.Tx, error) {
	if db == nil {
		return nil, purchases.ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if e = authorize(ctx, tx, a, d, finance); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func GetDelivery(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Delivery, error) {
	if !key(id) {
		return Delivery{}, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return Delivery{}, e
	}
	defer tx.Rollback()
	v, e := deliveryTx(ctx, tx, a, id)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func GetCycle(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (Cycle, error) {
	var out Cycle
	if !key(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	o, e := orderTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	dec, e := decisionTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	allocated, e := allocatedTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out = Cycle{OrderID: id, Decision: dec, PlannedMilli: o.Planned, EffectiveAcceptedMilli: o.Accepted, RemainingOrderMilli: o.Planned - o.Accepted, AllocatedMilli: allocated, Deliveries: []Delivery{}, Limit: limit, Offset: offset}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM purchase_deliveries WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY id`, a.TenantID, a.StoreID, id)
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
	out.Total = len(ids)
	out.HasMore = offset+limit < out.Total
	for i, id := range ids {
		if i >= offset && i-offset < limit {
			v, e := deliveryTx(ctx, tx, a, id)
			if e != nil {
				return out, e
			}
			out.Deliveries = append(out.Deliveries, v)
		}
	}
	return out, tx.Commit()
}
func Operation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Result, error) {
	if !key(id) {
		return Result{}, purchases.ErrInvalid
	}
	if db == nil {
		return Result{}, purchases.ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	var who, kind, body string
	e = tx.QueryRowContext(ctx, `SELECT actor_id,kind,result_json FROM purchase_cycle_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &kind, &body)
	if errors.Is(e, sql.ErrNoRows) || e == nil && who != a.IdentityID {
		return Result{}, purchases.ErrNotFound
	}
	if e != nil {
		return Result{}, e
	}
	if e = authorize(ctx, tx, a, d, kind == "finance"); e != nil {
		return Result{}, e
	}
	r, e := verifiedOperation(ctx, tx, a, d, id, body)
	if e != nil {
		return r, e
	}
	return r, tx.Commit()
}
func DeliveryHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (History, error) {
	out := History{Items: []Result{}, Limit: limit, Offset: offset}
	if !key(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	v, e := deliveryTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out.Total = int(v.Revision)
	out.HasMore = offset+limit < out.Total
	rows, e := tx.QueryContext(ctx, `SELECT result_json FROM purchase_cycle_operations WHERE tenant_id=? AND store_id=? AND kind='delivery' AND entity_id=? ORDER BY revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, id, limit, offset)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var b string
		var r Result
		if e = rows.Scan(&b); e != nil {
			return out, e
		}
		if json.Unmarshal([]byte(b), &r) != nil || r.Delivery == nil || r.Payable != nil {
			return out, purchases.ErrConflict
		}
		out.Items = append(out.Items, r)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	return out, tx.Commit()
}
func GetOrigin(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Origin, error) {
	var o Origin
	if !key(id) {
		return o, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, true)
	if e != nil {
		return o, e
	}
	defer tx.Rollback()
	e = tx.QueryRowContext(ctx, `SELECT payable_id,delivery_id,receipt_id,order_id,supplier_id,amount_cents,source_reference,operation_id,created_at FROM payable_receiving_origins WHERE tenant_id=? AND store_id=? AND device_id=? AND payable_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&o.PayableID, &o.DeliveryID, &o.ReceiptID, &o.OrderID, &o.SupplierID, &o.AmountCents, &o.SourceReference, &o.OperationID, &o.CreatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return o, purchases.ErrNotFound
	}
	if e != nil {
		return o, e
	}
	e = payables.CheckReceivingOriginTx(ctx, tx, a, d, id)
	if errors.Is(e, payables.ErrConflict) {
		o.ReviewRequired = true
	} else if e != nil {
		return o, e
	}
	return o, tx.Commit()
}
