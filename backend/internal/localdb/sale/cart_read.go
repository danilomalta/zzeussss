package sale

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"titansystem-backend/internal/localdb/identity"
)

func validStoredCart(v Cart) bool {
	if !cartKey(v.ID) || !cartKey(v.SaleID) || !cartKey(v.PaymentID) || v.Revision < 1 || v.Revision > 4096 || !cartText(v.Label, 160, false) || len(v.Items) < 1 || len(v.Items) > 500 || v.TotalCents < 1 || v.TotalCents > checkoutMaxSafe {
		return false
	}
	if v.State != "suspended" && v.State != "claimed" && v.State != "cancelled" && v.State != "completed" {
		return false
	}
	var total int64
	for _, i := range v.Items {
		if !cartKey(i.ProductID) || !cartKey(i.LocationID) || i.QuantityMilli < 1 || i.QuantityMilli > checkoutMaxSafe || i.UnitPriceCents < 0 || i.UnitPriceCents > checkoutMaxSafe || i.AvailableMilli < 0 || i.AvailableMilli > checkoutMaxSafe {
			return false
		}
		n, e := roundedCents(i.UnitPriceCents, i.QuantityMilli)
		if e != nil || n != i.TotalCents || n > checkoutMaxSafe-total {
			return false
		}
		total += n
	}
	if total != v.TotalCents {
		return false
	}
	if v.State == "completed" {
		return cartKey(v.SaleOperationID) && cartKey(v.CashSessionID) && v.ReceivedCents >= v.TotalCents && v.ReceivedCents <= checkoutMaxSafe && v.ChangeCents == v.ReceivedCents-v.TotalCents
	}
	return v.SaleOperationID == "" && v.ReceivedCents == 0 && v.ChangeCents == 0 && ((v.State == "claimed" && cartKey(v.CashSessionID)) || (v.State != "claimed" && v.CashSessionID == ""))
}
func cartVerifySaleTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, v Cart) error {
	in := Input{OperationID: v.SaleOperationID, SaleID: v.SaleID, CashSessionID: v.CashSessionID, Items: cartItems(v), Payments: []Payment{{Method: "cash", AmountCents: v.TotalCents}}}
	var n int
	e := tx.QueryRowContext(ctx, `SELECT count(*) FROM sales s JOIN sale_operations o ON o.tenant_id=s.tenant_id AND o.sale_id=s.id JOIN sale_payments p ON p.tenant_id=s.tenant_id AND p.sale_id=s.id WHERE s.tenant_id=? AND s.store_id=? AND s.device_id=? AND s.id=? AND s.status IN('committed','cancelled') AND s.cash_session_id=? AND s.total_cents=? AND o.store_id=s.store_id AND o.device_id=s.device_id AND o.actor_identity_id=? AND o.operation_id=? AND o.request_hash=? AND p.id=? AND p.method='cash' AND p.status IN('confirmed','reversed') AND p.amount_cents=?`, a.TenantID, a.StoreID, d.DeviceID, v.SaleID, v.CashSessionID, v.TotalCents, a.IdentityID, v.SaleOperationID, planHash(in), v.PaymentID, v.TotalCents).Scan(&n)
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func cartOperationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id, raw string) (CartResult, error) {
	var r CartResult
	if json.Unmarshal([]byte(raw), &r) != nil || r.Repeated || r.OperationID != id || !validStoredCart(r.Cart) {
		return r, ErrConflict
	}
	var n int
	e := tx.QueryRowContext(ctx, `SELECT count(*) FROM pos_cart_operations o JOIN pos_cart_events e ON e.tenant_id=o.tenant_id AND e.store_id=o.store_id AND e.device_id=o.device_id AND e.operation_id=o.operation_id WHERE o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.operation_id=? AND o.actor_id=? AND o.cart_id=? AND o.revision=? AND o.action=? AND o.result_json=? AND o.created_at=? AND e.payload_json=o.result_json AND e.event_type=? AND e.created_at=o.created_at`, a.TenantID, a.StoreID, d.DeviceID, id, a.IdentityID, r.Cart.ID, r.Cart.Revision, r.Action, raw, r.Cart.UpdatedAt, "pos.cart."+r.Action).Scan(&n)
	if e != nil {
		return r, e
	}
	if n != 1 {
		return r, ErrConflict
	}
	if r.Cart.State == "completed" {
		if e = cartVerifySaleTx(ctx, tx, a, d, r.Cart); e != nil {
			return r, e
		}
	}
	return r, nil
}
func cartGetTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Cart, error) {
	var v Cart
	var raw, state, saleID, updated string
	var revision int64
	e := tx.QueryRowContext(ctx, `SELECT state,revision,sale_id,payload_json,updated_at FROM pos_carts WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id).Scan(&state, &revision, &saleID, &raw, &updated)
	if errors.Is(e, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	if json.Unmarshal([]byte(raw), &v) != nil || !validStoredCart(v) || v.ID != id || v.State != state || v.Revision != revision || v.SaleID != saleID || v.UpdatedAt != updated {
		return v, ErrConflict
	}
	rows, e := tx.QueryContext(ctx, `SELECT operation_id,result_json FROM pos_cart_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND cart_id=? ORDER BY revision LIMIT 4097`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id)
	if e != nil {
		return v, e
	}
	type saved struct{ id, raw string }
	savedRows := []saved{}
	for rows.Next() {
		var s saved
		if e = rows.Scan(&s.id, &s.raw); e != nil {
			rows.Close()
			return v, e
		}
		savedRows = append(savedRows, s)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	if int64(len(savedRows)) != v.Revision {
		return v, ErrConflict
	}
	for i, s := range savedRows {
		r, e := cartOperationTx(ctx, tx, a, d, s.id, s.raw)
		if e != nil {
			return v, e
		}
		if r.Cart.Revision != int64(i+1) || r.Cart.SaleID != v.SaleID || r.Cart.PaymentID != v.PaymentID || r.Cart.CreatedAt != v.CreatedAt {
			return v, ErrConflict
		}
		if i == len(savedRows)-1 && !reflect.DeepEqual(r.Cart, v) {
			return v, ErrConflict
		}
	}
	return v, nil
}
func GetCart(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Cart, error) {
	if !cartKey(id) {
		return Cart{}, ErrInvalid
	}
	tx, e := cartBegin(ctx, db, nil, a, d, false)
	if e != nil {
		return Cart{}, e
	}
	defer tx.Rollback()
	return cartGetTx(ctx, tx, a, d, id)
}
func CartOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (CartResult, error) {
	if !cartKey(id) {
		return CartResult{}, ErrInvalid
	}
	tx, e := cartBegin(ctx, db, nil, a, d, false)
	if e != nil {
		return CartResult{}, e
	}
	defer tx.Rollback()
	var raw string
	e = tx.QueryRowContext(ctx, `SELECT result_json FROM pos_cart_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return CartResult{}, ErrNotFound
	}
	if e != nil {
		return CartResult{}, e
	}
	return cartOperationTx(ctx, tx, a, d, id, raw)
}
func ListCarts(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, state string, limit, offset int) (CartPage, error) {
	out := CartPage{Items: []Cart{}, Limit: limit, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000 || state != "" && state != "suspended" && state != "claimed" && state != "completed" && state != "cancelled" {
		return out, ErrInvalid
	}
	tx, e := cartBegin(ctx, db, nil, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	where := `tenant_id=? AND store_id=? AND device_id=? AND actor_id=?`
	args := []any{a.TenantID, a.StoreID, d.DeviceID, a.IdentityID}
	if state != "" {
		where += ` AND state=?`
		args = append(args, state)
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM pos_carts WHERE `+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM pos_carts WHERE `+where+` ORDER BY updated_at DESC,id LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		v, e := cartGetTx(ctx, tx, a, d, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	out.HasMore = int64(offset+len(out.Items)) < out.Total
	return out, nil
}
func CartEvents(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (CartHistory, error) {
	out := CartHistory{Items: []CartResult{}, Limit: limit, Offset: offset}
	if !cartKey(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := cartBegin(ctx, db, nil, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	v, e := cartGetTx(ctx, tx, a, d, id)
	if e != nil {
		return out, e
	}
	out.Total = v.Revision
	rows, e := tx.QueryContext(ctx, `SELECT result_json FROM pos_cart_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND cart_id=? ORDER BY revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id, limit, offset)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		var r CartResult
		if e = rows.Scan(&raw); e != nil {
			return out, e
		}
		if json.Unmarshal([]byte(raw), &r) != nil {
			return out, ErrConflict
		}
		out.Items = append(out.Items, r)
	}
	out.HasMore = int64(offset+len(out.Items)) < out.Total
	return out, rows.Err()
}
