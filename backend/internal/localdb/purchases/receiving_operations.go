package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

type ReceivingOperationReceipt struct {
	Kind   string          `json:"kind"`
	Input  json.RawMessage `json:"input"`
	Result any             `json:"result"`
}

// Original results, scoped to operator/device. Subsequent receipts/voids do not replace them.
// Uses the existing validators for the original audit record and stock links.
func ReceivingOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, op string) (ReceivingOperationReceipt, error) {
	out := ReceivingOperationReceipt{Kind: kind}
	if !validSearchID(op) {
		return out, ErrInvalid
	}
	table, key := "", "order_id"
	switch kind {
	case "authorize":
		table = "purchase_receiving_authorizations"
	case "receipt":
		table = "purchase_receipts"
		key = "id"
	case "reject":
		table = "purchase_delivery_rejections"
		key = "id"
	case "void":
		table = "purchase_receipt_voids"
		key = "receipt_id"
	default:
		return out, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if kind != "authorize" {
		if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
			return out, err
		}
	}
	var target, order, body string
	err = tx.QueryRowContext(ctx, `SELECT `+key+`,order_id,request_json FROM `+table+` WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&target, &order, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	var original any
	switch kind {
	case "authorize":
		v, e := receivingAuthorizationTx(ctx, tx, a, target)
		if e != nil {
			return out, e
		}
		if v == nil || v.OperationID != op || v.ActorID != a.IdentityID || v.DeviceID != d.DeviceID || order != target {
			return out, ErrConflict
		}
		original = ReceivingAuthorizationInput{op, order, v.Reference, v.Reason}
		out.Result = ReceivingAuthorizationResult{order, false, *v}
	case "receipt":
		v, e := receiptTx(ctx, tx, a, target)
		if e != nil {
			return out, ErrConflict
		}
		if v.OrderID != order || v.OperationID != op || v.ActorID != a.IdentityID || v.DeviceID != d.DeviceID {
			return out, ErrConflict
		}
		original = v.ReceiptInput
		out.Result = ReceiptResult{v, false}
	case "reject":
		v, e := rejectionTx(ctx, tx, a, target)
		if e != nil {
			return out, ErrConflict
		}
		if v.OrderID != order || v.OperationID != op || v.ActorID != a.IdentityID || v.DeviceID != d.DeviceID {
			return out, ErrConflict
		}
		original = v.RejectionInput
		out.Result = RejectionResult{v, false}
	case "void":
		r, e := receiptTx(ctx, tx, a, target)
		if e != nil {
			return out, ErrConflict
		}
		v, e := receiptVoidTx(ctx, tx, a, r)
		if e != nil {
			return out, e
		}
		if v == nil || v.OrderID != order || v.OperationID != op || v.ActorID != a.IdentityID || v.DeviceID != d.DeviceID {
			return out, ErrConflict
		}
		original = v.ReceiptVoidInput
		out.Result = ReceiptVoidResult{*v, false}
	}
	canonical, err := json.Marshal(original)
	if err != nil {
		return out, err
	}
	if body != string(canonical) {
		return out, ErrConflict
	}
	out.Input = canonical
	return out, tx.Commit()
}
