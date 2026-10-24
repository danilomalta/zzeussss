package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stock"
)

func returnCaseTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (ReceivingCase, error) {
	v, e := caseGetTx(ctx, tx, a, id)
	if e != nil {
		return v, e
	}
	if v.Status != "open" && v.Status != "in_progress" {
		return v, ErrConflict
	}
	if v.RequestedOutcome != "collection" && v.RequestedOutcome != "replacement" && v.RequestedOutcome != "credit" && v.RequestedOutcome != "refund" {
		return v, ErrConflict
	}
	return v, nil
}
func OpenSupplierReturn(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, caseID string, in ReturnOpenInput) (ReturnResult, error) {
	if !returnKey(caseID) || !returnKey(in.OperationID) || !returnKey(in.ReturnID) || !returnKey(in.ReceiptID) || in.QuantityMilli < 1 || in.QuantityMilli > MaxQuantity || !returnText(in.Reason, 255, false) {
		return ReturnResult{}, ErrInvalid
	}
	tx, e := returnBegin(ctx, db, c, a, d, false)
	if e != nil {
		return ReturnResult{}, e
	}
	defer tx.Rollback()
	body := returnRequest(caseID, "open", in)
	if r, ok, e := returnPrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	issue, e := returnCaseTx(ctx, tx, a, caseID)
	if e != nil {
		return ReturnResult{}, e
	}
	receipt, e := ReceiptEvidenceTx(ctx, tx, a, d, in.ReceiptID)
	if errors.Is(e, sql.ErrNoRows) {
		return ReturnResult{}, ErrNotFound
	}
	if e != nil {
		return ReturnResult{}, e
	}
	if receipt.Void != nil || receipt.OrderID != issue.OrderID || receipt.ProductID != issue.OpeningEvidence.ProductID || receipt.Unit != issue.OpeningEvidence.Unit {
		return ReturnResult{}, ErrConflict
	}
	others, e := returnSourceSetTx(ctx, tx, a, ` AND r.receipt_id=?`, in.ReceiptID)
	if e != nil {
		return ReturnResult{}, e
	}
	if len(others) >= MaxSupplierReturns {
		return ReturnResult{}, ErrReturnLimit
	}
	var used int64
	for _, v := range others {
		var qty int64
		switch v.State {
		case "requested":
			qty = v.RequestedMilli
		case "approved", "scheduled":
			qty = v.AuthorizedMilli
		case "dispatched", "completed":
			qty = v.DispatchedMilli - v.RecoveredMilli
		}
		if qty < 0 || qty > receipt.AcceptedMilli-used {
			return ReturnResult{}, ErrConflict
		}
		used += qty
	}
	if in.QuantityMilli > receipt.AcceptedMilli-used {
		return ReturnResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	v := SupplierReturn{ID: in.ReturnID, CaseID: caseID, ReceiptID: in.ReceiptID, OrderID: receipt.OrderID, SupplierID: issue.SupplierID, ProductID: receipt.ProductID, Unit: receipt.Unit, RequestedMilli: in.QuantityMilli, State: "requested", Revision: 1, CreatedBy: a.IdentityID, CreatedAt: now, UpdatedAt: now, LastReason: in.Reason}
	raw, _ := json.Marshal(v)
	if e = one(ctx, tx, `INSERT INTO purchase_supplier_returns VALUES(?,?,?,?,?,?,?,?,?,?,0,0,0,0,'requested',1,?)`, a.TenantID, a.StoreID, v.ID, v.CaseID, v.ReceiptID, v.OrderID, v.SupplierID, v.ProductID, v.Unit, v.RequestedMilli, string(raw)); e != nil {
		return ReturnResult{}, e
	}
	return returnFinish(ctx, tx, a, d, body, ReturnResult{OperationID: in.OperationID, Return: v, Action: "open"})
}
func validReturnAction(in ReturnActionInput) bool {
	if !returnKey(in.OperationID) || in.ExpectedRevision < 1 || in.ExpectedRevision >= 4096 || in.QuantityMilli < 0 || in.QuantityMilli > MaxQuantity || !returnText(in.Contact, 255, false) || !returnText(in.EvidenceReference, 255, false) || !returnObserved(in.ObservedAt) || !returnText(in.CollectionMode, 128, true) || !returnText(in.PairedDeliveryID, 128, true) || !returnText(in.RouteReference, 255, true) || !returnText(in.Deadline, 64, true) || !returnText(in.FinancialInstruction, 128, true) || !returnText(in.Reason, 255, false) {
		return false
	}
	if in.Action != "schedule" && (in.CollectionMode != "" || in.PairedDeliveryID != "" || in.RouteReference != "" || in.Deadline != "") {
		return false
	}
	if in.Action != "approve" && in.FinancialInstruction != "" {
		return false
	}
	if in.Action != "approve" && in.Action != "supplier_receive" && in.QuantityMilli != 0 {
		return false
	}
	return in.Action == "approve" || in.Action == "decline" || in.Action == "schedule" || in.Action == "cancel" || in.Action == "supplier_receive"
}
func returnPairedTx(ctx context.Context, tx *sql.Tx, a identity.Scope, v SupplierReturn, arrived bool) error {
	if v.CollectionMode == "relationship_end" {
		if v.RouteReference == "" || v.PairedDeliveryID != "" {
			return ErrConflict
		}
		return nil
	}
	if v.CollectionMode != "next_delivery" || v.PairedDeliveryID == "" {
		return ErrConflict
	}
	var closed int
	if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_delivery_full_rejections WHERE tenant_id=? AND store_id=? AND delivery_id=?`, a.TenantID, a.StoreID, v.PairedDeliveryID).Scan(&closed); e != nil {
		return e
	}
	if closed != 0 {
		return ErrConflict
	}
	var supplier, state string
	var receipt sql.NullString
	e := tx.QueryRowContext(ctx, `SELECT d.supplier_id,d.state,d.receipt_id FROM purchase_deliveries d JOIN purchase_cycle_operations o ON o.tenant_id=d.tenant_id AND o.store_id=d.store_id AND o.entity_id=d.id AND o.kind='delivery' AND o.revision=d.revision AND json_extract(o.result_json,'$.delivery')=d.payload_json WHERE d.tenant_id=? AND d.store_id=? AND d.id=?`, a.TenantID, a.StoreID, v.PairedDeliveryID).Scan(&supplier, &state, &receipt)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	if supplier != v.SupplierID || state == "cancelled" || receipt.Valid && receipt.String == v.ReceiptID {
		return ErrConflict
	}
	if arrived && state != "arrived" && state != "received" {
		return ErrConflict
	}
	return nil
}
func ActSupplierReturn(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in ReturnActionInput) (ReturnResult, error) {
	if !returnKey(id) || !validReturnAction(in) {
		return ReturnResult{}, ErrInvalid
	}
	// Commercial decisions and supplier-receipt attestations require a manager;
	// dispatch/recovery below can be performed by authorized stock staff.
	tx, e := returnBegin(ctx, db, c, a, d, true)
	if e != nil {
		return ReturnResult{}, e
	}
	defer tx.Rollback()
	body := returnRequest(id, in.Action, in)
	if r, ok, e := returnPrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	v, e := returnGetTx(ctx, tx, a, id)
	if e != nil {
		return ReturnResult{}, e
	}
	if v.Revision != in.ExpectedRevision {
		return ReturnResult{}, ErrConflict
	}
	if in.Action != "cancel" && in.Action != "decline" && in.Action != "supplier_receive" {
		if _, e = returnCaseTx(ctx, tx, a, v.CaseID); e != nil {
			return ReturnResult{}, e
		}
		r, e := ReceiptEvidenceTx(ctx, tx, a, d, v.ReceiptID)
		if e != nil {
			return ReturnResult{}, e
		}
		if r.Void != nil {
			return ReturnResult{}, ErrConflict
		}
	}
	switch in.Action {
	case "approve":
		if v.State != "requested" || in.QuantityMilli < 1 || in.QuantityMilli > v.RequestedMilli {
			return ReturnResult{}, ErrConflict
		}
		switch in.FinancialInstruction {
		case "replacement_expected", "credit_requested", "refund_requested", "review":
		default:
			return ReturnResult{}, ErrInvalid
		}
		v.AuthorizedMilli = in.QuantityMilli
		v.FinancialInstruction = in.FinancialInstruction
		v.State = "approved"
	case "decline":
		if v.State != "requested" {
			return ReturnResult{}, ErrConflict
		}
		v.State = "declined"
	case "cancel":
		if v.State != "requested" && v.State != "approved" && v.State != "scheduled" {
			return ReturnResult{}, ErrConflict
		}
		v.State = "cancelled"
	case "schedule":
		if v.State != "approved" && v.State != "scheduled" {
			return ReturnResult{}, ErrConflict
		}
		deadline, e := time.Parse(time.RFC3339Nano, in.Deadline)
		if e != nil || deadline.Year() < 2000 || deadline.Year() > 2200 {
			return ReturnResult{}, ErrInvalid
		}
		if in.CollectionMode != "next_delivery" && in.CollectionMode != "relationship_end" {
			return ReturnResult{}, ErrInvalid
		}
		if in.CollectionMode == "next_delivery" && (in.PairedDeliveryID == "" || in.RouteReference != "") {
			return ReturnResult{}, ErrInvalid
		}
		if in.CollectionMode == "relationship_end" && (in.PairedDeliveryID != "" || in.RouteReference == "") {
			return ReturnResult{}, ErrInvalid
		}
		v.CollectionMode = in.CollectionMode
		v.PairedDeliveryID = in.PairedDeliveryID
		v.RouteReference = in.RouteReference
		v.Deadline = in.Deadline
		if e = returnPairedTx(ctx, tx, a, v, false); e != nil {
			return ReturnResult{}, e
		}
		v.State = "scheduled"
	case "supplier_receive":
		if v.State != "dispatched" || in.QuantityMilli < 1 || in.QuantityMilli > v.DispatchedMilli-v.SupplierReceivedMilli-v.RecoveredMilli {
			return ReturnResult{}, ErrConflict
		}
		v.SupplierReceivedMilli += in.QuantityMilli
		if v.SupplierReceivedMilli+v.RecoveredMilli == v.DispatchedMilli {
			v.State = "completed"
		}
	}
	v.Revision++
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	v.LastReason = in.Reason
	if e = returnSave(ctx, tx, a, v, in.ExpectedRevision); e != nil {
		return ReturnResult{}, e
	}
	return returnFinish(ctx, tx, a, d, body, ReturnResult{OperationID: in.OperationID, Return: v, Action: in.Action, ActionEvidence: &in})
}
func SupplierReturnStock(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, action string, in ReturnStockInput) (ReturnResult, error) {
	if !returnKey(id) || !returnKey(in.OperationID) || in.ExpectedRevision < 1 || in.ExpectedRevision >= 4096 || !returnKey(in.LocationID) || in.QuantityMilli < 1 || in.QuantityMilli > MaxQuantity || !returnText(in.HandoverReference, 255, false) || !returnObserved(in.ObservedAt) || !returnText(in.Reason, 255, false) || (action != "dispatch" && action != "recover") {
		return ReturnResult{}, ErrInvalid
	}
	tx, e := returnBegin(ctx, db, c, a, d, false)
	if e != nil {
		return ReturnResult{}, e
	}
	defer tx.Rollback()
	body := returnRequest(id, action, in)
	if r, ok, e := returnPrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	v, e := returnGetTx(ctx, tx, a, id)
	if e != nil {
		return ReturnResult{}, e
	}
	if v.Revision != in.ExpectedRevision {
		return ReturnResult{}, ErrConflict
	}
	source, e := ReceiptEvidenceTx(ctx, tx, a, d, v.ReceiptID)
	if e != nil {
		return ReturnResult{}, e
	}
	stockID, e := localdb.NewID()
	if e != nil {
		return ReturnResult{}, e
	}
	input := stock.Input{OperationID: stockID, ProductID: v.ProductID, ExpectedUnit: v.Unit, QuantityMilli: in.QuantityMilli, Reason: in.Reason}
	if action == "dispatch" {
		if v.State != "scheduled" || in.QuantityMilli != v.AuthorizedMilli || source.Void != nil {
			return ReturnResult{}, ErrConflict
		}
		if _, e = returnCaseTx(ctx, tx, a, v.CaseID); e != nil {
			return ReturnResult{}, e
		}
		if e = returnPairedTx(ctx, tx, a, v, true); e != nil {
			return ReturnResult{}, e
		}
		input.Kind = "loss"
		input.FromLocationID = in.LocationID
		v.DispatchedMilli = in.QuantityMilli
		v.State = "dispatched"
	} else {
		if v.State != "dispatched" || in.QuantityMilli > v.DispatchedMilli-v.SupplierReceivedMilli-v.RecoveredMilli {
			return ReturnResult{}, ErrConflict
		}
		input.Kind = "entry"
		input.ToLocationID = in.LocationID
		v.RecoveredMilli += in.QuantityMilli
		if v.RecoveredMilli+v.SupplierReceivedMilli == v.DispatchedMilli {
			v.State = "completed"
		}
	}
	if _, e = stock.RecordInTxWithContract(ctx, tx, c, a, d, input); e != nil {
		return ReturnResult{}, e
	}
	var now string
	if e = tx.QueryRowContext(ctx, `SELECT occurred_at FROM stock_operations WHERE id=? AND tenant_id=? AND store_id=? AND device_id=?`, stockID, a.TenantID, a.StoreID, d.DeviceID).Scan(&now); e != nil {
		return ReturnResult{}, e
	}
	v.Revision++
	v.UpdatedAt = now
	v.LastReason = in.Reason
	if e = returnSave(ctx, tx, a, v, in.ExpectedRevision); e != nil {
		return ReturnResult{}, e
	}
	return returnFinish(ctx, tx, a, d, body, ReturnResult{OperationID: in.OperationID, Return: v, Action: action, StockOperationID: stockID, StockEvidence: &in})
}
