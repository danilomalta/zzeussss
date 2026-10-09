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
)

type RejectionInput struct {
	OperationID       string `json:"operation_id"`
	OrderID           string `json:"order_id"`
	DeliveryReference string `json:"delivery_reference"`
	Unit              string `json:"unit"`
	DeliveredMilli    int64  `json:"delivered_milli"`
	Reason            string `json:"reason"`
}
type DeliveryRejection struct {
	RejectionInput
	ID        string `json:"id"`
	DeviceID  string `json:"device_id"`
	ActorID   string `json:"actor_id"`
	ProductID string `json:"product_id"`
	CreatedAt string `json:"created_at"`
}
type RejectionResult struct {
	Rejection DeliveryRejection `json:"rejection"`
	Repeated  bool              `json:"repeated"`
}

func exceptionAudit(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, id, body, now string) error {
	return one(ctx, tx, `INSERT INTO purchase_receiving_exception_events VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, kind, id, a.IdentityID, body, now)
}
func deliveryReferenceUsedTx(ctx context.Context, tx *sql.Tx, a identity.Scope, supplier, ref string) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND supplier_id=? AND delivery_reference=?)+(SELECT count(*) FROM purchase_delivery_rejections WHERE tenant_id=? AND store_id=? AND supplier_id=? AND delivery_reference=?)`, a.TenantID, a.StoreID, supplier, ref, a.TenantID, a.StoreID, supplier, ref).Scan(&n)
	return n != 0, err
}
func rejectionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (DeliveryRejection, error) {
	var out DeliveryRejection
	var body, supplier string
	err := tx.QueryRowContext(ctx, `SELECT id,operation_id,order_id,delivery_reference,unit,delivered_milli,reason,device_id,actor_id,product_id,created_at,request_json,supplier_id FROM purchase_delivery_rejections WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.ID, &out.OperationID, &out.OrderID, &out.DeliveryReference, &out.Unit, &out.DeliveredMilli, &out.Reason, &out.DeviceID, &out.ActorID, &out.ProductID, &out.CreatedAt, &body, &supplier)
	if err != nil {
		return DeliveryRejection{}, err
	}
	canonical, _ := json.Marshal(out.RejectionInput)
	if body != string(canonical) || !validSearchID(out.OperationID) || !validSearchID(out.OrderID) || !validSearchID(out.DeliveryReference) || !validReceivingReason(out.Reason) || out.DeliveredMilli < 1 || out.DeliveredMilli > MaxQuantity {
		return DeliveryRejection{}, ErrConflict
	}
	auth, err := receivingAuthorizationTx(ctx, tx, a, out.OrderID)
	if err != nil {
		return DeliveryRejection{}, err
	}
	if auth == nil {
		return DeliveryRejection{}, ErrConflict
	}
	var n int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receiving_exception_events WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND kind='rejected' AND order_id=? AND actor_id=? AND request_json=? AND created_at=?`, a.TenantID, a.StoreID, out.DeviceID, out.OperationID, out.OrderID, out.ActorID, body, out.CreatedAt).Scan(&n)
	if err != nil {
		return DeliveryRejection{}, err
	}
	if n != 1 {
		return DeliveryRejection{}, ErrConflict
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_order_items i JOIN purchase_orders o ON o.tenant_id=i.tenant_id AND o.store_id=i.store_id AND o.id=i.order_id WHERE i.tenant_id=? AND i.store_id=? AND i.order_id=? AND i.product_id=? AND i.unit=? AND o.supplier_id=? AND o.status='local_not_sent'`, a.TenantID, a.StoreID, out.OrderID, out.ProductID, out.Unit, supplier).Scan(&n)
	if err != nil {
		return DeliveryRejection{}, err
	}
	if n != 1 {
		return DeliveryRejection{}, ErrConflict
	}
	return out, nil
}
func RejectDelivery(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in RejectionInput) (RejectionResult, error) {
	if !validSearchID(in.OperationID) || !validSearchID(in.OrderID) || !validSearchID(in.DeliveryReference) || !validReceivingReason(in.Reason) || in.DeliveredMilli < 1 || in.DeliveredMilli > MaxQuantity || (in.Unit != "unit" && in.Unit != "kg" && in.Unit != "g" && in.Unit != "liter" && in.Unit != "ml" && in.Unit != "meter") {
		return RejectionResult{}, ErrInvalid
	}
	if db == nil {
		return RejectionResult{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return RejectionResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); err != nil {
		return RejectionResult{}, err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageStock, modules.Inventory); err != nil {
		return RejectionResult{}, err
	}
	var id, store string
	err = tx.QueryRowContext(ctx, `SELECT id,store_id FROM purchase_delivery_rejections WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&id, &store)
	if err == nil {
		if store != a.StoreID {
			return RejectionResult{}, ErrConflict
		}
		prior, e := rejectionTx(ctx, tx, a, id)
		if e != nil {
			return RejectionResult{}, e
		}
		if prior.RejectionInput != in || prior.ActorID != a.IdentityID || prior.DeviceID != d.DeviceID {
			return RejectionResult{}, ErrConflict
		}
		return RejectionResult{prior, true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return RejectionResult{}, err
	}
	var supplier, product, unit, status string
	var count int
	err = tx.QueryRowContext(ctx, `SELECT supplier_id,status FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&supplier, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return RejectionResult{}, ErrNotFound
	}
	if err != nil {
		return RejectionResult{}, err
	}
	if status != "local_not_sent" {
		return RejectionResult{}, ErrConflict
	}
	auth, err := receivingAuthorizationTx(ctx, tx, a, in.OrderID)
	if err != nil {
		return RejectionResult{}, err
	}
	if auth == nil {
		return RejectionResult{}, ErrConflict
	}
	cancelled, err := cancellationTx(ctx, tx, a, in.OrderID)
	if err != nil {
		return RejectionResult{}, err
	}
	if cancelled != nil {
		return RejectionResult{}, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&count); err != nil {
		return RejectionResult{}, err
	}
	if count != 1 {
		return RejectionResult{}, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT product_id,unit FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&product, &unit); err != nil {
		return RejectionResult{}, err
	}
	if unit != in.Unit {
		return RejectionResult{}, ErrConflict
	}
	used, err := deliveryReferenceUsedTx(ctx, tx, a, supplier, in.DeliveryReference)
	if err != nil {
		return RejectionResult{}, err
	}
	if used {
		return RejectionResult{}, ErrConflict
	}
	id, err = localdb.NewID()
	if err != nil {
		return RejectionResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	body, _ := json.Marshal(in)
	if err = one(ctx, tx, `INSERT INTO purchase_delivery_rejections VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, id, in.OrderID, d.DeviceID, in.OperationID, a.IdentityID, supplier, in.DeliveryReference, product, in.Unit, in.DeliveredMilli, in.Reason, string(body), now); err != nil {
		return RejectionResult{}, err
	}
	if err = exceptionAudit(ctx, tx, a, d, in.OperationID, "rejected", in.OrderID, string(body), now); err != nil {
		return RejectionResult{}, err
	}
	event, err := localdb.NewID()
	if err != nil {
		return RejectionResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'purchase.delivery.rejected',1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, in.OrderID, string(body), now); err != nil {
		return RejectionResult{}, err
	}
	return RejectionResult{DeliveryRejection{in, id, d.DeviceID, a.IdentityID, product, now}, false}, tx.Commit()
}
