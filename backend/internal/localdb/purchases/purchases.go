// Package purchases manages local orders and authorized receiving. It never sends orders to suppliers.
package purchases

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var ErrInvalid = errors.New("pedido invalido")
var ErrConflict = errors.New("operacao ou aprovacao ja utilizada")
var ErrNotFound = errors.New("registro nao encontrado")
var ErrApproval = errors.New("aprovacao ou fornecedor indisponivel")

const MaxQuantity int64 = 9007199254740991

type Input struct {
	OperationID  string `json:"operation_id"`
	OrderID      string `json:"order_id"`
	SupplierID   string `json:"supplier_id"`
	SuggestionID string `json:"suggestion_id"`
}
type SupplierInput struct {
	OperationID string `json:"operation_id"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
}
type Supplier struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
type Result struct {
	ID       string `json:"id"`
	Repeated bool   `json:"repeated"`
}
type Item struct {
	ProductID string `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	Quantity  int64  `json:"quantity_milli"`
}
type Order struct {
	ID              string `json:"id"`
	OperationID     string `json:"operation_id"`
	SupplierID      string `json:"supplier_id"`
	SupplierName    string `json:"supplier_name"`
	SuggestionID    string `json:"suggestion_id"`
	Status          string `json:"status"`
	ReceivingStatus string `json:"receiving_status"`
	CreatedAt       string `json:"created_at"`
	ApprovedBy      string `json:"approved_by"`
	ApprovedAt      string `json:"approved_at"`
	Items           []Item `json:"items"`
}
type Approval struct {
	SuggestionID string `json:"suggestion_id"`
	ProductID    string `json:"product_id"`
	Name         string `json:"name"`
	Unit         string `json:"unit"`
	Quantity     int64  `json:"quantity_milli"`
}

