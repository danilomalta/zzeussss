package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

// Original stage receipts are independent of later progress and license expiry.
func getStageOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, op string) (OperationReceipt, error) {
	if !validID(op) {
		return OperationReceipt{}, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return OperationReceipt{}, e
	}
	defer tx.Rollback()
	var body, result, order, savedKind, reason string
	e = tx.QueryRowContext(ctx, `SELECT request_json,result_json,order_id,kind,reason FROM production_stage_events WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&body, &result, &order, &savedKind, &reason)
	if errors.Is(e, sql.ErrNoRows) {
		return OperationReceipt{}, ErrNotFound
	}
	if e != nil {
		return OperationReceipt{}, e
	}
	if (kind == "stage_plan" && savedKind != "configured") || (kind == "stage_state" && savedKind != "running" && savedKind != "completed") {
		return OperationReceipt{}, ErrNotFound
	}
	var out StageResult
	if json.Unmarshal([]byte(result), &out) != nil || !exactOperationBody(result, out) || out.OrderID != order || !validID(order) || out.Repeated || !validOperationReason(reason) {
		return OperationReceipt{}, ErrConflict
	}
	plan, e := stagePlanTx(ctx, tx, a, order)
	if e != nil {
		return OperationReceipt{}, e
	}
	if kind == "stage_plan" {
		var in StagePlanInput
		if json.Unmarshal([]byte(body), &in) != nil {
			return OperationReceipt{}, ErrConflict
		}
		normalized, err := normalizeStagePlan(in)
		if err != nil || !exactOperationBody(body, normalized) || in.OperationID != op || in.OrderID != order || in.Reason != reason || out.StageID != "" || out.Revision != 1 || out.Status != "configured" || len(in.Stages) != len(plan.Items) {
			return OperationReceipt{}, ErrConflict
		}
		for i, s := range in.Stages {
			if s != plan.Items[i].StageDefinition {
				return OperationReceipt{}, ErrConflict
			}
		}
	} else {
		var in StageStateInput
		expected := int64(1)
		if savedKind == "completed" {
			expected = 2
		}
		if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || in.OperationID != op || in.OrderID != order || in.Reason != reason || !validID(in.StageID) || out.StageID != in.StageID || in.ExpectedRevision != expected || out.Revision != expected+1 || in.Status != savedKind || out.Status != savedKind {
			return OperationReceipt{}, ErrConflict
		}
		found := false
		for _, s := range plan.Items {
			if s.StageID == in.StageID {
				found = true
				if s.ResponsibleID != a.IdentityID || s.Revision < out.Revision {
					return OperationReceipt{}, ErrConflict
				}
			}
		}
		if !found {
			return OperationReceipt{}, ErrConflict
		}
	}
	return OperationReceipt{Kind: kind, Input: json.RawMessage(body), Result: json.RawMessage(result)}, tx.Commit()
}
