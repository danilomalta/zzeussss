package purchases

import (
	"context"
	"database/sql"
	"math/big"
	"titansystem-backend/internal/localdb/identity"
)

type ReceivingHistoryTotals struct {
	PlannedMilli               int64  `json:"planned_milli"`
	EffectiveAcceptedMilli     int64  `json:"effective_accepted_milli"`
	EffectiveDeliveredMilli    int64  `json:"effective_delivered_milli"`
	PartialRejectedMilli       int64  `json:"partial_rejected_milli"`
	RemainingMilli             int64  `json:"remaining_milli"`
	FullyRejectedMilliExact    string `json:"fully_rejected_milli_exact"`
	VoidedAcceptedMilliExact   string `json:"voided_accepted_milli_exact"`
	RecordedAcceptedMilliExact string `json:"recorded_accepted_milli_exact"`
}
type ReceivingHistoryEntry struct {
	Kind      string             `json:"kind"`
	ID        string             `json:"id"`
	CreatedAt string             `json:"created_at"`
	Receipt   *Receipt           `json:"receipt,omitempty"`
	Rejection *DeliveryRejection `json:"rejection,omitempty"`
	Void      *ReceiptVoid       `json:"void,omitempty"`
}
type ReceivingHistoryPage struct {
	OrderID          string                  `json:"order_id"`
	CommercialStatus string                  `json:"commercial_status"`
	ReceivingStatus  string                  `json:"receiving_status"`
	ProductID        string                  `json:"product_id"`
	Unit             string                  `json:"unit"`
	Totals           ReceivingHistoryTotals  `json:"totals"`
	TotalCount       int64                   `json:"total_count"`
	Offset           int64                   `json:"offset"`
	Limit            int                     `json:"limit"`
	HasMore          bool                    `json:"has_more"`
	Items            []ReceivingHistoryEntry `json:"items"`
}

// The whole ledger is validated before pagination. Totals are never page subtotals.
func ReceivingHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int64) (ReceivingHistoryPage, error) {
	if !validSearchID(id) || offset < 0 || offset > MaxQuantity {
		return ReceivingHistoryPage{}, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return ReceivingHistoryPage{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
		return ReceivingHistoryPage{}, err
	}
	base, err := receivingTraceTx(ctx, tx, a, id, 0)
	if err != nil {
		return ReceivingHistoryPage{}, err
	}
	out := ReceivingHistoryPage{OrderID: id, CommercialStatus: base.CommercialStatus, ReceivingStatus: base.ReceivingStatus, ProductID: base.ProductID, Unit: base.Unit, Offset: offset, Limit: 50, Items: []ReceivingHistoryEntry{}, Totals: ReceivingHistoryTotals{PlannedMilli: base.PlannedMilli, EffectiveAcceptedMilli: base.AcceptedMilli, EffectiveDeliveredMilli: base.DeliveredMilli, PartialRejectedMilli: base.RejectedMilli, RemainingMilli: base.RemainingMilli, VoidedAcceptedMilliExact: base.VoidedAcceptedMilliExact}}
	rows, err := tx.QueryContext(ctx, `SELECT 'received' AS kind,id,created_at FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND order_id=? UNION ALL SELECT 'rejected',id,created_at FROM purchase_delivery_rejections WHERE tenant_id=? AND store_id=? AND order_id=? UNION ALL SELECT 'voided',receipt_id,created_at FROM purchase_receipt_voids WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY created_at,id,kind`, a.TenantID, a.StoreID, id, a.TenantID, a.StoreID, id, a.TenantID, a.StoreID, id)
	if err != nil {
		return ReceivingHistoryPage{}, err
	}
	defer rows.Close()
	entries := []ReceivingHistoryEntry{}
	for rows.Next() {
		var e ReceivingHistoryEntry
		if err = rows.Scan(&e.Kind, &e.ID, &e.CreatedAt); err != nil {
			return ReceivingHistoryPage{}, err
		}
		entries = append(entries, e)
	}
	if err = rows.Err(); err != nil {
		return ReceivingHistoryPage{}, err
	}
	if err = rows.Close(); err != nil {
		return ReceivingHistoryPage{}, err
	}
	out.TotalCount = int64(len(entries))
	if out.TotalCount > MaxQuantity {
		return ReceivingHistoryPage{}, ErrConflict
	}
	fullyRejected, recordedAccepted := new(big.Int), new(big.Int)
	for index, e := range entries {
		switch e.Kind {
		case "received":
			r, err := receiptTx(ctx, tx, a, e.ID)
			if err != nil {
				return ReceivingHistoryPage{}, err
			}
			if r.OrderID != id || r.CreatedAt != e.CreatedAt {
				return ReceivingHistoryPage{}, ErrConflict
			}
			r.Void, err = receiptVoidTx(ctx, tx, a, r)
			if err != nil {
				return ReceivingHistoryPage{}, err
			}
			e.Receipt = &r
			recordedAccepted.Add(recordedAccepted, big.NewInt(r.AcceptedMilli))
		case "rejected":
			r, err := rejectionTx(ctx, tx, a, e.ID)
			if err != nil {
				return ReceivingHistoryPage{}, err
			}
			if r.OrderID != id || r.CreatedAt != e.CreatedAt || r.ProductID != base.ProductID || r.Unit != base.Unit {
				return ReceivingHistoryPage{}, ErrConflict
			}
			e.Rejection = &r
			fullyRejected.Add(fullyRejected, big.NewInt(r.DeliveredMilli))
		case "voided":
			r, err := receiptTx(ctx, tx, a, e.ID)
			if err != nil {
				return ReceivingHistoryPage{}, err
			}
			v, err := receiptVoidTx(ctx, tx, a, r)
			if err != nil {
				return ReceivingHistoryPage{}, err
			}
			if v == nil || v.OrderID != id || v.CreatedAt != e.CreatedAt {
				return ReceivingHistoryPage{}, ErrConflict
			}
			e.Void = v
		default:
			return ReceivingHistoryPage{}, ErrConflict
		}
		if int64(index) >= offset && len(out.Items) < out.Limit {
			out.Items = append(out.Items, e)
		}
	}
	out.Totals.FullyRejectedMilliExact = fullyRejected.String()
	out.Totals.RecordedAcceptedMilliExact = recordedAccepted.String()
	out.HasMore = offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-offset
	return out, tx.Commit()
}
