package receivables

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitewrite"
)

type ReconcileInput struct {
	DocumentID string `json:"document_id"`
	Action     string `json:"action"`
	Confirmed  bool   `json:"confirmed"`
}

func reconciliationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op string) (Result, error) {
	out := Result{OperationID: op, Repeated: true, Resolution: "not_applied"}
	e := tx.QueryRowContext(ctx, `SELECT document_id,action FROM receivable_reconciliations WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&out.DocumentID, &out.Action)
	if errors.Is(e, sql.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	return out, e
}

// Reconcile never sends the original command or reverses a business effect.
// BEGIN IMMEDIATE serializes its durable fence with every financial mutation.
func Reconcile(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, op string, in ReconcileInput) (Result, error) {
	var out Result
	if db == nil || c == nil || !validID(op) || !validID(in.DocumentID) || !in.Confirmed || (in.Action != "create" && in.Action != "receive_cash" && in.Action != "cancel") {
		return out, ErrInvalid
	}
	e := sqlitewrite.Run(ctx, db, ErrConflict, func(tx *sql.Tx) error {
		if e := fiscal.ValidateSaleSessionTx(ctx, tx, a, d, token); e != nil {
			return e
		}
		if e := c.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.Finance); e != nil {
			return e
		}
		var raw, actor, doc, action string
		e := tx.QueryRowContext(ctx, `SELECT result_json,actor_id,document_id,action FROM receivable_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&raw, &actor, &doc, &action)
		if e == nil {
			if actor != a.IdentityID || doc != in.DocumentID || action != in.Action || json.Unmarshal([]byte(raw), &out) != nil || out.OperationID != op || out.Document.ID != in.DocumentID || out.Action != in.Action || out.Resolution != "" {
				return ErrConflict
			}
			out.Repeated = true
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		var oldActor string
		e = tx.QueryRowContext(ctx, `SELECT actor_id,document_id,action FROM receivable_reconciliations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&oldActor, &doc, &action)
		if e == nil {
			if oldActor != a.IdentityID || doc != in.DocumentID || action != in.Action {
				return ErrConflict
			}
			out, e = reconciliationTx(ctx, tx, a, d, op)
			return e
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		var effects int
		if e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM receivable_cash_receipts WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?)+(SELECT count(*) FROM cash_adjustments WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?)`, a.TenantID, a.StoreID, d.DeviceID, op, a.TenantID, a.StoreID, d.DeviceID, "ar-"+digest([]string{a.TenantID, a.StoreID, d.DeviceID, op})).Scan(&effects); e != nil {
			return e
		}
		if effects != 0 {
			return ErrConflict
		}
		if e = checked(ctx, tx, `INSERT INTO receivable_reconciliations VALUES(?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID, in.DocumentID, in.Action, time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
			return e
		}
		out = Result{OperationID: op, DocumentID: in.DocumentID, Action: in.Action, Resolution: "not_applied"}
		return nil
	})
	return out, e
}
