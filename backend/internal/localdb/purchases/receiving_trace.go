package purchases

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"titansystem-backend/internal/localdb/identity"
)

type ReceivingTracePage struct {
	OrderID                  string                  `json:"order_id"`
	CommercialStatus         string                  `json:"commercial_status"`
	ReceivingStatus          string                  `json:"receiving_status"`
	Authorization            *ReceivingAuthorization `json:"authorization"`
	ProductID                string                  `json:"product_id"`
	Unit                     string                  `json:"unit"`
	PlannedMilli             int64                   `json:"planned_milli"`
	DeliveredMilli           int64                   `json:"delivered_milli"`
	AcceptedMilli            int64                   `json:"accepted_milli"`
	RejectedMilli            int64                   `json:"rejected_milli"`
	RemainingMilli           int64                   `json:"remaining_milli"`
	TotalCount               int64                   `json:"total_count"`
	Offset                   int64                   `json:"offset"`
	Limit                    int                     `json:"limit"`
	HasMore                  bool                    `json:"has_more"`
	VoidedCount              int64                   `json:"voided_count"`
	VoidedAcceptedMilliExact string                  `json:"voided_accepted_milli_exact"`
	Items                    []Receipt               `json:"items"`
}

// A single read transaction validates all receipts, even outside the returned page.
func ReceivingTrace(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int64) (ReceivingTracePage, error) {
	if !validSearchID(id) || offset < 0 || offset > MaxQuantity {
		return ReceivingTracePage{}, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return ReceivingTracePage{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
		return ReceivingTracePage{}, err
	}
	out := ReceivingTracePage{OrderID: id, Offset: offset, Limit: 50, Items: []Receipt{}}
	var rawStatus, supplier string
	err = tx.QueryRowContext(ctx, `SELECT status,supplier_id FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&rawStatus, &supplier)
	if errors.Is(err, sql.ErrNoRows) {
		return ReceivingTracePage{}, ErrNotFound
	}
	if err != nil {
		return ReceivingTracePage{}, err
	}
	var v Order
	v.ID = id
	v.Status = rawStatus
	if _, err = orderStatusTx(ctx, tx, a, &v); err != nil {
		return ReceivingTracePage{}, err
	}
	out.CommercialStatus = v.Status
	out.ReceivingStatus = v.ReceivingStatus
	out.Authorization, err = receivingAuthorizationTx(ctx, tx, a, id)
	if err != nil {
		return ReceivingTracePage{}, err
	}
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&n); err != nil {
		return ReceivingTracePage{}, err
	}
	if n != 1 {
		return ReceivingTracePage{}, ErrConflict
	}
	err = tx.QueryRowContext(ctx, `SELECT product_id,unit,quantity_milli FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&out.ProductID, &out.Unit, &out.PlannedMilli)
	if err != nil {
		return ReceivingTracePage{}, err
	}
	if !validSearchID(out.ProductID) || out.PlannedMilli < 1 || out.PlannedMilli > MaxQuantity || (out.Unit != "unit" && out.Unit != "kg" && out.Unit != "g" && out.Unit != "liter" && out.Unit != "ml" && out.Unit != "meter") {
		return ReceivingTracePage{}, ErrConflict
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,supplier_id FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY created_at,id`, a.TenantID, a.StoreID, id)
	if err != nil {
		return ReceivingTracePage{}, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var rid, rsupplier string
		if err = rows.Scan(&rid, &rsupplier); err != nil {
			return ReceivingTracePage{}, err
		}
		if rsupplier != supplier {
			return ReceivingTracePage{}, ErrConflict
		}
		ids = append(ids, rid)
	}
	if err = rows.Err(); err != nil {
		return ReceivingTracePage{}, err
	}
	if err = rows.Close(); err != nil {
		return ReceivingTracePage{}, err
	}
	out.TotalCount = int64(len(ids))
	if out.TotalCount > MaxQuantity || (out.TotalCount > 0 && out.Authorization == nil) {
		return ReceivingTracePage{}, ErrConflict
	}
	voided := new(big.Int)
	for index, rid := range ids {
		receipt, err := receiptTx(ctx, tx, a, rid)
		if err != nil {
			return ReceivingTracePage{}, err
		}
		if receipt.OrderID != id || receipt.ProductID != out.ProductID || receipt.Unit != out.Unit {
			return ReceivingTracePage{}, ErrConflict
		}
		v, err := receiptVoidTx(ctx, tx, a, receipt)
		if err != nil {
			return ReceivingTracePage{}, err
		}
		receipt.Void = v
		if v == nil {
			if receipt.AcceptedMilli > out.PlannedMilli-out.AcceptedMilli || receipt.DeliveredMilli > MaxQuantity-out.DeliveredMilli {
				return ReceivingTracePage{}, ErrConflict
			}
			out.AcceptedMilli += receipt.AcceptedMilli
			out.DeliveredMilli += receipt.DeliveredMilli
		} else {
			out.VoidedCount++
			voided.Add(voided, big.NewInt(receipt.AcceptedMilli))
		}

		if int64(index) >= offset && len(out.Items) < out.Limit {
			out.Items = append(out.Items, receipt)
		}
	}
	out.VoidedAcceptedMilliExact = voided.String()
	out.RejectedMilli = out.DeliveredMilli - out.AcceptedMilli
	out.RemainingMilli = out.PlannedMilli - out.AcceptedMilli
	out.HasMore = offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-offset
	return out, tx.Commit()
}
