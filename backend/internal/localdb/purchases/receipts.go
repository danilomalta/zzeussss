package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stock"
	"titansystem-backend/internal/localdb/stockreservation"
)

type ReceiptInput struct {
	OperationID       string `json:"operation_id"`
	OrderID           string `json:"order_id"`
	DeliveryReference string `json:"delivery_reference"`
	LocationID        string `json:"location_id"`
	Unit              string `json:"unit"`
	DeliveredMilli    int64  `json:"delivered_milli"`
	AcceptedMilli     int64  `json:"accepted_milli"`
	Reason            string `json:"reason"`
}
type Receipt struct {
	ReceiptInput
	ID               string `json:"id"`
	DeviceID         string `json:"device_id"`
	ActorID          string `json:"actor_id"`
	ProductID        string `json:"product_id"`
	StockOperationID string `json:"stock_operation_id"`
	MovementID       string `json:"movement_id"`
	CreatedAt        string `json:"created_at"`
}
type ReceiptResult struct {
	Receipt  Receipt `json:"receipt"`
	Repeated bool    `json:"repeated"`
}

func validReceipt(in ReceiptInput) bool {
	return validSearchID(in.OperationID) && validSearchID(in.OrderID) && validSearchID(in.DeliveryReference) && validSearchID(in.LocationID) && validReceivingReason(in.Reason) && in.DeliveredMilli >= 1 && in.DeliveredMilli <= MaxQuantity && in.AcceptedMilli >= 1 && in.AcceptedMilli <= in.DeliveredMilli && (in.Unit == "unit" || in.Unit == "kg" || in.Unit == "g" || in.Unit == "liter" || in.Unit == "ml" || in.Unit == "meter")
}

// Exact accepted totals, bounded by the preserved order. No floating point or conversion.
func receivedQuantitiesTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, planned int64) (int64, int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT accepted_milli,delivered_milli FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	if planned < 1 || planned > MaxQuantity {
		return 0, 0, ErrConflict
	}
	var total, deliveredTotal int64
	for rows.Next() {
		var accepted, delivered int64
		if err = rows.Scan(&accepted, &delivered); err != nil {
			return 0, 0, err
		}
		if accepted < 1 || delivered < accepted || delivered > MaxQuantity-deliveredTotal || accepted > planned-total {
			return 0, 0, ErrConflict
		}
		total += accepted
		deliveredTotal += delivered
	}
	return total, deliveredTotal, rows.Err()
}
func receivedTotalTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, planned int64) (int64, error) {
	total, _, err := receivedQuantitiesTx(ctx, tx, a, id, planned)
	return total, err
}

const receiptColumns = `id,operation_id,order_id,delivery_reference,location_id,unit,delivered_milli,accepted_milli,reason,device_id,actor_id,product_id,stock_operation_id,movement_id,created_at,request_json`

func receiptTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Receipt, error) {
	var out Receipt
	var body string
	err := tx.QueryRowContext(ctx, `SELECT `+receiptColumns+` FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.ID, &out.OperationID, &out.OrderID, &out.DeliveryReference, &out.LocationID, &out.Unit, &out.DeliveredMilli, &out.AcceptedMilli, &out.Reason, &out.DeviceID, &out.ActorID, &out.ProductID, &out.StockOperationID, &out.MovementID, &out.CreatedAt, &body)
	if err != nil {
		return Receipt{}, err
	}
	auth, err := receivingAuthorizationTx(ctx, tx, a, out.OrderID)
	if err != nil {
		return Receipt{}, err
	}
	if auth == nil {
		return Receipt{}, ErrConflict
	}
	cancelled, err := cancellationTx(ctx, tx, a, out.OrderID)
	if err != nil {
		return Receipt{}, err
	}
	if cancelled != nil {
		return Receipt{}, ErrConflict
	}
	canonical, _ := json.Marshal(out.ReceiptInput)
	if !validReceipt(out.ReceiptInput) || body != string(canonical) {
		return Receipt{}, ErrConflict
	}
	var n int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receiving_events WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND kind='received' AND order_id=? AND actor_id=? AND request_json=? AND created_at=?`, a.TenantID, a.StoreID, out.DeviceID, out.OperationID, out.OrderID, out.ActorID, body, out.CreatedAt).Scan(&n)
	if err != nil {
		return Receipt{}, err
	}
	if n != 1 {
		return Receipt{}, ErrConflict
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_operations o JOIN stock_operation_movements l ON l.operation_id=o.id JOIN stock_movements m ON m.id=l.movement_id WHERE o.id=? AND m.id=? AND o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.actor_identity_id=? AND o.kind='entry' AND o.product_id=? AND o.to_location_id=? AND o.from_location_id IS NULL AND o.quantity_milli=? AND o.reason=? AND o.occurred_at=? AND m.tenant_id=o.tenant_id AND m.store_id=o.store_id AND m.device_id=o.device_id AND m.product_id=o.product_id AND m.location_id=o.to_location_id AND m.quantity_milli=o.quantity_milli AND m.reason=o.reason AND m.created_at=o.occurred_at`, out.StockOperationID, out.MovementID, a.TenantID, a.StoreID, out.DeviceID, out.ActorID, out.ProductID, out.LocationID, out.AcceptedMilli, out.Reason, out.CreatedAt).Scan(&n)
	if err != nil {
		return Receipt{}, err
	}
	if n != 1 {
		return Receipt{}, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_operation_movements WHERE operation_id=?`, out.StockOperationID).Scan(&n); err != nil {
		return Receipt{}, err
	}
	if n != 1 {
		return Receipt{}, ErrConflict
	}
	return out, nil
}
func RecordReceiving(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in ReceiptInput) (ReceiptResult, error) {
	if !validReceipt(in) {
		return ReceiptResult{}, ErrInvalid
	}
	if db == nil {
		return ReceiptResult{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ReceiptResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); err != nil {
		return ReceiptResult{}, err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageStock, modules.Inventory); err != nil {
		return ReceiptResult{}, err
	}
	body, _ := json.Marshal(in)
	var id, store string
	err = tx.QueryRowContext(ctx, `SELECT id,store_id FROM purchase_receipts WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&id, &store)
	if err == nil {
		if store != a.StoreID {
			return ReceiptResult{}, ErrConflict
		}
		prior, e := receiptTx(ctx, tx, a, id)
		if e != nil {
			return ReceiptResult{}, e
		}
		if prior.ReceiptInput != in || prior.ActorID != a.IdentityID || prior.DeviceID != d.DeviceID {
			return ReceiptResult{}, ErrConflict
		}
		return ReceiptResult{prior, true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ReceiptResult{}, err
	}
	auth, err := receivingAuthorizationTx(ctx, tx, a, in.OrderID)
	if err != nil {
		return ReceiptResult{}, err
	}
	if auth == nil {
		var exists int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&exists); err != nil {
			return ReceiptResult{}, err
		}
		if exists == 0 {
			return ReceiptResult{}, ErrNotFound
		}
		return ReceiptResult{}, ErrConflict
	}
	cancelled, err := cancellationTx(ctx, tx, a, in.OrderID)
	if err != nil {
		return ReceiptResult{}, err
	}
	if cancelled != nil {
		return ReceiptResult{}, ErrConflict
	}
	var product, supplier, unit, status, currentUnit string
	var planned int64
	err = tx.QueryRowContext(ctx, `SELECT i.product_id,i.unit,i.quantity_milli,o.supplier_id,o.status,p.unit FROM purchase_order_items i JOIN purchase_orders o ON o.tenant_id=i.tenant_id AND o.store_id=i.store_id AND o.id=i.order_id JOIN products p ON p.tenant_id=i.tenant_id AND p.id=i.product_id WHERE i.tenant_id=? AND i.store_id=? AND i.order_id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&product, &unit, &planned, &supplier, &status, &currentUnit)
	if err != nil {
		return ReceiptResult{}, ErrConflict
	}
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&n); err != nil {
		return ReceiptResult{}, err
	}
	if n != 1 || status != "local_not_sent" || unit != in.Unit || unit != currentUnit {
		return ReceiptResult{}, ErrConflict
	}
	total, deliveredTotal, err := receivedQuantitiesTx(ctx, tx, a, in.OrderID, planned)
	if err != nil {
		return ReceiptResult{}, err
	}
	if in.AcceptedMilli > planned-total || in.DeliveredMilli > MaxQuantity-deliveredTotal {
		return ReceiptResult{}, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND supplier_id=? AND delivery_reference=?`, a.TenantID, a.StoreID, supplier, in.DeliveryReference).Scan(&n); err != nil {
		return ReceiptResult{}, err
	}
	if n != 0 {
		return ReceiptResult{}, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_locations WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LocationID).Scan(&n); err != nil {
		return ReceiptResult{}, err
	}
	if n != 1 {
		return ReceiptResult{}, ErrNotFound
	}
	var balance int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, a.TenantID, a.StoreID, product, in.LocationID).Scan(&balance); err != nil {
		return ReceiptResult{}, ErrConflict
	}
	if _, err = stockreservation.FreeTx(ctx, tx, a.TenantID, a.StoreID, product, in.LocationID, balance); err != nil {
		return ReceiptResult{}, ErrConflict
	}
	if balance > math.MaxInt64-in.AcceptedMilli {
		return ReceiptResult{}, ErrConflict
	}
	receiptID, err := localdb.NewID()
	if err != nil {
		return ReceiptResult{}, err
	}
	stockID, err := localdb.NewID()
	if err != nil {
		return ReceiptResult{}, err
	}
	movementID, err := localdb.NewID()
	if err != nil {
		return ReceiptResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO stock_operations(id,tenant_id,store_id,device_id,actor_identity_id,kind,product_id,from_location_id,to_location_id,quantity_milli,reason,occurred_at) VALUES(?,?,?,?,?,'entry',?,NULL,?,?,?,?)`, stockID, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, product, in.LocationID, in.AcceptedMilli, in.Reason, now); err != nil {
		return ReceiptResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO stock_movements VALUES(?,?,?,?,?,?,?,?,?)`, movementID, a.TenantID, a.StoreID, d.DeviceID, product, in.LocationID, in.AcceptedMilli, in.Reason, now); err != nil {
		return ReceiptResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO stock_operation_movements VALUES(?,?)`, stockID, movementID); err != nil {
		return ReceiptResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO purchase_receipts VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, receiptID, in.OrderID, d.DeviceID, in.OperationID, a.IdentityID, supplier, in.DeliveryReference, product, in.Unit, in.LocationID, in.DeliveredMilli, in.AcceptedMilli, in.Reason, stockID, movementID, string(body), now); err != nil {
		return ReceiptResult{}, err
	}
	if err = receivingAudit(ctx, tx, a, d, in.OperationID, "received", in.OrderID, string(body), now); err != nil {
		return ReceiptResult{}, err
	}
	stockBody, _ := json.Marshal(struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		DeviceID string `json:"device_id"`
		ActorID  string `json:"actor_id"`
		stock.Input
	}{a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, stock.Input{OperationID: stockID, Kind: "entry", ProductID: product, ToLocationID: in.LocationID, QuantityMilli: in.AcceptedMilli, Reason: in.Reason}})
	for _, e := range []struct{ kind, op, aggregate, body string }{{"stock.operation", stockID, product, string(stockBody)}, {"purchase.received", in.OperationID, in.OrderID, string(body)}} {
		event, err := localdb.NewID()
		if err != nil {
			return ReceiptResult{}, err
		}
		if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, e.op, e.aggregate, e.kind, e.body, now); err != nil {
			return ReceiptResult{}, err
		}
	}
	out := Receipt{in, receiptID, d.DeviceID, a.IdentityID, product, stockID, movementID, now}
	return ReceiptResult{out, false}, tx.Commit()
}
