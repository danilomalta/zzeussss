package purchases

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type SupplierEditReceipt struct {
	Input        SupplierEditInput `json:"input"`
	Result       SupplierState     `json:"result"`
	BeforeName   string            `json:"before_name"`
	BeforeStatus string            `json:"before_status"`
	ActorID      string            `json:"actor_id"`
	DeviceID     string            `json:"device_id"`
	CreatedAt    string            `json:"created_at"`
}

// Original scoped decision, independent of later changes to the supplier.
func SupplierEditOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, op string) (SupplierEditReceipt, error) {
	var out SupplierEditReceipt
	if !validSearchID(op) {
		return out, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var body, id string
	var rev int64
	err = tx.QueryRowContext(ctx, `SELECT supplier_id,request_json,revision,before_name,before_status,actor_id,device_id,created_at FROM purchase_supplier_edits WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&id, &body, &rev, &out.BeforeName, &out.BeforeStatus, &out.ActorID, &out.DeviceID, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	out.Input, err = historicalSupplierEdit(body, id, op, rev)
	if err != nil {
		return out, err
	}
	if !validSearchID(id) || out.ActorID != a.IdentityID || out.DeviceID != d.DeviceID {
		return out, ErrConflict
	}
	if _, err = time.Parse(time.RFC3339Nano, out.CreatedAt); err != nil {
		return out, ErrConflict
	}
	var name, status string
	if rev == 1 {
		err = tx.QueryRowContext(ctx, `SELECT name,status FROM purchase_supplier_originals WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&name, &status)
	} else {
		var priorBody, priorOp string
		err = tx.QueryRowContext(ctx, `SELECT request_json,operation_id FROM purchase_supplier_edits WHERE tenant_id=? AND store_id=? AND supplier_id=? AND revision=?`, a.TenantID, a.StoreID, id, rev-1).Scan(&priorBody, &priorOp)
		if err == nil {
			var prior SupplierEditInput
			prior, err = historicalSupplierEdit(priorBody, id, priorOp, rev-1)
			name, status = prior.Name, prior.Status
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrConflict
	}
	if err != nil {
		return out, err
	}
	if name != out.BeforeName || status != out.BeforeStatus || (name == out.Input.Name && status == out.Input.Status) {
		return out, ErrConflict
	}
	out.Result = SupplierState{Supplier: Supplier{ID: id, Name: out.Input.Name, Status: out.Input.Status}, Revision: rev}
	return out, tx.Commit()
}
