package purchases

import (
	"context"
	"database/sql"
	"errors"

	"titansystem-backend/internal/localdb/identity"
)

type CreationAudit struct {
	Kind        string `json:"kind"`
	OperationID string `json:"operation_id"`
	DeviceID    string `json:"device_id"`
	ActorID     string `json:"actor_id"`
	CreatedAt   string `json:"created_at"`
}
type HumanApproval struct {
	SuggestionID string `json:"suggestion_id"`
	OperationID  string `json:"operation_id"`
	DeviceID     string `json:"device_id"`
	ReviewerID   string `json:"reviewer_id"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
	DecidedAt    string `json:"decided_at"`
}
type OrderTrace struct {
	Order        Order         `json:"order"`
	Creation     CreationAudit `json:"creation"`
	Approval     HumanApproval `json:"approval"`
	Cancellation *Cancellation `json:"cancellation,omitempty"`
}

// Trace reads the preserved order and its recorded approval/creation evidence in
// one transaction. Neither a supplier relationship nor an external receipt is inferred.
func Trace(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (OrderTrace, error) {
	if !validSearchID(id) {
		return OrderTrace{}, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return OrderTrace{}, err
	}
	defer tx.Rollback()
	out := OrderTrace{Order: Order{Items: []Item{}}}
	v := &out.Order
	var creator, device string
	err = tx.QueryRowContext(ctx, `SELECT id,operation_id,supplier_id,supplier_name,suggestion_id,status,created_at,approved_by,approved_at,created_by,device_id FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&v.ID, &v.OperationID, &v.SupplierID, &v.SupplierName, &v.SuggestionID, &v.Status, &v.CreatedAt, &v.ApprovedBy, &v.ApprovedAt, &creator, &device)
	if errors.Is(err, sql.ErrNoRows) {
		return OrderTrace{}, ErrNotFound
	}
	if err != nil {
		return OrderTrace{}, err
	}
	out.Cancellation, err = orderStatusTx(ctx, tx, a, v)
	if err != nil {
		return OrderTrace{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT product_id,sku,name,unit,quantity_milli FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY product_id`, a.TenantID, a.StoreID, id)
	if err != nil {
		return OrderTrace{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Item
		if err = rows.Scan(&item.ProductID, &item.SKU, &item.Name, &item.Unit, &item.Quantity); err != nil {
			return OrderTrace{}, err
		}
		if item.Quantity < 1 || item.Quantity > MaxQuantity || !validSearchID(item.ProductID) || (item.Unit != "unit" && item.Unit != "kg" && item.Unit != "g" && item.Unit != "liter" && item.Unit != "ml" && item.Unit != "meter") {
			return OrderTrace{}, ErrConflict
		}
		v.Items = append(v.Items, item)
		if len(v.Items) > 1 {
			return OrderTrace{}, ErrConflict
		}
	}
	if err = rows.Err(); err != nil {
		return OrderTrace{}, err
	}
	if err = rows.Close(); err != nil {
		return OrderTrace{}, err
	}
	if len(v.Items) != 1 {
		return OrderTrace{}, ErrConflict
	}
	var count int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_audit WHERE tenant_id=? AND store_id=? AND kind='purchase.created' AND aggregate_id=?`, a.TenantID, a.StoreID, id).Scan(&count); err != nil {
		return OrderTrace{}, err
	}
	if count != 1 {
		return OrderTrace{}, ErrConflict
	}
	c := &out.Creation
	err = tx.QueryRowContext(ctx, `SELECT kind,operation_id,device_id,actor_id,created_at FROM purchase_audit WHERE tenant_id=? AND store_id=? AND kind='purchase.created' AND aggregate_id=?`, a.TenantID, a.StoreID, id).Scan(&c.Kind, &c.OperationID, &c.DeviceID, &c.ActorID, &c.CreatedAt)
	if err != nil {
		return OrderTrace{}, err
	}
	if c.OperationID != v.OperationID || c.ActorID != creator || c.DeviceID != device || c.CreatedAt != v.CreatedAt {
		return OrderTrace{}, ErrConflict
	}
	r := &out.Approval
	err = tx.QueryRowContext(ctx, `SELECT suggestion_id,operation_id,device_id,reviewer_identity_id,decision,reason,decided_at FROM restock_reviews WHERE tenant_id=? AND store_id=? AND suggestion_id=?`, a.TenantID, a.StoreID, v.SuggestionID).Scan(&r.SuggestionID, &r.OperationID, &r.DeviceID, &r.ReviewerID, &r.Decision, &r.Reason, &r.DecidedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OrderTrace{}, ErrConflict
	}
	if err != nil {
		return OrderTrace{}, err
	}
	if r.Decision != "approved" || r.ReviewerID != v.ApprovedBy || r.DecidedAt != v.ApprovedAt {
		return OrderTrace{}, ErrConflict
	}
	return out, tx.Commit()
}
