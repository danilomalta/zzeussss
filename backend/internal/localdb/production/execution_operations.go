package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"titansystem-backend/internal/localdb/identity"
)

func exactOperationBody(body string, in any) bool {
	b, e := json.Marshal(in)
	return e == nil && string(b) == body
}
func validOperationReason(s string) bool {
	return len(s) > 0 && len(s) <= 255 && strings.TrimSpace(s) == s
}

// Original, scoped execution receipts. Later transitions never replace their result.
func getExecutionOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, op string) (OperationReceipt, error) {
	if !validID(op) {
		return OperationReceipt{}, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return OperationReceipt{}, e
	}
	defer tx.Rollback()
	var body, result string
	switch kind {
	case "recipe_state":
		var recipe, status, reason string
		var revision int64
		e = tx.QueryRowContext(ctx, `SELECT request_json,recipe_id,after_status,revision,reason FROM production_recipe_state_events WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&body, &recipe, &status, &revision, &reason)
		if e == nil {
			var in RecipeStateInput
			if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || in.OperationID != op || in.RecipeID != recipe || !validID(recipe) || in.Status != status || (status != "active" && status != "inactive") || in.ExpectedRevision < 0 || in.ExpectedRevision >= MaxRevision || in.ExpectedRevision+1 != revision || in.Reason != reason || !validOperationReason(reason) {
				return OperationReceipt{}, ErrConflict
			}
			b, _ := json.Marshal(RecipeState{RecipeID: recipe, Status: status, Revision: revision})
			result = string(b)
		}
	case "reserve", "materials":
		var res, savedKind string
		e = tx.QueryRowContext(ctx, `SELECT request_json,result_json,reservation_id,kind FROM production_material_events WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&body, &result, &res, &savedKind)
		if e == nil {
			if (kind == "reserve" && savedKind != "reserve") || (kind == "materials" && savedKind != "release" && savedKind != "consume") {
				return OperationReceipt{}, ErrNotFound
			}
			var out MaterialResult
			if json.Unmarshal([]byte(result), &out) != nil || !exactOperationBody(result, out) || out.ReservationID != res || !validID(res) || !validID(out.OrderID) || out.Repeated {
				return OperationReceipt{}, ErrConflict
			}
			current, err := materialGetTx(ctx, tx, a, res)
			if err != nil {
				return OperationReceipt{}, err
			}
			if current.OrderID != out.OrderID {
				return OperationReceipt{}, ErrConflict
			}
			if kind == "reserve" {
				var in ReserveInput
				if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || in.OperationID != op || in.ReservationID != res || in.OrderID != out.OrderID || !validOperationReason(in.Reason) || out.Status != "active" {
					return OperationReceipt{}, ErrConflict
				}
			} else {
				var in MaterialChangeInput
				want := "released"
				if savedKind == "consume" {
					want = "consumed"
				}
				if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || in.OperationID != op || in.ReservationID != res || in.Action != savedKind || !validOperationReason(in.Reason) || out.Status != want {
					return OperationReceipt{}, ErrConflict
				}
			}
		}
	case "result":
		var resultID string
		e = tx.QueryRowContext(ctx, `SELECT request_json,result_json,id FROM production_results WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&body, &result, &resultID)
		if e == nil {
			var in ResultInput
			var out CompletionResult
			if json.Unmarshal([]byte(body), &in) != nil || json.Unmarshal([]byte(result), &out) != nil || !exactOperationBody(body, in) || !exactOperationBody(result, out) || in.OperationID != op || in.ResultID != resultID || !validID(resultID) || !validID(in.OrderID) || in.OrderID != out.OrderID || in.ExpectedRevision < 1 || in.ExpectedRevision >= MaxRevision || out.Revision != in.ExpectedRevision+1 || in.ProducedMilli < 0 || in.ProducedMilli > MaxQuantity || out.ProducedMilli != in.ProducedMilli || out.PlannedMilli < 1 || out.PlannedMilli > MaxQuantity || out.ProducedMilli > out.PlannedMilli || out.ShortfallMilli != out.PlannedMilli-out.ProducedMilli || out.Status != "completed" || out.Repeated || !validOperationReason(in.Reason) {
				return OperationReceipt{}, ErrConflict
			}
			saved, err := traceResultTx(ctx, tx, a, resultID)
			if err != nil {
				return OperationReceipt{}, err
			}
			if saved.CompletionResult != out || saved.OperationID != op || saved.Reason != in.Reason {
				return OperationReceipt{}, ErrConflict
			}
		}
	default:
		return OperationReceipt{}, ErrInvalid
	}
	if errors.Is(e, sql.ErrNoRows) {
		return OperationReceipt{}, ErrNotFound
	}
	if e != nil {
		return OperationReceipt{}, e
	}
	return OperationReceipt{Kind: kind, Input: json.RawMessage(body), Result: json.RawMessage(result)}, tx.Commit()
}
