package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type CancelInput struct {
	OperationID    string `json:"operation_id"`
	OrderID        string `json:"order_id"`
	ExpectedStatus string `json:"expected_status"`
	Reason         string `json:"reason"`
}
type Cancellation struct {
	OperationID string `json:"operation_id"`
	DeviceID    string `json:"device_id"`
	ActorID     string `json:"actor_id"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}
type CancelResult struct {
	OrderID      string       `json:"order_id"`
	Status       string       `json:"status"`
	Repeated     bool         `json:"repeated"`
	Cancellation Cancellation `json:"cancellation"`
}

// Original status remains local_not_sent in the immutable creation row.
func cancellationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (*Cancellation, error) {
	var out Cancellation
	var payload string
	err := tx.QueryRowContext(ctx, `SELECT operation_id,device_id,actor_id,reason,created_at,request_json FROM purchase_order_cancellations WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&out.OperationID, &out.DeviceID, &out.ActorID, &out.Reason, &out.CreatedAt, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var in CancelInput
	if json.Unmarshal([]byte(payload), &in) != nil || in.OperationID != out.OperationID || in.OrderID != id || in.Reason != out.Reason || in.ExpectedStatus != "local_not_sent" || !validSearchID(out.OperationID) || len(out.Reason) < 1 || len(out.Reason) > 255 {
		return nil, ErrConflict
	}
	return &out, nil
}
func orderStatusTx(ctx context.Context, tx *sql.Tx, a identity.Scope, v *Order) (*Cancellation, error) {
	if v.Status != "local_not_sent" {
		return nil, ErrConflict
	}
	out, err := cancellationTx(ctx, tx, a, v.ID)
	if err != nil {
		return nil, err
	}
	if out != nil {
		v.Status = "cancelled"
	}
	return out, nil
}
func Cancel(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in CancelInput) (CancelResult, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if !validSearchID(in.OperationID) || !validSearchID(in.OrderID) || in.ExpectedStatus != "local_not_sent" || len(in.Reason) < 1 || len(in.Reason) > 255 || strings.ContainsAny(in.Reason, "\x00\r\n") {
		return CancelResult{}, ErrInvalid
	}
	if db == nil {
		return CancelResult{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CancelResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); err != nil {
		return CancelResult{}, err
	}
	body, err := json.Marshal(in)
	if err != nil {
		return CancelResult{}, err
	}
	var payload, actor, store, id string
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,order_id FROM purchase_order_cancellations WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&payload, &actor, &store, &id)
	if err == nil {
		if payload != string(body) || actor != a.IdentityID || store != a.StoreID || id != in.OrderID {
			return CancelResult{}, ErrConflict
		}
		prior, e := cancellationTx(ctx, tx, a, id)
		if e != nil {
			return CancelResult{}, e
		}
		if prior == nil {
			return CancelResult{}, ErrConflict
		}
		return CancelResult{id, "cancelled", true, *prior}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CancelResult{}, err
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return CancelResult{}, ErrNotFound
	}
	if err != nil {
		return CancelResult{}, err
	}
	if status != in.ExpectedStatus {
		return CancelResult{}, ErrConflict
	}
	prior, err := cancellationTx(ctx, tx, a, in.OrderID)
	if err != nil {
		return CancelResult{}, err
	}
	if prior != nil {
		return CancelResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO purchase_order_cancellations VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.OrderID, d.DeviceID, in.OperationID, a.IdentityID, string(body), in.Reason, now); err != nil {
		return CancelResult{}, err
	}
	event, err := localdb.NewID()
	if err != nil {
		return CancelResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'purchase.order.cancelled',1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, in.OrderID, string(body), now); err != nil {
		return CancelResult{}, err
	}
	return CancelResult{in.OrderID, "cancelled", false, Cancellation{in.OperationID, d.DeviceID, a.IdentityID, in.Reason, now}}, tx.Commit()
}
