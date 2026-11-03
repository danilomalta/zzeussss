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

type SupplierEditInput struct {
	OperationID      string `json:"operation_id"`
	SupplierID       string `json:"supplier_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
}
type SupplierState struct {
	Supplier
	Revision int64 `json:"revision"`
	Repeated bool  `json:"repeated"`
}

func supplierStateTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (SupplierState, error) {
	var out SupplierState
	err := tx.QueryRowContext(ctx, `SELECT s.id,s.name,s.status,COALESCE(r.revision,0) FROM purchase_suppliers s LEFT JOIN purchase_supplier_revisions r ON r.tenant_id=s.tenant_id AND r.store_id=s.store_id AND r.supplier_id=s.id WHERE s.tenant_id=? AND s.store_id=? AND s.id=?`, a.TenantID, a.StoreID, id).Scan(&out.ID, &out.Name, &out.Status, &out.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}
func SupplierEditState(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (SupplierState, error) {
	if !validSearchID(id) {
		return SupplierState{}, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return SupplierState{}, err
	}
	defer tx.Rollback()
	out, err := supplierStateTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func EditSupplier(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in SupplierEditInput) (SupplierState, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Reason = strings.TrimSpace(in.Reason)
	if !validSearchID(in.OperationID) || !validSearchID(in.SupplierID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= 2147483647 || len(in.Name) < 1 || len(in.Name) > 255 || len(in.Reason) < 1 || len(in.Reason) > 255 || strings.ContainsAny(in.Name+in.Reason, "\x00\r\n") || (in.Status != "active" && in.Status != "inactive") {
		return SupplierState{}, ErrInvalid
	}
	if db == nil {
		return SupplierState{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SupplierState{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); err != nil {
		return SupplierState{}, err
	}
	body, err := json.Marshal(in)
	if err != nil {
		return SupplierState{}, err
	}
	var saved, actor, store string
	var rev int64
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,revision FROM purchase_supplier_edits WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&saved, &actor, &store, &rev)
	if err == nil {
		if saved != string(body) || actor != a.IdentityID || store != a.StoreID {
			return SupplierState{}, ErrConflict
		}
		return SupplierState{Supplier: Supplier{in.SupplierID, in.Name, in.Status}, Revision: rev, Repeated: true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return SupplierState{}, err
	}
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_supplier_originals WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&n); err != nil {
		return SupplierState{}, err
	}
	if n != 0 {
		return SupplierState{}, ErrConflict
	}
	before, err := supplierStateTx(ctx, tx, a, in.SupplierID)
	if err != nil {
		return SupplierState{}, err
	}
	if before.Revision != in.ExpectedRevision || (before.Name == in.Name && before.Status == in.Status) {
		return SupplierState{}, ErrConflict
	}
	if err = one(ctx, tx, `UPDATE purchase_suppliers SET name=?,status=? WHERE tenant_id=? AND store_id=? AND id=?`, in.Name, in.Status, a.TenantID, a.StoreID, in.SupplierID); err != nil {
		return SupplierState{}, err
	}
	if before.Revision == 0 {
		err = one(ctx, tx, `INSERT INTO purchase_supplier_revisions VALUES(?,?,?,1)`, a.TenantID, a.StoreID, in.SupplierID)
	} else {
		err = one(ctx, tx, `UPDATE purchase_supplier_revisions SET revision=? WHERE tenant_id=? AND store_id=? AND supplier_id=? AND revision=?`, before.Revision+1, a.TenantID, a.StoreID, in.SupplierID, before.Revision)
	}
	if err != nil {
		return SupplierState{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO purchase_supplier_edits VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, a.IdentityID, in.SupplierID, string(body), before.Name, before.Status, before.Revision+1, now); err != nil {
		return SupplierState{}, err
	}
	event, err := localdb.NewID()
	if err != nil {
		return SupplierState{}, err
	}
	if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'purchase.supplier.edited',1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, in.SupplierID, string(body), now); err != nil {
		return SupplierState{}, err
	}
	return SupplierState{Supplier: Supplier{in.SupplierID, in.Name, in.Status}, Revision: before.Revision + 1}, tx.Commit()
}
