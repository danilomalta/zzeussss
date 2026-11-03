package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type CancelReceipt struct {
	Input  CancelInput  `json:"input"`
	Result CancelResult `json:"result"`
}

// Read the original cancellation belonging to this operator, device and store.
func CancelOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, op string) (CancelReceipt, error) {
	var out CancelReceipt
	if !validSearchID(op) {
		return out, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var body, id string
	err = tx.QueryRowContext(ctx, `SELECT order_id,request_json FROM purchase_order_cancellations WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&id, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	c, err := cancellationTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	if c == nil || c.OperationID != op || c.DeviceID != d.DeviceID || c.ActorID != a.IdentityID || !validSearchID(id) || !validReceivingReason(c.Reason) {
		return out, ErrConflict
	}
	if _, err = time.Parse(time.RFC3339Nano, c.CreatedAt); err != nil {
		return out, ErrConflict
	}
	if err = json.Unmarshal([]byte(body), &out.Input); err != nil {
		return out, ErrConflict
	}
	canonical, err := json.Marshal(out.Input)
	if err != nil {
		return out, err
	}
	if string(canonical) != body {
		return out, ErrConflict
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrConflict
	}
	if err != nil {
		return out, err
	}
	if status != "local_not_sent" {
		return out, ErrConflict
	}
	auth, err := receivingAuthorizationTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	if auth != nil {
		return out, ErrConflict
	}
	out.Result = CancelResult{OrderID: id, Status: "cancelled", Repeated: false, Cancellation: *c}
	return out, tx.Commit()
}