func validID(s string) bool { return len(s) > 0 && len(s) <= 128 && strings.TrimSpace(s) == s }
func one(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, e := tx.ExecContext(ctx, q, args...)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func audit(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, id, now string) error {
	return one(ctx, tx, `INSERT INTO purchase_audit VALUES(?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, kind, id, a.IdentityID, now)
}
func CreateSupplier(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in SupplierInput) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponivel")
	}
	in.Name = strings.TrimSpace(in.Name)
	if !validID(in.OperationID) || !validID(in.ID) || len(in.Name) == 0 || len(in.Name) > 255 || (in.Status != "active" && in.Status != "inactive") {
		return Result{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if e = license.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); e != nil {
		return Result{}, e
	}
	var id, name, status, actor, store string
	e = tx.QueryRowContext(ctx, `SELECT id,name,status,created_by,store_id FROM purchase_supplier_originals WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&id, &name, &status, &actor, &store)
	if e == nil {
		if id != in.ID || name != in.Name || status != in.Status || actor != a.IdentityID || store != a.StoreID {
			return Result{}, ErrConflict
		}
		e = tx.Commit()
		return Result{id, true}, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Result{}, e
	}
	var used int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_supplier_edits WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&used); e != nil {
		return Result{}, e
	}
	if used != 0 {
		return Result{}, ErrConflict
	}
	var exists int
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM purchase_suppliers WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.ID).Scan(&exists); e != nil {
		return Result{}, e
	}
	if exists != 0 {
		return Result{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = one(ctx, tx, `INSERT INTO purchase_suppliers VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.ID, d.DeviceID, in.OperationID, a.IdentityID, in.Name, in.Status, now); e != nil {
		return Result{}, e
	}
	if e = one(ctx, tx, `INSERT INTO purchase_supplier_originals SELECT * FROM purchase_suppliers WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.ID); e != nil {
		return Result{}, e
	}
	if e = audit(ctx, tx, a, d, in.OperationID, "supplier.created", in.ID, now); e != nil {
		return Result{}, e
	}
	e = tx.Commit()
	return Result{in.ID, false}, e
}
func Create(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in Input) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponivel")
	}
	if !validID(in.OperationID) || !validID(in.OrderID) || !validID(in.SupplierID) || !validID(in.SuggestionID) {
		return Result{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if e = license.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); e != nil {
		return Result{}, e
	}
	var id, supplier, suggestion, actor, store string
	e = tx.QueryRowContext(ctx, `SELECT id,supplier_id,suggestion_id,created_by,store_id FROM purchase_orders WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&id, &supplier, &suggestion, &actor, &store)
	if e == nil {
		if id != in.OrderID || supplier != in.SupplierID || suggestion != in.SuggestionID || actor != a.IdentityID || store != a.StoreID {
			return Result{}, ErrConflict
		}
		e = tx.Commit()
		return Result{id, true}, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Result{}, e
	}
	var exists int
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM purchase_orders WHERE tenant_id=? AND (suggestion_id=? OR (store_id=? AND id=?))`, a.TenantID, in.SuggestionID, a.StoreID, in.OrderID).Scan(&exists); e != nil {
		return Result{}, e
	}
	if exists != 0 {
		return Result{}, ErrConflict
	}
	var supplierName, supplierStatus string
	e = tx.QueryRowContext(ctx, `SELECT name,status FROM purchase_suppliers WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.SupplierID).Scan(&supplierName, &supplierStatus)
	if errors.Is(e, sql.ErrNoRows) {
		return Result{}, ErrApproval
	}
	if e != nil {
		return Result{}, e
	}
	if supplierStatus != "active" {
		return Result{}, ErrApproval
	}
	var item Item
	var approvedBy, approvedAt string
	e = tx.QueryRowContext(ctx, `SELECT p.id,p.sku,p.name,p.unit,s.recommended_milli,r.reviewer_identity_id,r.decided_at
 FROM restock_suggestions s JOIN restock_reviews r ON r.tenant_id=s.tenant_id AND r.store_id=s.store_id AND r.suggestion_id=s.id
 JOIN products p ON p.tenant_id=s.tenant_id AND p.id=s.product_id
 WHERE s.tenant_id=? AND s.store_id=? AND s.id=? AND s.status='approved' AND r.decision='approved'`, a.TenantID, a.StoreID, in.SuggestionID).Scan(&item.ProductID, &item.SKU, &item.Name, &item.Unit, &item.Quantity, &approvedBy, &approvedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return Result{}, ErrApproval
	}
	if e != nil {
		return Result{}, e
	}
	if err := catalog.RequireActiveProductTx(ctx, tx, a.TenantID, item.ProductID); err != nil {
		return Result{}, err
	}
	if item.Quantity <= 0 || item.Quantity > MaxQuantity {
		return Result{}, ErrInvalid
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if e = one(ctx, tx, `INSERT INTO purchase_orders VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.OrderID, d.DeviceID, in.OperationID, a.IdentityID, in.SupplierID, supplierName, in.SuggestionID, approvedBy, approvedAt, "local_not_sent", now); e != nil {
		return Result{}, e
	}
	if e = one(ctx, tx, `INSERT INTO purchase_order_items VALUES(?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.OrderID, item.ProductID, item.SKU, item.Name, item.Unit, item.Quantity); e != nil {
		return Result{}, e
	}
	if e = audit(ctx, tx, a, d, in.OperationID, "purchase.created", in.OrderID, now); e != nil {
		return Result{}, e
	}
	e = tx.Commit()
	return Result{in.OrderID, false}, e
}

// Reads preserve access to owned records after contract expiry; they still check
// current membership, store and device inside their transaction.
func ReadTx(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (*sql.Tx, error) {
	if db == nil {
		return nil, errors.New("banco local indisponivel")
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ViewOrders); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func Suppliers(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, offset int) ([]Supplier, error) {
	if offset < 0 {
		return nil, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,name,status FROM purchase_suppliers WHERE tenant_id=? AND store_id=? ORDER BY name,id LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Supplier{}
	for rows.Next() {
		var s Supplier
		if e = rows.Scan(&s.ID, &s.Name, &s.Status); e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	e = tx.Commit()
	return out, e
}
func Approvals(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, offset int) ([]Approval, error) {
	if offset < 0 {
		return nil, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ManageReplenishment); e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT s.id,p.id,p.name,p.unit,s.recommended_milli FROM restock_suggestions s
 JOIN restock_reviews r ON r.tenant_id=s.tenant_id AND r.store_id=s.store_id AND r.suggestion_id=s.id
 JOIN products p ON p.tenant_id=s.tenant_id AND p.id=s.product_id
 WHERE s.tenant_id=? AND s.store_id=? AND s.status='approved' AND r.decision='approved' AND s.recommended_milli BETWEEN 1 AND 9007199254740991
 AND NOT EXISTS(SELECT 1 FROM purchase_orders o WHERE o.tenant_id=s.tenant_id AND o.suggestion_id=s.id)
 ORDER BY s.created_at,s.id LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Approval{}
	for rows.Next() {
		var v Approval
		if e = rows.Scan(&v.SuggestionID, &v.ProductID, &v.Name, &v.Unit, &v.Quantity); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	e = tx.Commit()
	return out, e
}
func Orders(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, offset int) ([]Order, error) {
	if offset < 0 {
		return nil, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,operation_id,supplier_id,supplier_name,suggestion_id,status,created_at,approved_by,approved_at FROM purchase_orders WHERE tenant_id=? AND store_id=? ORDER BY created_at DESC,id DESC LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		var v Order
		if e = rows.Scan(&v.ID, &v.OperationID, &v.SupplierID, &v.SupplierName, &v.SuggestionID, &v.Status, &v.CreatedAt, &v.ApprovedBy, &v.ApprovedAt); e != nil {
			return nil, e
		}
		v.Items = []Item{}
		out = append(out, v)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	for n := range out {
		if _, err := orderStatusTx(ctx, tx, a, &out[n]); err != nil {
			return nil, err
		}
	}
	e = tx.Commit()
	return out, e
}
func Get(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Order, error) {
	if !validID(id) {
		return Order{}, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return Order{}, e
	}
	defer tx.Rollback()
	v := Order{Items: []Item{}}
	e = tx.QueryRowContext(ctx, `SELECT id,operation_id,supplier_id,supplier_name,suggestion_id,status,created_at,approved_by,approved_at FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&v.ID, &v.OperationID, &v.SupplierID, &v.SupplierName, &v.SuggestionID, &v.Status, &v.CreatedAt, &v.ApprovedBy, &v.ApprovedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if e != nil {
		return Order{}, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT product_id,sku,name,unit,quantity_milli FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY product_id`, a.TenantID, a.StoreID, id)
	if e != nil {
		return Order{}, e
	}
	defer rows.Close()
	for rows.Next() {
		var item Item
		if e = rows.Scan(&item.ProductID, &item.SKU, &item.Name, &item.Unit, &item.Quantity); e != nil {
			return Order{}, e
		}
		v.Items = append(v.Items, item)
	}
	if e = rows.Err(); e != nil {
		return Order{}, e
	}
	rows.Close()
	if len(v.Items) != 1 {
		return Order{}, ErrConflict
	}
	if _, err := orderStatusTx(ctx, tx, a, &v); err != nil {
		return Order{}, err
	}
	e = tx.Commit()
	return v, e
}
