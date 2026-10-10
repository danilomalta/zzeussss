package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"titansystem-backend/internal/localdb/identity"
)

type EditingDetails struct {
	ProductID string       `json:"product_id"`
	Revision  int64        `json:"revision"`
	Product   ProductInput `json:"product"`
}
type OperationReceipt struct {
	Kind   string          `json:"kind"`
	Input  json.RawMessage `json:"input"`
	Result any             `json:"result"`
}

func catalogManagerRead(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (*sql.Tx, error) {
	tx, e := productStateReadTx(ctx, db, a, d)
	if e != nil {
		return nil, e
	}
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func validEditingProduct(in ProductInput) bool {
	return strings.TrimSpace(in.SKU) == in.SKU && len(in.SKU) > 0 && len(in.SKU) <= 100 && strings.TrimSpace(in.Name) == in.Name && len(in.Name) > 0 && len(in.Name) <= 255 && strings.TrimSpace(in.Barcode) == in.Barcode && len(in.Barcode) <= 128 && validUnit(in.Unit) && in.PriceCents >= 0 && in.PriceCents <= 9007199254740991 && in.CostCents >= 0 && in.CostCents <= 9007199254740991
}
func EditingSnapshot(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (EditingDetails, error) {
	if !validEditID(id) {
		return EditingDetails{}, ErrInvalidCatalog
	}
	tx, e := catalogManagerRead(ctx, db, a, d)
	if e != nil {
		return EditingDetails{}, e
	}
	defer tx.Rollback()
	out := EditingDetails{ProductID: id}
	var code sql.NullString
	e = tx.QueryRowContext(ctx, `SELECT sku,name,unit,barcode,price_cents,cost_cents FROM products WHERE tenant_id=? AND id=?`, a.TenantID, id).Scan(&out.Product.SKU, &out.Product.Name, &out.Product.Unit, &code, &out.Product.PriceCents, &out.Product.CostCents)
	if errors.Is(e, sql.ErrNoRows) {
		return EditingDetails{}, ErrEditNotFound
	}
	if e != nil {
		return EditingDetails{}, e
	}
	out.Product.Barcode = code.String
	out.Revision, e = editRevisionTx(ctx, tx, a.TenantID, id)
	if e != nil {
		return EditingDetails{}, e
	}
	if !validEditingProduct(out.Product) || out.Revision < 0 || out.Revision > 2147483647 {
		return EditingDetails{}, ErrEditConflict
	}
	return out, tx.Commit()
}
func CatalogOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, op string) (OperationReceipt, error) {
	if !validEditID(op) || (kind != "edit" && kind != "state") {
		return OperationReceipt{}, ErrInvalidCatalog
	}
	tx, e := catalogManagerRead(ctx, db, a, d)
	if e != nil {
		return OperationReceipt{}, e
	}
	defer tx.Rollback()
	table := "catalog_edit_events"
	if kind == "state" {
		table = "catalog_product_state_events"
	}
	var body, product string
	var revision int64
	e = tx.QueryRowContext(ctx, `SELECT request_json,product_id,revision FROM `+table+` WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&body, &product, &revision)
	if errors.Is(e, sql.ErrNoRows) {
		return OperationReceipt{}, ErrEditNotFound
	}
	if e != nil {
		return OperationReceipt{}, e
	}
	if !validEditID(product) || revision < 1 || revision > 2147483647 {
		return OperationReceipt{}, ErrEditConflict
	}
	out := OperationReceipt{Kind: kind, Input: json.RawMessage(body)}
	if kind == "edit" {
		var in EditInput
		if json.Unmarshal([]byte(body), &in) != nil || in.OperationID != op || in.ProductID != product || in.ExpectedRevision != revision-1 || !validEditingProduct(in.ProductInput) {
			return OperationReceipt{}, ErrEditConflict
		}
		canonical, e := json.Marshal(in)
		if e != nil || string(canonical) != body {
			return OperationReceipt{}, ErrEditConflict
		}
		current, e := editRevisionTx(ctx, tx, a.TenantID, product)
		if e != nil {
			return OperationReceipt{}, e
		}
		if current < revision {
			return OperationReceipt{}, ErrEditConflict
		}
		out.Result = EditResult{ProductID: product, Revision: revision}
	} else {
		in, e := historicalProductState(body, product, op, revision)
		if e != nil {
			return OperationReceipt{}, e
		}
		var after, reason, before string
		e = tx.QueryRowContext(ctx, `SELECT before_status,after_status,reason FROM catalog_product_state_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, op).Scan(&before, &after, &reason)
		if e != nil {
			return OperationReceipt{}, e
		}
		if after != in.Status || reason != in.Reason || (before != "active" && before != "inactive") || before == after {
			return OperationReceipt{}, ErrEditConflict
		}
		current, e := ProductStateTx(ctx, tx, a, product)
		if e != nil {
			return OperationReceipt{}, e
		}
		if current.Revision < revision {
			return OperationReceipt{}, ErrEditConflict
		}
		out.Result = ProductState{ProductID: product, Status: in.Status, Revision: revision}
	}
	return out, tx.Commit()
}
