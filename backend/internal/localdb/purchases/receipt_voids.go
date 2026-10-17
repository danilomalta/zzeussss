package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stock"
	"titansystem-backend/internal/localdb/stockreservation"
)

type ReceiptVoidInput struct {
	OperationID string `json:"operation_id"`
	OrderID     string `json:"order_id"`
	ReceiptID   string `json:"receipt_id"`
	Reason      string `json:"reason"`
}
type ReceiptVoid struct {
	ReceiptVoidInput
	DeviceID         string `json:"device_id"`
	ActorID          string `json:"actor_id"`
	QuantityMilli    int64  `json:"quantity_milli"`
	Unit             string `json:"unit"`
	ProductID        string `json:"product_id"`
	LocationID       string `json:"location_id"`
	StockOperationID string `json:"stock_operation_id"`
	MovementID       string `json:"movement_id"`
	CreatedAt        string `json:"created_at"`
}
type ReceiptVoidResult struct {
	Void     ReceiptVoid `json:"void"`
	Repeated bool        `json:"repeated"`
}

func receiptVoidTx(ctx context.Context, tx *sql.Tx, a identity.Scope, r Receipt) (*ReceiptVoid, error) {
	var out ReceiptVoid
	var body string
	err := tx.QueryRowContext(ctx, `SELECT operation_id,order_id,receipt_id,reason,device_id,actor_id,quantity_milli,unit,product_id,location_id,stock_operation_id,movement_id,created_at,request_json FROM purchase_receipt_voids WHERE tenant_id=? AND store_id=? AND receipt_id=?`, a.TenantID, a.StoreID, r.ID).Scan(&out.OperationID, &out.OrderID, &out.ReceiptID, &out.Reason, &out.DeviceID, &out.ActorID, &out.QuantityMilli, &out.Unit, &out.ProductID, &out.LocationID, &out.StockOperationID, &out.MovementID, &out.CreatedAt, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	canonical, _ := json.Marshal(out.ReceiptVoidInput)
	if body != string(canonical) || out.ReceiptID != r.ID || out.OrderID != r.OrderID || out.QuantityMilli != r.AcceptedMilli || out.ProductID != r.ProductID || out.Unit != r.Unit || out.LocationID != r.LocationID || !validSearchID(out.OperationID) || !validReceivingReason(out.Reason) {
		return nil, ErrConflict
	}
	var n int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receiving_exception_events WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND kind='receipt_voided' AND order_id=? AND actor_id=? AND request_json=? AND created_at=?`, a.TenantID, a.StoreID, out.DeviceID, out.OperationID, out.OrderID, out.ActorID, body, out.CreatedAt).Scan(&n)
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrConflict
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_operations o JOIN stock_operation_movements l ON l.operation_id=o.id JOIN stock_movements m ON m.id=l.movement_id WHERE o.id=? AND m.id=? AND o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.actor_identity_id=? AND o.kind='loss' AND o.product_id=? AND o.from_location_id=? AND o.to_location_id IS NULL AND o.quantity_milli=? AND o.reason=? AND o.occurred_at=? AND m.tenant_id=o.tenant_id AND m.store_id=o.store_id AND m.device_id=o.device_id AND m.product_id=o.product_id AND m.location_id=o.from_location_id AND m.quantity_milli=-o.quantity_milli AND m.reason=o.reason AND m.created_at=o.occurred_at`, out.StockOperationID, out.MovementID, a.TenantID, a.StoreID, out.DeviceID, out.ActorID, out.ProductID, out.LocationID, out.QuantityMilli, out.Reason, out.CreatedAt).Scan(&n)
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_operation_movements WHERE operation_id=?`, out.StockOperationID).Scan(&n); err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, ErrConflict
	}
	return &out, nil
}
func VoidReceipt(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in ReceiptVoidInput) (ReceiptVoidResult, error) {
	if !validSearchID(in.OperationID) || !validSearchID(in.OrderID) || !validSearchID(in.ReceiptID) || !validReceivingReason(in.Reason) {
		return ReceiptVoidResult{}, ErrInvalid
	}
	if db == nil {
		return ReceiptVoidResult{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ReceiptVoidResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); err != nil {
		return ReceiptVoidResult{}, err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageStock, modules.Inventory); err != nil {
		return ReceiptVoidResult{}, err
	}
	var priorID, priorStore string
	err = tx.QueryRowContext(ctx, `SELECT receipt_id,store_id FROM purchase_receipt_voids WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&priorID, &priorStore)
	if err == nil {
		if priorStore != a.StoreID || priorID != in.ReceiptID {
			return ReceiptVoidResult{}, ErrConflict
		}
		r, e := receiptTx(ctx, tx, a, priorID)
		if e != nil {
			return ReceiptVoidResult{}, e
		}
		v, e := receiptVoidTx(ctx, tx, a, r)
		if e != nil {
			return ReceiptVoidResult{}, e
		}
		if v == nil || v.ReceiptVoidInput != in || v.ActorID != a.IdentityID || v.DeviceID != d.DeviceID {
			return ReceiptVoidResult{}, ErrConflict
		}
		return ReceiptVoidResult{*v, true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ReceiptVoidResult{}, err
	}
	r, err := receiptTx(ctx, tx, a, in.ReceiptID)
	if errors.Is(err, sql.ErrNoRows) {
		return ReceiptVoidResult{}, ErrNotFound
	}
	if err != nil {
		return ReceiptVoidResult{}, err
	}
	if r.OrderID != in.OrderID {
		return ReceiptVoidResult{}, ErrNotFound
	}
	v, err := receiptVoidTx(ctx, tx, a, r)
	if err != nil {
		return ReceiptVoidResult{}, err
	}
	if v != nil {
		return ReceiptVoidResult{}, ErrConflict
	}
	var unit string
	if err = tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, r.ProductID).Scan(&unit); err != nil {
		return ReceiptVoidResult{}, err
	}
	if unit != r.Unit {
		return ReceiptVoidResult{}, ErrConflict
	}
	var balance int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, a.TenantID, a.StoreID, r.ProductID, r.LocationID).Scan(&balance); err != nil {
		return ReceiptVoidResult{}, ErrConflict
	}
	free, err := stockreservation.FreeTx(ctx, tx, a.TenantID, a.StoreID, r.ProductID, r.LocationID, balance)
	if err != nil || free < r.AcceptedMilli {
		return ReceiptVoidResult{}, ErrConflict
	}
	stockID, err := localdb.NewID()
	if err != nil {
		return ReceiptVoidResult{}, err
	}
	movementID, err := localdb.NewID()
	if err != nil {
		return ReceiptVoidResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	body, _ := json.Marshal(in)
	if err = one(ctx, tx, `INSERT INTO stock_operations(id,tenant_id,store_id,device_id,actor_identity_id,kind,product_id,from_location_id,to_location_id,quantity_milli,reason,occurred_at) VALUES(?,?,?,?,?,'loss',?,?,NULL,?,?,?)`, stockID, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, r.ProductID, r.LocationID, r.AcceptedMilli, in.Reason, now); err != nil {
		return ReceiptVoidResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO stock_movements VALUES(?,?,?,?,?,?,?,?,?)`, movementID, a.TenantID, a.StoreID, d.DeviceID, r.ProductID, r.LocationID, -r.AcceptedMilli, in.Reason, now); err != nil {
		return ReceiptVoidResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO stock_operation_movements VALUES(?,?)`, stockID, movementID); err != nil {
		return ReceiptVoidResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO purchase_receipt_voids VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, r.ID, r.OrderID, d.DeviceID, in.OperationID, a.IdentityID, in.Reason, r.AcceptedMilli, r.Unit, r.ProductID, r.LocationID, stockID, movementID, string(body), now); err != nil {
		return ReceiptVoidResult{}, err
	}
	if err = exceptionAudit(ctx, tx, a, d, in.OperationID, "receipt_voided", in.OrderID, string(body), now); err != nil {
		return ReceiptVoidResult{}, err
	}
	stockBody, _ := json.Marshal(struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		DeviceID string `json:"device_id"`
		ActorID  string `json:"actor_id"`
		stock.Input
	}{a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, stock.Input{OperationID: stockID, Kind: "loss", ProductID: r.ProductID, FromLocationID: r.LocationID, QuantityMilli: r.AcceptedMilli, Reason: in.Reason}})
	for _, e := range []struct{ kind, op, aggregate, body string }{{"stock.operation", stockID, r.ProductID, string(stockBody)}, {"purchase.receipt.voided", in.OperationID, r.OrderID, string(body)}} {
		event, err := localdb.NewID()
		if err != nil {
			return ReceiptVoidResult{}, err
		}
		if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, e.op, e.aggregate, e.kind, e.body, now); err != nil {
			return ReceiptVoidResult{}, err
		}
	}
	out := ReceiptVoid{in, d.DeviceID, a.IdentityID, r.AcceptedMilli, r.Unit, r.ProductID, r.LocationID, stockID, movementID, now}
	return ReceiptVoidResult{out, false}, tx.Commit()
}
