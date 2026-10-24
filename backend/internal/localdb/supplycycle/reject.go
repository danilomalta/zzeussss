package supplycycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
)

// Reject closes a named arrived delivery without a receipt or stock/financial
// effect. An explicitly supplied legacy refusal is linked, never recreated.
func Reject(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in FullRejectionInput) (Result, error) {
	if !key(id) || !key(in.OperationID) || in.ExpectedRevision < 1 || in.DeliveredMilli < 1 || in.DeliveredMilli > purchases.MaxQuantity || !text(in.ExistingRejectionID, 128, true) || !text(in.Reason, 255, false) || !text(in.EvidenceReference, 255, false) || !stamp(in.ObservedAt) {
		return Result{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if e = c.RequireTx(ctx, tx, a, d, identity.ManageStock, modules.Inventory); e != nil {
		return Result{}, e
	}
	body := request(id, "reject", in)
	if r, ok, e := prior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	v, e := deliveryTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	if v.State != "arrived" || v.Revision != in.ExpectedRevision || in.DeliveredMilli > v.PlannedMilli {
		return Result{}, purchases.ErrConflict
	}
	order, e := orderTx(ctx, tx, a, v.OrderID)
	if e != nil {
		return Result{}, e
	}
	if order.Cancelled {
		return Result{}, purchases.ErrConflict
	}
	var rejection purchases.DeliveryRejection
	if in.ExistingRejectionID != "" {
		rejection, e = purchases.CheckedRejectionTx(ctx, tx, a, in.ExistingRejectionID)
	} else {
		var r purchases.RejectionResult
		r, e = purchases.RejectDeliveryTx(ctx, tx, c, a, d, purchases.RejectionInput{OperationID: in.OperationID, OrderID: v.OrderID, DeliveryReference: v.DeliveryReference, Unit: v.Unit, DeliveredMilli: in.DeliveredMilli, Reason: in.Reason})
		rejection = r.Rejection
		if e == nil && r.Repeated {
			e = purchases.ErrConflict
		}
	}
	if e != nil {
		return Result{}, e
	}
	if rejection.OrderID != v.OrderID || rejection.DeliveryReference != v.DeliveryReference || rejection.ProductID != v.ProductID || rejection.Unit != v.Unit || rejection.DeliveredMilli != in.DeliveredMilli {
		return Result{}, purchases.ErrConflict
	}
	base, _ := json.Marshal(v)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	v.State = "rejected"
	v.Revision++
	v.UpdatedAt = now
	v.LastReason = in.Reason
	result := Result{OperationID: in.OperationID, Delivery: &v, Rejection: &rejection, RejectionEvidence: &FullRejectionEvidence{FullRejectionInput: in, RecordedBy: a.IdentityID, RecordedDeviceID: d.DeviceID, RecordedAt: now}}
	raw, _ := json.Marshal(result)
	if e = one(ctx, tx, `INSERT INTO purchase_delivery_full_rejections VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, id, rejection.ID, d.DeviceID, in.OperationID, in.ExpectedRevision, string(base), string(raw)); e != nil {
		return Result{}, e
	}
	return finish(ctx, tx, a, d, "delivery", id, body, v.Revision, result)
}

// The old delivery row remains a checked arrived snapshot. Its additive terminal
// ledger must agree with both immutable operation/event and receiving evidence.
func rejectedDeliveryTx(ctx context.Context, tx *sql.Tx, a identity.Scope, base Delivery, payload string) (Delivery, error) {
	var device, op, old, raw, rejectionID string
	var rev int64
	e := tx.QueryRowContext(ctx, `SELECT rejection_id,device_id,operation_id,base_revision,base_payload_json,result_json FROM purchase_delivery_full_rejections WHERE tenant_id=? AND store_id=? AND delivery_id=?`, a.TenantID, a.StoreID, base.ID).Scan(&rejectionID, &device, &op, &rev, &old, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return base, nil
	}
	if e != nil {
		return Delivery{}, e
	}
	if base.State != "arrived" || base.Revision != rev || old != payload {
		return Delivery{}, purchases.ErrConflict
	}
	r, e := verifiedOperation(ctx, tx, a, identity.DeviceContext{DeviceID: device}, op, raw)
	if e != nil {
		return Delivery{}, e
	}
	if r.Delivery == nil || r.Rejection == nil || r.RejectionEvidence == nil || r.Receipt != nil || r.Payable != nil || r.Origin != nil || r.Action != nil || r.Decision != nil {
		return Delivery{}, purchases.ErrConflict
	}
	proof := r.RejectionEvidence
	if proof.ExpectedRevision != rev || proof.OperationID != op || proof.RecordedDeviceID != device || proof.RecordedBy == "" || proof.RecordedAt == "" || proof.DeliveredMilli < 1 || proof.DeliveredMilli > base.PlannedMilli || (proof.ExistingRejectionID != "" && proof.ExistingRejectionID != rejectionID) {
		return Delivery{}, purchases.ErrConflict
	}
	want := base
	want.State = "rejected"
	want.Revision++
	want.UpdatedAt = proof.RecordedAt
	want.LastReason = proof.Reason
	if !reflect.DeepEqual(r.Delivery, &want) {
		return Delivery{}, purchases.ErrConflict
	}
	rejection, e := purchases.CheckedRejectionTx(ctx, tx, a, rejectionID)
	if e != nil {
		return Delivery{}, e
	}
	if !reflect.DeepEqual(r.Rejection, &rejection) || rejection.OrderID != base.OrderID || rejection.DeliveryReference != base.DeliveryReference || rejection.ProductID != base.ProductID || rejection.Unit != base.Unit || rejection.DeliveredMilli != proof.DeliveredMilli {
		return Delivery{}, purchases.ErrConflict
	}
	var n int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_cycle_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND actor_id=? AND kind='delivery' AND entity_id=? AND revision=? AND request_json=? AND result_json=?`, a.TenantID, a.StoreID, device, op, proof.RecordedBy, base.ID, want.Revision, request(base.ID, "reject", proof.FullRejectionInput), raw).Scan(&n)
	if e != nil {
		return Delivery{}, e
	}
	if n != 1 {
		return Delivery{}, purchases.ErrConflict
	}
	return want, nil
}
