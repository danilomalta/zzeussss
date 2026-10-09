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

type ReceivingAuthorizationInput struct {
	OperationID string `json:"operation_id"`
	OrderID     string `json:"order_id"`
	Reference   string `json:"reference"`
	Reason      string `json:"reason"`
}
type ReceivingAuthorization struct {
	OperationID string `json:"operation_id"`
	DeviceID    string `json:"device_id"`
	ActorID     string `json:"actor_id"`
	Reference   string `json:"reference"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}
type ReceivingAuthorizationResult struct {
	OrderID       string                 `json:"order_id"`
	Repeated      bool                   `json:"repeated"`
	Authorization ReceivingAuthorization `json:"authorization"`
}

func receivingAuthorizationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (*ReceivingAuthorization, error) {
	var out ReceivingAuthorization
	var payload string
	err := tx.QueryRowContext(ctx, `SELECT operation_id,device_id,actor_id,reference,reason,created_at,request_json FROM purchase_receiving_authorizations WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&out.OperationID, &out.DeviceID, &out.ActorID, &out.Reference, &out.Reason, &out.CreatedAt, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var in ReceivingAuthorizationInput
	if json.Unmarshal([]byte(payload), &in) != nil || in.OperationID != out.OperationID || in.OrderID != id || in.Reference != out.Reference || in.Reason != out.Reason || !validSearchID(out.OperationID) || !validSearchID(out.Reference) || !validReceivingReason(out.Reason) {
		return nil, ErrConflict
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receiving_events WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND kind='authorized' AND order_id=? AND actor_id=? AND created_at=? AND request_json=?`, a.TenantID, a.StoreID, out.DeviceID, out.OperationID, id, out.ActorID, out.CreatedAt, payload).Scan(&count); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, ErrConflict
	}
	return &out, nil
}
func validReceivingReason(s string) bool {
	return len(s) > 0 && len(s) <= 255 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}

// Authorization is an accountable local human decision, not a supplier confirmation.
func AuthorizeReceiving(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in ReceivingAuthorizationInput) (ReceivingAuthorizationResult, error) {
	if !validSearchID(in.OperationID) || !validSearchID(in.OrderID) || !validSearchID(in.Reference) || !validReceivingReason(in.Reason) {
		return ReceivingAuthorizationResult{}, ErrInvalid
	}
	if db == nil {
		return ReceivingAuthorizationResult{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	body, _ := json.Marshal(in)
	var priorBody, actor, store, id string
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,order_id FROM purchase_receiving_authorizations WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&priorBody, &actor, &store, &id)
	if err == nil {
		if priorBody != string(body) || actor != a.IdentityID || store != a.StoreID || id != in.OrderID {
			return ReceivingAuthorizationResult{}, ErrConflict
		}
		prior, e := receivingAuthorizationTx(ctx, tx, a, id)
		if e != nil {
			return ReceivingAuthorizationResult{}, e
		}
		if prior == nil {
			return ReceivingAuthorizationResult{}, ErrConflict
		}
		return ReceivingAuthorizationResult{id, true, *prior}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ReceivingAuthorizationResult{}, err
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ReceivingAuthorizationResult{}, ErrNotFound
	}
	if err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	if status != "local_not_sent" {
		return ReceivingAuthorizationResult{}, ErrConflict
	}
	cancelled, err := cancellationTx(ctx, tx, a, in.OrderID)
	if err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	if cancelled != nil {
		return ReceivingAuthorizationResult{}, ErrConflict
	}
	prior, err := receivingAuthorizationTx(ctx, tx, a, in.OrderID)
	if err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	if prior != nil {
		return ReceivingAuthorizationResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO purchase_receiving_authorizations VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.OrderID, d.DeviceID, in.OperationID, a.IdentityID, in.Reference, in.Reason, string(body), now); err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	if err = receivingAudit(ctx, tx, a, d, in.OperationID, "authorized", in.OrderID, string(body), now); err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	event, err := localdb.NewID()
	if err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'purchase.receiving.authorized',1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, in.OrderID, string(body), now); err != nil {
		return ReceivingAuthorizationResult{}, err
	}
	return ReceivingAuthorizationResult{in.OrderID, false, ReceivingAuthorization{in.OperationID, d.DeviceID, a.IdentityID, in.Reference, in.Reason, now}}, tx.Commit()
}

func receivingAudit(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, id, body, now string) error {
	return one(ctx, tx, `INSERT INTO purchase_receiving_events VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, kind, id, a.IdentityID, body, now)
}
