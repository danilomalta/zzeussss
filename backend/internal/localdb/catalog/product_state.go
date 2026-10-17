package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type ProductStateInput struct {
	OperationID      string `json:"operation_id"`
	ProductID        string `json:"product_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
}
type ProductState struct {
	ProductID string `json:"product_id"`
	Status    string `json:"status"`
	Revision  int64  `json:"revision"`
	Repeated  bool   `json:"repeated"`
}

func ProductStateTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (ProductState, error) {
	out := ProductState{ProductID: id, Status: "active"}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM products WHERE tenant_id=? AND id=?`, a.TenantID, id).Scan(&n); err != nil {
		return out, err
	}
	if n != 1 {
		return out, ErrEditNotFound
	}
	err := tx.QueryRowContext(ctx, `SELECT status,revision FROM catalog_product_states WHERE tenant_id=? AND product_id=?`, a.TenantID, id).Scan(&out.Status, &out.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	return out, err
}
func GetProductState(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (ProductState, error) {
	if !validEditID(id) {
		return ProductState{}, ErrInvalidCatalog
	}
	tx, err := productStateReadTx(ctx, db, a, d)
	if err != nil {
		return ProductState{}, err
	}
	defer tx.Rollback()
	out, err := ProductStateTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func ChangeProductState(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in ProductStateInput) (ProductState, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if !validEditID(in.OperationID) || !validEditID(in.ProductID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= 2147483647 || (in.Status != "active" && in.Status != "inactive") || len(in.Reason) == 0 || len(in.Reason) > 255 || strings.ContainsAny(in.Reason, "\x00\r\n") {
		return ProductState{}, ErrInvalidCatalog
	}
	if db == nil {
		return ProductState{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ProductState{}, err
	}
	defer tx.Rollback()
	if err = inventoryAuthorization(license)(ctx, tx, a, d); err != nil {
		return ProductState{}, err
	}
	b, err := json.Marshal(in)
	if err != nil {
		return ProductState{}, err
	}
	var saved, actor, store, status string
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,after_status,revision FROM catalog_product_state_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&saved, &actor, &store, &status, &revision)
	if err == nil {
		if saved != string(b) || actor != a.IdentityID || store != a.StoreID {
			return ProductState{}, ErrEditConflict
		}
		return ProductState{in.ProductID, status, revision, true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ProductState{}, err
	}
	prior, err := ProductStateTx(ctx, tx, a, in.ProductID)
	if err != nil {
		return ProductState{}, err
	}
	if prior.Revision != in.ExpectedRevision || prior.Status == in.Status {
		return ProductState{}, ErrEditConflict
	}
	revision = prior.Revision + 1
	if prior.Revision == 0 {
		err = editOne(ctx, tx, `INSERT INTO catalog_product_states VALUES(?,?,?,?)`, a.TenantID, in.ProductID, in.Status, revision)
	} else {
		err = editOne(ctx, tx, `UPDATE catalog_product_states SET status=?,revision=? WHERE tenant_id=? AND product_id=? AND revision=?`, in.Status, revision, a.TenantID, in.ProductID, prior.Revision)
	}
	if err != nil {
		return ProductState{}, err
	}
	err = editOne(ctx, tx, `INSERT INTO catalog_product_state_events VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, a.IdentityID, in.ProductID, string(b), prior.Status, in.Status, revision, in.Reason, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return ProductState{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return ProductState{}, err
	}
	if err = editOne(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'catalog.product.state.changed',1,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, in.ProductID, string(b), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return ProductState{}, err
	}
	return ProductState{in.ProductID, in.Status, revision, false}, tx.Commit()
}

// These Tx helpers require the writer's authorization to have already succeeded.
var ErrProductInactive = errors.New("produto inativo")

func RequireActiveProductTx(ctx context.Context, tx *sql.Tx, tenant, id string) error {
	out, err := ProductStateTx(ctx, tx, identity.Scope{TenantID: tenant}, id)
	if err != nil {
		return err
	}
	if out.Status != "active" {
		return ErrProductInactive
	}
	return nil
}
func productStateReadTx(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (*sql.Tx, error) {
	if db == nil {
		return nil, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewCatalog); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}
