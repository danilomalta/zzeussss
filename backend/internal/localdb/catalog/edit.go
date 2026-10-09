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

var ErrEditConflict = errors.New("edicao conflitante")
var ErrEditNotFound = errors.New("produto nao encontrado")

type EditInput struct {
	OperationID      string `json:"operation_id"`
	ProductID        string `json:"product_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	ProductInput
}
type EditResult struct {
	ProductID string `json:"product_id"`
	Revision  int64  `json:"revision"`
	Repeated  bool   `json:"repeated"`
}

func validEditID(s string) bool {
	return len(s) > 0 && len(s) <= 128 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}
func editOne(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrEditConflict
	}
	return nil
}
func editRevisionTx(ctx context.Context, tx *sql.Tx, tenant, product string) (int64, error) {
	var rev int64
	err := tx.QueryRowContext(ctx, `SELECT revision FROM catalog_edit_revisions WHERE tenant_id=? AND product_id=?`, tenant, product).Scan(&rev)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return rev, err
}
func EditState(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (EditResult, error) {
	if !validEditID(id) {
		return EditResult{}, ErrInvalidCatalog
	}
	if db == nil {
		return EditResult{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return EditResult{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
		return EditResult{}, err
	}
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM products WHERE tenant_id=? AND id=?`, a.TenantID, id).Scan(&n); err != nil {
		return EditResult{}, err
	}
	if n != 1 {
		return EditResult{}, ErrEditNotFound
	}
	rev, err := editRevisionTx(ctx, tx, a.TenantID, id)
	if err != nil {
		return EditResult{}, err
	}
	return EditResult{id, rev, false}, tx.Commit()
}

// Unit belongs to the company-wide product; even zero balances retain history.
func unitEditAllowedTx(ctx context.Context, tx *sql.Tx, tenant, product string) (bool, error) {
	for _, query := range []string{
		`SELECT EXISTS(SELECT 1 FROM stock_movements WHERE tenant_id=? AND product_id=?)`,
		`SELECT EXISTS(SELECT 1 FROM inventory_counts WHERE tenant_id=? AND product_id=?)`,
		`SELECT EXISTS(SELECT 1 FROM sale_items WHERE tenant_id=? AND product_id=?)`,
		`SELECT EXISTS(SELECT 1 FROM restock_policies WHERE tenant_id=? AND product_id=?)`,
		`SELECT EXISTS(SELECT 1 FROM restock_suggestions WHERE tenant_id=? AND product_id=?)`,
		`SELECT EXISTS(SELECT 1 FROM purchase_order_items WHERE tenant_id=? AND product_id=?)`,
		`SELECT EXISTS(SELECT 1 FROM production_recipe_versions WHERE tenant_id=? AND output_product_id=?)`,
		`SELECT EXISTS(SELECT 1 FROM production_recipe_ingredients WHERE tenant_id=? AND product_id=?)`,
		`SELECT EXISTS(SELECT 1 FROM production_material_items WHERE tenant_id=? AND product_id=?)`,
	} {
		var used bool
		if err := tx.QueryRowContext(ctx, query, tenant, product).Scan(&used); err != nil {
			return false, err
		}
		if used {
			return false, nil
		}
	}
	return true, nil
}
func Edit(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in EditInput) (EditResult, error) {
	in.SKU, in.Name, in.Barcode = strings.TrimSpace(in.SKU), strings.TrimSpace(in.Name), strings.TrimSpace(in.Barcode)
	if !validEditID(in.OperationID) || !validEditID(in.ProductID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= 2147483647 || len(in.SKU) == 0 || len(in.SKU) > 100 || len(in.Name) == 0 || len(in.Name) > 255 || len(in.Barcode) > 128 || !validUnit(in.Unit) || in.PriceCents < 0 || in.CostCents < 0 || in.PriceCents > 9007199254740991 || in.CostCents > 9007199254740991 {
		return EditResult{}, ErrInvalidCatalog
	}
	if db == nil {
		return EditResult{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return EditResult{}, err
	}
	defer tx.Rollback()
	if err = inventoryAuthorization(license)(ctx, tx, a, d); err != nil {
		return EditResult{}, err
	}
	body, err := json.Marshal(in)
	if err != nil {
		return EditResult{}, err
	}
	var saved, actor, store string
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,revision FROM catalog_edit_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&saved, &actor, &store, &revision)
	if err == nil {
		if saved != string(body) || actor != a.IdentityID || store != a.StoreID {
			return EditResult{}, ErrEditConflict
		}
		return EditResult{in.ProductID, revision, true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return EditResult{}, err
	}
	var prior ProductInput
	var barcode sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT sku,name,unit,barcode,price_cents,cost_cents FROM products WHERE tenant_id=? AND id=?`, a.TenantID, in.ProductID).Scan(&prior.SKU, &prior.Name, &prior.Unit, &barcode, &prior.PriceCents, &prior.CostCents)
	if errors.Is(err, sql.ErrNoRows) {
		return EditResult{}, ErrEditNotFound
	}
	if err != nil {
		return EditResult{}, err
	}
	prior.Barcode = barcode.String
	revision, err = editRevisionTx(ctx, tx, a.TenantID, in.ProductID)
	if err != nil {
		return EditResult{}, err
	}
	if revision != in.ExpectedRevision {
		return EditResult{}, ErrEditConflict
	}
	if in.Unit != prior.Unit {
		allowed, e := unitEditAllowedTx(ctx, tx, a.TenantID, in.ProductID)
		if e != nil {
			return EditResult{}, e
		}
		if !allowed {
			return EditResult{}, ErrEditConflict
		}
	}
	var conflicts int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM products WHERE tenant_id=? AND id<>? AND (sku=? OR (?<>'' AND barcode=?))`, a.TenantID, in.ProductID, in.SKU, in.Barcode, in.Barcode).Scan(&conflicts); err != nil {
		return EditResult{}, err
	}
	if conflicts != 0 {
		return EditResult{}, ErrEditConflict
	}
	var code any
	if in.Barcode != "" {
		code = in.Barcode
	}
	if err = editOne(ctx, tx, `UPDATE products SET sku=?,name=?,unit=?,barcode=?,price_cents=?,cost_cents=? WHERE tenant_id=? AND id=?`, in.SKU, in.Name, in.Unit, code, in.PriceCents, in.CostCents, a.TenantID, in.ProductID); err != nil {
		return EditResult{}, err
	}
	if revision == 0 {
		err = editOne(ctx, tx, `INSERT INTO catalog_edit_revisions VALUES(?,?,?)`, a.TenantID, in.ProductID, 1)
	} else {
		err = editOne(ctx, tx, `UPDATE catalog_edit_revisions SET revision=? WHERE tenant_id=? AND product_id=? AND revision=?`, revision+1, a.TenantID, in.ProductID, revision)
	}
	if err != nil {
		return EditResult{}, err
	}
	revision++
	before, err := json.Marshal(prior)
	if err != nil {
		return EditResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = editOne(ctx, tx, `INSERT INTO catalog_edit_events VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, a.IdentityID, in.ProductID, string(body), string(before), revision, now); err != nil {
		return EditResult{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return EditResult{}, err
	}
	if err = editOne(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'catalog.product.edited',1,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, in.ProductID, string(body), now); err != nil {
		return EditResult{}, err
	}
	return EditResult{in.ProductID, revision, false}, tx.Commit()
}
