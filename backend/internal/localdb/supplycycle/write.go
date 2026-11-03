package supplycycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/purchases"
)

func RecordDecision(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in DecisionInput) (Result, error) {
	if !key(id) || !key(in.OperationID) || in.ExpectedRevision < 0 || in.ExpectedRevision > 1000000 || !text(in.Reason, 500, false) || !text(in.Contact, 255, false) || !text(in.EvidenceReference, 255, false) || !stamp(in.ObservedAt) || (in.Decision != "confirmed" && in.Decision != "declined") || in.AcceptedMilli < 0 || in.AcceptedMilli > purchases.MaxQuantity {
		return Result{}, purchases.ErrInvalid
	}
	promised, e := time.Parse(time.RFC3339Nano, in.PromisedAt)
	if in.Decision == "confirmed" && (in.AcceptedMilli < 1 || e != nil || promised.Year() < 2000 || promised.Year() > 2200) || in.Decision == "declined" && (in.AcceptedMilli != 0 || in.PromisedAt != "") {
		return Result{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	body := request(id, "decision", in)
	if r, ok, e := prior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	o, e := orderTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	if o.Cancelled {
		return Result{}, purchases.ErrConflict
	}
	previous, e := decisionTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	revision := int64(0)
	if previous != nil {
		revision = previous.Revision
	}
	if previous != nil && previous.ExternalVerified {
		return Result{}, purchases.ErrConflict
	}
	if revision != in.ExpectedRevision {
		return Result{}, purchases.ErrConflict
	}
	allocated, e := allocatedTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	if in.AcceptedMilli > o.Planned || in.AcceptedMilli < allocated || in.AcceptedMilli < o.Accepted {
		return Result{}, purchases.ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	v := Decision{DecisionInput: in, OrderID: id, Revision: revision + 1, EvidenceMode: "manual_external", RecordedBy: a.IdentityID, RecordedAt: now}
	b, _ := json.Marshal(v)
	if previous == nil {
		e = one(ctx, tx, `INSERT INTO purchase_supplier_decisions VALUES(?,?,?,?,?)`, a.TenantID, a.StoreID, id, v.Revision, string(b))
	} else {
		e = one(ctx, tx, `UPDATE purchase_supplier_decisions SET revision=?,payload_json=? WHERE tenant_id=? AND store_id=? AND order_id=? AND revision=?`, v.Revision, string(b), a.TenantID, a.StoreID, id, revision)
	}
	if e != nil {
		return Result{}, e
	}
	return finish(ctx, tx, a, d, "decision", id, body, v.Revision, Result{OperationID: in.OperationID, Decision: &v})
}
func Plan(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in PlanInput) (Result, error) {
	if !key(id) || !key(in.OperationID) || !key(in.DeliveryID) || !key(in.DeliveryReference) || in.DecisionRevision < 1 || in.PlannedMilli < 1 || in.PlannedMilli > purchases.MaxQuantity || !text(in.Reason, 500, false) || !text(in.ReceivingCaseID, 128, true) || !text(in.AppointmentID, 128, true) || !text(in.WindowEvidenceReference, 255, true) || (in.AppointmentID == "") != (in.WindowEvidenceReference == "") {
		return Result{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	body := request(id, "plan", in)
	if r, ok, e := prior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	o, e := orderTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	if o.Cancelled {
		return Result{}, purchases.ErrConflict
	}
	dec, e := decisionTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	if dec == nil || dec.Decision != "confirmed" || dec.Revision != in.DecisionRevision {
		return Result{}, purchases.ErrConflict
	}
	allocated, e := allocatedTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	// Named shipments consume disjoint parts of the recorded commitment.
	// Legacy receipts additionally bound what the order can still accept.
	if in.PlannedMilli > dec.AcceptedMilli-allocated || in.PlannedMilli > o.Planned-o.Accepted {
		return Result{}, purchases.ErrConflict
	}
	var n int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_deliveries WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&n)
	if e != nil {
		return Result{}, e
	}
	if n >= MaxDeliveries {
		return Result{}, ErrLimit
	}
	if in.AppointmentID != "" {
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND id=? AND order_id=? AND status IN('reserved','arrived','unloading')`, a.TenantID, a.StoreID, in.AppointmentID, id).Scan(&n)
		if e != nil {
			return Result{}, e
		}
		if n != 1 {
			return Result{}, purchases.ErrConflict
		}
	}
	if in.ReceivingCaseID != "" {
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receiving_cases WHERE tenant_id=? AND store_id=? AND id=? AND order_id=? AND supplier_id=? AND status IN('open','in_progress') AND requested_outcome='replacement'`, a.TenantID, a.StoreID, in.ReceivingCaseID, id, o.Supplier).Scan(&n)
		if e != nil {
			return Result{}, e
		}
		if n != 1 {
			return Result{}, purchases.ErrConflict
		}
	}
	// Preserve supplier-level delivery-reference uniqueness across legacy receiving.
	e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND supplier_id=? AND delivery_reference=?)+(SELECT count(*) FROM purchase_delivery_rejections WHERE tenant_id=? AND store_id=? AND supplier_id=? AND delivery_reference=?)`, a.TenantID, a.StoreID, o.Supplier, in.DeliveryReference, a.TenantID, a.StoreID, o.Supplier, in.DeliveryReference).Scan(&n)
	if e != nil {
		return Result{}, e
	}
	if n != 0 {
		return Result{}, purchases.ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	v := Delivery{ID: in.DeliveryID, OrderID: id, SupplierID: o.Supplier, ProductID: o.Product, Unit: o.Unit, DeliveryReference: in.DeliveryReference, PlannedMilli: in.PlannedMilli, DecisionRevision: in.DecisionRevision, AppointmentID: in.AppointmentID, WindowEvidenceReference: in.WindowEvidenceReference, ReceivingCaseID: in.ReceivingCaseID, State: "planned", Revision: 1, CreatedBy: a.IdentityID, CreatedAt: now, UpdatedAt: now, LastReason: in.Reason}
	b, _ := json.Marshal(v)
	if e = one(ctx, tx, `INSERT INTO purchase_deliveries VALUES(?,?,?,?,?,?,?, ?,?,NULL,?)`, a.TenantID, a.StoreID, v.ID, id, o.Supplier, v.DeliveryReference, v.Revision, v.State, v.PlannedMilli, string(b)); e != nil {
		return Result{}, e
	}
	return finish(ctx, tx, a, d, "delivery", v.ID, body, 1, Result{OperationID: in.OperationID, Delivery: &v})
}
func saveDelivery(ctx context.Context, tx *sql.Tx, a identity.Scope, v Delivery, previous int64) error {
	b, _ := json.Marshal(v)
	var receipt any
	if v.ReceiptID != "" {
		receipt = v.ReceiptID
	}
	return one(ctx, tx, `UPDATE purchase_deliveries SET revision=?,state=?,receipt_id=?,payload_json=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=?`, v.Revision, v.State, receipt, string(b), a.TenantID, a.StoreID, v.ID, previous)
}
func Act(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in ActionInput) (Result, error) {
	if !key(id) || !key(in.OperationID) || in.ExpectedRevision < 1 || !text(in.Reason, 500, false) || !text(in.EvidenceReference, 255, false) || !stamp(in.ObservedAt) || (in.Action != "load" && in.Action != "arrive" && in.Action != "cancel") {
		return Result{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	body := request(id, "action", in)
	if r, ok, e := prior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	v, e := deliveryTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	if v.Revision != in.ExpectedRevision {
		return Result{}, purchases.ErrConflict
	}
	o, e := orderTx(ctx, tx, a, v.OrderID)
	if e != nil {
		return Result{}, e
	}
	if o.Cancelled && in.Action != "cancel" {
		return Result{}, purchases.ErrConflict
	}
	next := ""
	switch {
	case in.Action == "load" && v.State == "planned":
		next = "loaded"
	case in.Action == "arrive" && v.State == "loaded":
		next = "arrived"
	case in.Action == "cancel" && (v.State == "planned" || v.State == "loaded" || v.State == "arrived"):
		next = "cancelled"
	default:
		return Result{}, purchases.ErrConflict
	}
	if in.Action == "cancel" {
		var n int
		e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND supplier_id=? AND delivery_reference=?)+(SELECT count(*) FROM purchase_delivery_rejections WHERE tenant_id=? AND store_id=? AND supplier_id=? AND delivery_reference=?)`, a.TenantID, a.StoreID, v.SupplierID, v.DeliveryReference, a.TenantID, a.StoreID, v.SupplierID, v.DeliveryReference).Scan(&n)
		if e != nil {
			return Result{}, e
		}
		if n != 0 {
			return Result{}, purchases.ErrConflict
		}
	}
	v.State = next
	v.Revision++
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	v.LastReason = in.Reason
	if e = saveDelivery(ctx, tx, a, v, in.ExpectedRevision); e != nil {
		return Result{}, e
	}
	return finish(ctx, tx, a, d, "delivery", id, body, v.Revision, Result{OperationID: in.OperationID, Delivery: &v, Action: &in})
}
func Receive(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in ReceiveInput) (Result, error) {
	if !key(id) || !key(in.OperationID) || in.ExpectedRevision < 1 || !key(in.LocationID) || in.DeliveredMilli < 1 || in.AcceptedMilli < 1 || in.AcceptedMilli > in.DeliveredMilli || !text(in.Reason, 255, false) {
		return Result{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	body := request(id, "receive", in)
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
	r, e := purchases.RecordReceivingTx(ctx, tx, c, a, d, purchases.ReceiptInput{OperationID: in.OperationID, OrderID: v.OrderID, DeliveryReference: v.DeliveryReference, LocationID: in.LocationID, Unit: v.Unit, DeliveredMilli: in.DeliveredMilli, AcceptedMilli: in.AcceptedMilli, Reason: in.Reason})
	if e != nil {
		return Result{}, e
	}
	if r.Repeated {
		return Result{}, purchases.ErrConflict
	}
	v.State = "received"
	v.Revision++
	v.ReceiptID = r.Receipt.ID
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	v.LastReason = in.Reason
	if e = saveDelivery(ctx, tx, a, v, in.ExpectedRevision); e != nil {
		return Result{}, e
	}
	return finish(ctx, tx, a, d, "delivery", id, body, v.Revision, Result{OperationID: in.OperationID, Delivery: &v, Receipt: &r.Receipt})
}
func CreatePayable(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in FinanceInput) (Result, error) {
	if !key(id) || !key(in.OperationID) || in.ExpectedRevision < 1 || !text(in.SourceReference, 255, false) {
		return Result{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, true)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	body := request(id, "finance", in)
	if r, ok, e := prior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	v, e := deliveryTx(ctx, tx, a, id)
	if e != nil {
		return Result{}, e
	}
	if v.State != "received" || v.Revision != in.ExpectedRevision {
		return Result{}, purchases.ErrConflict
	}
	receipt, e := purchases.ReceiptForPayableTx(ctx, tx, a, d, v.ReceiptID)
	if e != nil {
		return Result{}, e
	}
	if receipt.Void != nil {
		return Result{}, purchases.ErrConflict
	}
	input := payables.CreateInput{OperationID: in.OperationID, PayableID: in.PayableID, SupplierID: v.SupplierID, DocumentReference: in.DocumentReference, Description: in.Description, IssuedOn: in.IssuedOn, Currency: in.Currency, TotalCents: in.TotalCents, Installments: in.Installments}
	r, e := payables.CreateTx(ctx, tx, c, a, d, input)
	if e != nil {
		return Result{}, e
	}
	if r.Repeated {
		return Result{}, purchases.ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	o := Origin{PayableID: in.PayableID, DeliveryID: id, ReceiptID: v.ReceiptID, OrderID: v.OrderID, SupplierID: v.SupplierID, AmountCents: in.TotalCents, SourceReference: in.SourceReference, OperationID: in.OperationID, CreatedAt: now}
	raw, _ := json.Marshal(receipt.ReceiptInput)
	if e = one(ctx, tx, `INSERT INTO payable_receiving_origins VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, o.PayableID, id, o.ReceiptID, o.OrderID, o.SupplierID, string(raw), o.AmountCents, o.SourceReference, o.OperationID, now); e != nil {
		return Result{}, e
	}
	if e = payables.CheckReceivingOriginTx(ctx, tx, a, d, o.PayableID); e != nil {
		return Result{}, e
	}
	return finish(ctx, tx, a, d, "finance", in.PayableID, body, 1, Result{OperationID: in.OperationID, Payable: &r.Payable, Origin: &o})
}

// IsConflict allows the HTTP layer to retain the established error vocabulary.
func IsConflict(e error) bool {
	return errors.Is(e, purchases.ErrConflict) || errors.Is(e, payables.ErrConflict) || errors.Is(e, payables.ErrInconsistent)
}
