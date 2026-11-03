package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stock"
)

const MaxReturnEvents = 10000

func validReturn(v SupplierReturn) bool {
	return returnKey(v.ID) && v.Revision >= 1 && v.Revision <= 4096 && !v.ExternalVerified && v.RequestedMilli >= 1 && v.RequestedMilli <= MaxQuantity && v.AuthorizedMilli >= 0 && v.AuthorizedMilli <= v.RequestedMilli && v.DispatchedMilli >= 0 && v.DispatchedMilli <= v.AuthorizedMilli && v.SupplierReceivedMilli >= 0 && v.SupplierReceivedMilli <= v.DispatchedMilli && v.RecoveredMilli >= 0 && v.RecoveredMilli <= v.DispatchedMilli-v.SupplierReceivedMilli
}
func returnOperationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, device, id, raw string) (ReturnResult, error) {
	var r ReturnResult
	if json.Unmarshal([]byte(raw), &r) != nil || r.OperationID != id || r.Repeated || !validReturn(r.Return) {
		return r, ErrConflict
	}
	var actor, requestBody string
	if e := tx.QueryRowContext(ctx, `SELECT actor_id,request_json FROM purchase_supplier_return_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, device, id).Scan(&actor, &requestBody); e != nil {
		return r, e
	}
	var request struct {
		ID, Action string
		Input      json.RawMessage
	}
	if json.Unmarshal([]byte(requestBody), &request) != nil || request.Action != r.Action {
		return r, ErrConflict
	}
	expectedID := r.Return.ID
	if r.Action == "open" {
		expectedID = r.Return.CaseID
	}
	if request.ID != expectedID {
		return r, ErrConflict
	}
	if r.ActionEvidence != nil {
		expected, _ := json.Marshal(r.ActionEvidence)
		if string(expected) != string(request.Input) {
			return r, ErrConflict
		}
	}
	if r.StockEvidence != nil {
		expected, _ := json.Marshal(r.StockEvidence)
		if string(expected) != string(request.Input) {
			return r, ErrConflict
		}
	}
	var n int
	e := tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_supplier_return_operations o JOIN purchase_supplier_return_events e ON e.tenant_id=o.tenant_id AND e.store_id=o.store_id AND e.device_id=o.device_id AND e.operation_id=o.operation_id WHERE o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.operation_id=? AND o.return_id=? AND o.revision=? AND o.action=? AND o.result_json=? AND e.payload_json=o.result_json AND e.created_at=o.created_at AND o.created_at=?`, a.TenantID, a.StoreID, device, id, r.Return.ID, r.Return.Revision, r.Action, raw, r.Return.UpdatedAt).Scan(&n)
	if e != nil {
		return r, e
	}
	if n != 1 {
		return r, ErrConflict
	}
	if r.StockOperationID != "" {
		if r.StockEvidence == nil {
			return r, ErrConflict
		}
		v := r.StockEvidence
		kind := "loss"
		sign := int64(-1)
		from, to := v.LocationID, ""
		if r.Action == "recover" {
			kind = "entry"
			sign = 1
			from, to = "", v.LocationID
		} else if r.Action != "dispatch" {
			return r, ErrConflict
		}
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_operations o JOIN stock_operation_movements l ON l.operation_id=o.id JOIN stock_movements m ON m.id=l.movement_id WHERE o.id=? AND o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.actor_identity_id=? AND o.kind=? AND o.product_id=? AND COALESCE(o.from_location_id,'')=? AND COALESCE(o.to_location_id,'')=? AND o.quantity_milli=? AND o.reason=? AND o.occurred_at=? AND m.tenant_id=o.tenant_id AND m.store_id=o.store_id AND m.device_id=o.device_id AND m.product_id=o.product_id AND m.quantity_milli=? AND m.location_id=? AND m.reason=o.reason AND m.created_at=o.occurred_at`, r.StockOperationID, a.TenantID, a.StoreID, device, actor, kind, r.Return.ProductID, from, to, v.QuantityMilli, v.Reason, r.Return.UpdatedAt, sign*v.QuantityMilli, v.LocationID).Scan(&n)
		if e != nil {
			return r, e
		}
		if n != 1 {
			return r, ErrConflict
		}
		input := stock.Input{OperationID: r.StockOperationID, Kind: kind, ProductID: r.Return.ProductID, FromLocationID: from, ToLocationID: to, QuantityMilli: v.QuantityMilli, Reason: v.Reason}
		payload, _ := json.Marshal(struct {
			TenantID string `json:"tenant_id"`
			StoreID  string `json:"store_id"`
			DeviceID string `json:"device_id"`
			ActorID  string `json:"actor_id"`
			stock.Input
		}{a.TenantID, a.StoreID, device, actor, input})
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND event_type='stock.operation' AND payload_json=?`, a.TenantID, a.StoreID, device, r.StockOperationID, string(payload)).Scan(&n)
		if e != nil {
			return r, e
		}
		if n != 1 {
			return r, ErrConflict
		}
	}
	return r, nil
}
func returnIdentity(v SupplierReturn) string {
	b, _ := json.Marshal(struct {
		ID, Case, Receipt, Order, Supplier, Product, Unit, Creator, At string
		Requested                                                      int64
	}{v.ID, v.CaseID, v.ReceiptID, v.OrderID, v.SupplierID, v.ProductID, v.Unit, v.CreatedBy, v.CreatedAt, v.RequestedMilli})
	return string(b)
}
func returnGetTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (SupplierReturn, error) {
	var v SupplierReturn
	var raw, caseID, receipt, order, supplier, product, unit, state string
	var requested, authorized, dispatched, received, recovered, revision int64
	e := tx.QueryRowContext(ctx, `SELECT case_id,receipt_id,order_id,supplier_id,product_id,unit,requested_milli,authorized_milli,dispatched_milli,supplier_received_milli,recovered_milli,state,revision,payload_json FROM purchase_supplier_returns WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&caseID, &receipt, &order, &supplier, &product, &unit, &requested, &authorized, &dispatched, &received, &recovered, &state, &revision, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	if json.Unmarshal([]byte(raw), &v) != nil || !validReturn(v) || v.ID != id || v.CaseID != caseID || v.ReceiptID != receipt || v.OrderID != order || v.SupplierID != supplier || v.ProductID != product || v.Unit != unit || v.RequestedMilli != requested || v.AuthorizedMilli != authorized || v.DispatchedMilli != dispatched || v.SupplierReceivedMilli != received || v.RecoveredMilli != recovered || v.State != state || v.Revision != revision {
		return v, ErrConflict
	}
	rows, e := tx.QueryContext(ctx, `SELECT device_id,operation_id,result_json FROM purchase_supplier_return_operations WHERE tenant_id=? AND store_id=? AND return_id=? ORDER BY revision`, a.TenantID, a.StoreID, id)
	if e != nil {
		return v, e
	}
	type saved struct{ device, id, body string }
	history := []saved{}
	for rows.Next() {
		var s saved
		if e = rows.Scan(&s.device, &s.id, &s.body); e != nil {
			rows.Close()
			return v, e
		}
		history = append(history, s)
		if len(history) > 4096 {
			rows.Close()
			return v, ErrReturnLimit
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	if len(history) != int(v.Revision) {
		return v, ErrConflict
	}
	for i, s := range history {
		r, e := returnOperationTx(ctx, tx, a, s.device, s.id, s.body)
		if e != nil {
			return v, e
		}
		if r.Return.Revision != int64(i+1) || returnIdentity(r.Return) != returnIdentity(v) {
			return v, ErrConflict
		}
		if i == len(history)-1 && !reflect.DeepEqual(r.Return, v) {
			return v, ErrConflict
		}
	}
	return v, nil
}
func returnSourceSetTx(ctx context.Context, tx *sql.Tx, a identity.Scope, where string, args ...any) ([]SupplierReturn, error) {
	params := append([]any{a.TenantID, a.StoreID}, args...)
	var events int
	if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_supplier_return_operations o JOIN purchase_supplier_returns r ON r.tenant_id=o.tenant_id AND r.store_id=o.store_id AND r.id=o.return_id WHERE r.tenant_id=? AND r.store_id=?`+where, params...).Scan(&events); e != nil {
		return nil, e
	}
	if events > MaxReturnEvents {
		return nil, ErrReturnLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT r.id FROM purchase_supplier_returns r WHERE r.tenant_id=? AND r.store_id=?`+where+` ORDER BY r.id LIMIT 1001`, params...)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(ids) > MaxSupplierReturns {
		return nil, ErrReturnLimit
	}
	out := []SupplierReturn{}
	for _, id := range ids {
		v, e := returnGetTx(ctx, tx, a, id)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}

// ReturnsPermitVoidTx prevents a second physical debit of stock still outside
// the store. A fully recovered shipment does not permanently block correction.
func ReturnsPermitVoidTx(ctx context.Context, tx *sql.Tx, a identity.Scope, receipt string) error {
	rows, e := returnSourceSetTx(ctx, tx, a, ` AND r.receipt_id=?`, receipt)
	if e != nil {
		return e
	}
	for _, v := range rows {
		if v.DispatchedMilli > v.RecoveredMilli {
			return ErrConflict
		}
	}
	return nil
}

// SupplierReturnsForAccountingTx exposes the source inside an authorized
// financial snapshot; callers must project only the data their report needs.
func SupplierReturnsForAccountingTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, receipt string) ([]SupplierReturn, error) {
	if !returnKey(receipt) {
		return nil, ErrInvalid
	}
	if e := identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); e != nil {
		return nil, e
	}
	return returnSourceSetTx(ctx, tx, a, ` AND r.receipt_id=?`, receipt)
}
func returnReadTx(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (*sql.Tx, error) {
	if db == nil {
		return nil, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if e = returnPermissions(ctx, tx, a, d); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func GetSupplierReturn(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (SupplierReturn, error) {
	if !returnKey(id) {
		return SupplierReturn{}, ErrInvalid
	}
	tx, e := returnReadTx(ctx, db, a, d)
	if e != nil {
		return SupplierReturn{}, e
	}
	defer tx.Rollback()
	v, e := returnGetTx(ctx, tx, a, id)
	if e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func ListSupplierReturns(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, caseID, orderID, supplier, state string, limit, offset int) (ReturnPage, error) {
	out := ReturnPage{Items: []SupplierReturn{}, Limit: limit, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	where := ""
	args := []any{}
	for _, f := range []struct{ col, value string }{{"case_id", caseID}, {"order_id", orderID}, {"supplier_id", supplier}, {"state", state}} {
		if f.value != "" {
			if !returnKey(f.value) {
				return out, ErrInvalid
			}
			where += " AND r." + f.col + "=?"
			args = append(args, f.value)
		}
	}
	if state != "" && state != "requested" && state != "approved" && state != "scheduled" && state != "dispatched" && state != "completed" && state != "declined" && state != "cancelled" {
		return out, ErrInvalid
	}
	tx, e := returnReadTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	all, e := returnSourceSetTx(ctx, tx, a, where, args...)
	if e != nil {
		return out, e
	}
	out.Total = len(all)
	out.HasMore = offset+limit < len(all)
	for i, v := range all {
		if i >= offset && i-offset < limit {
			out.Items = append(out.Items, v)
		}
	}
	return out, tx.Commit()
}
func SupplierReturnHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (ReturnHistory, error) {
	out := ReturnHistory{Items: []ReturnResult{}, Limit: limit, Offset: offset}
	if !returnKey(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := returnReadTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	v, e := returnGetTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out.Total = int(v.Revision)
	out.HasMore = offset+limit < out.Total
	rows, e := tx.QueryContext(ctx, `SELECT device_id,operation_id,result_json FROM purchase_supplier_return_operations WHERE tenant_id=? AND store_id=? AND return_id=? ORDER BY revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, id, limit, offset)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var device, op, raw string
		if e = rows.Scan(&device, &op, &raw); e != nil {
			return out, e
		}
		var r ReturnResult
		if json.Unmarshal([]byte(raw), &r) != nil {
			return out, ErrConflict
		}
		out.Items = append(out.Items, r)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	rows.Close()
	return out, tx.Commit()
}
func SupplierReturnOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (ReturnResult, error) {
	if !returnKey(id) {
		return ReturnResult{}, ErrInvalid
	}
	tx, e := returnReadTx(ctx, db, a, d)
	if e != nil {
		return ReturnResult{}, e
	}
	defer tx.Rollback()
	var who, body string
	e = tx.QueryRowContext(ctx, `SELECT actor_id,result_json FROM purchase_supplier_return_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &body)
	if errors.Is(e, sql.ErrNoRows) || e == nil && who != a.IdentityID {
		return ReturnResult{}, ErrNotFound
	}
	if e != nil {
		return ReturnResult{}, e
	}
	r, e := returnOperationTx(ctx, tx, a, d.DeviceID, id, body)
	if e != nil {
		return r, e
	}
	return r, tx.Commit()
}
