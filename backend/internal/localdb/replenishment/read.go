package replenishment

import (
	"context"
	"database/sql"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

type Product struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Balance  int64  `json:"balance_milli"`
	Minimum  int64  `json:"minimum_milli"`
	Target   int64  `json:"target_milli"`
	Revision int64  `json:"revision"`
}
type Suggestion struct {
	ID          string `json:"id"`
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	Observed    int64  `json:"observed_milli"`
	Recommended int64  `json:"recommended_milli"`
	Revision    int64  `json:"policy_revision"`
	Status      string `json:"status"`
	Created     string `json:"created_at"`
	Stale       bool   `json:"stale"`
	Reason      string `json:"reason"`
	OrderID     string `json:"order_id"`
}

const balanceSQL = `SELECT COALESCE(SUM(m.quantity_milli),0) FROM stock_movements m JOIN stock_locations l ON l.tenant_id=m.tenant_id AND l.store_id=m.store_id AND l.id=m.location_id WHERE m.tenant_id=? AND m.store_id=? AND m.product_id=? AND l.kind IN ('shelf','backroom','receiving')`

func readTx(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, p identity.Permission) (*sql.Tx, error) {
	if db == nil {
		return nil, errors.New("banco indisponível")
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if e = identity.CanOperateTx(ctx, tx, a, d, p); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func Products(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, offset int) ([]Product, error) {
	if offset < 0 {
		return nil, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, identity.ViewOrders)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT p.id,p.name,p.unit,COALESCE(r.minimum_milli,0),COALESCE(r.target_milli,0),COALESCE(r.revision,0) FROM products p LEFT JOIN restock_policies r ON r.tenant_id=p.tenant_id AND r.store_id=? AND r.product_id=p.id WHERE p.tenant_id=? ORDER BY p.name,p.id LIMIT 50 OFFSET ?`, a.StoreID, a.TenantID, offset)
	if e != nil {
		return nil, e
	}
	out := []Product{}
	for rows.Next() {
		var v Product
		if e = rows.Scan(&v.ID, &v.Name, &v.Unit, &v.Minimum, &v.Target, &v.Revision); e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	for i := range out {
		v := &out[i]
		if e = tx.QueryRowContext(ctx, balanceSQL, a.TenantID, a.StoreID, v.ID).Scan(&v.Balance); e != nil {
			return nil, e
		}
		if v.Balance < 0 || v.Balance > MaxExact || v.Target > MaxExact || v.Revision > MaxExact {
			return nil, ErrInvalid
		}
	}
	return out, tx.Commit()
}
func Suggestions(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, offset int) ([]Suggestion, error) {
	if offset < 0 {
		return nil, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, identity.ViewOrders)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT s.id,s.product_id,p.name,p.unit,s.observed_milli,s.recommended_milli,s.policy_revision,s.status,s.created_at,COALESCE(r.reason,''),COALESCE(o.id,'') FROM restock_suggestions s JOIN products p ON p.tenant_id=s.tenant_id AND p.id=s.product_id LEFT JOIN restock_reviews r ON r.tenant_id=s.tenant_id AND r.store_id=s.store_id AND r.suggestion_id=s.id LEFT JOIN purchase_orders o ON o.tenant_id=s.tenant_id AND o.store_id=s.store_id AND o.suggestion_id=s.id WHERE s.tenant_id=? AND s.store_id=? ORDER BY s.created_at DESC,s.id LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, offset)
	if e != nil {
		return nil, e
	}
	out := []Suggestion{}
	for rows.Next() {
		var v Suggestion
		if e = rows.Scan(&v.ID, &v.ProductID, &v.Name, &v.Unit, &v.Observed, &v.Recommended, &v.Revision, &v.Status, &v.Created, &v.Reason, &v.OrderID); e != nil {
			rows.Close()
			return nil, e
		}
		if v.Observed < 0 || v.Observed > MaxExact || v.Recommended < 0 || v.Recommended > MaxExact || v.Revision > MaxExact {
			rows.Close()
			return nil, ErrInvalid
		}
		out = append(out, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	for i := range out {
		v := &out[i]
		if v.Status != "suggested" {
			continue
		}
		var current, rev int64
		if e = tx.QueryRowContext(ctx, balanceSQL, a.TenantID, a.StoreID, v.ProductID).Scan(&current); e != nil {
			return nil, e
		}
		e = tx.QueryRowContext(ctx, `SELECT revision FROM restock_policies WHERE tenant_id=? AND store_id=? AND product_id=?`, a.TenantID, a.StoreID, v.ProductID).Scan(&rev)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return nil, e
		}
		v.Stale = errors.Is(e, sql.ErrNoRows) || rev != v.Revision || current != v.Observed
	}
	return out, tx.Commit()
}
func Operation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, id string) (map[string]any, error) {
	p := identity.ManageReplenishment
	if kind == "suggest" {
		p = identity.ManageStock
	}
	if kind != "policy" && kind != "suggest" && kind != "review" || id == "" || len(id) > 128 {
		return nil, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, p)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var input any
	result := map[string]any{"operation_id": id, "repeated": true}
	switch kind {
	case "policy":
		v := PolicyInput{OperationID: id}
		var rev int64
		e = tx.QueryRowContext(ctx, `SELECT product_id,minimum_milli,target_milli,revision FROM restock_policy_changes WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_identity_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id).Scan(&v.ProductID, &v.MinimumMilli, &v.TargetMilli, &rev)
		input = v
		result["revision"] = rev
	case "suggest":
		v := SuggestInput{OperationID: id}
		var sid, status string
		var observed, recommended, rev int64
		e = tx.QueryRowContext(ctx, `SELECT product_id,id,status,observed_milli,recommended_milli,policy_revision FROM restock_suggestions WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_identity_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id).Scan(&v.ProductID, &sid, &status, &observed, &recommended, &rev)
		input = v
		result["suggestion_id"] = sid
		result["needed"] = status != "not_needed"
		result["observed_milli"] = observed
		result["recommended_milli"] = recommended
		result["policy_revision"] = rev
	case "review":
		v := ReviewInput{OperationID: id}
		e = tx.QueryRowContext(ctx, `SELECT suggestion_id,decision,reason FROM restock_reviews WHERE tenant_id=? AND store_id=? AND device_id=? AND reviewer_identity_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id).Scan(&v.SuggestionID, &v.Decision, &v.Reason)
		input = v
		result["suggestion_id"] = v.SuggestionID
		result["decision"] = v.Decision
	}
	if errors.Is(e, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	return map[string]any{"kind": kind, "input": input, "result": result}, tx.Commit()
}
