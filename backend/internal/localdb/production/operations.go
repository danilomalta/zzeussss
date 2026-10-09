package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"titansystem-backend/internal/localdb/identity"
)

// A durable result lookup, scoped to the original operator and device. It is a
// read, even after license expiration, and never retries a mutation implicitly.
type OperationReceipt struct {
	Kind   string          `json:"kind"`
	Input  json.RawMessage `json:"input"`
	Result json.RawMessage `json:"result"`
}

func GetOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, operationID string) (OperationReceipt, error) {
	if kind == "lot" || kind == "lot_void" {
		return getLotOperation(ctx, db, a, d, kind, operationID)
	}
	if kind == "quality" {
		return getQualityOperation(ctx, db, a, d, operationID)
	}
	if kind == "loss" || kind == "loss_void" {
		return getLossOperation(ctx, db, a, d, kind, operationID)
	}
	if kind == "stage_plan" || kind == "stage_state" {
		return getStageOperation(ctx, db, a, d, kind, operationID)
	}
	if kind == "recipe_state" || kind == "reserve" || kind == "materials" || kind == "result" {
		return getExecutionOperation(ctx, db, a, d, kind, operationID)
	}
	if !validID(operationID) || (kind != "recipe" && kind != "order" && kind != "state") {
		return OperationReceipt{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return OperationReceipt{}, err
	}
	defer tx.Rollback()
	var body, result string
	if kind == "recipe" {
		var recipeID, versionID string
		var revision int64
		err = tx.QueryRowContext(ctx, `SELECT request_json,recipe_id,version_id,revision FROM production_recipe_versions WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, operationID).Scan(&body, &recipeID, &versionID, &revision)
		if err == nil {
			var input PublishInput
			if e := json.Unmarshal([]byte(body), &input); e != nil {
				return OperationReceipt{}, ErrConflict
			}
			canonical, saved, e := normalize(input)
			if e != nil || saved != body || canonical.OperationID != operationID || canonical.RecipeID != recipeID || canonical.VersionID != versionID || canonical.ExpectedRevision+1 != revision {
				return OperationReceipt{}, ErrConflict
			}
			var audits int
			if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_recipe_audit WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=? AND recipe_id=? AND version_id=? AND revision=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, operationID, recipeID, versionID, revision).Scan(&audits); e != nil {
				return OperationReceipt{}, e
			}
			if audits != 1 {
				return OperationReceipt{}, ErrConflict
			}
			b, e := json.Marshal(Result{RecipeID: recipeID, VersionID: versionID, Revision: revision})
			if e != nil {
				return OperationReceipt{}, e
			}
			result = string(b)
		}
	} else {
		savedKind := "created"
		if kind == "state" {
			savedKind = "state"
		}
		var orderID string
		var revision int64
		var status string
		err = tx.QueryRowContext(ctx, `SELECT request_json,result_json,order_id,revision,after_status FROM production_order_events WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=? AND kind=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, operationID, savedKind).Scan(&body, &result, &orderID, &revision, &status)
		if err == nil {
			var out OrderResult
			if e := json.Unmarshal([]byte(result), &out); e != nil || out.OrderID != orderID || out.Revision != revision || out.Status != status || out.Repeated {
				return OperationReceipt{}, ErrConflict
			}
			if kind == "order" {
				var input OrderInput
				if e := json.Unmarshal([]byte(body), &input); e != nil || input.OperationID != operationID || input.OrderID != orderID || !validID(input.VersionID) || !validID(input.LocationID) || !validID(input.ResponsibleID) || input.PlannedBatches < 1 || input.PlannedBatches > MaxQuantity || revision != 1 || status != "planned" {
					return OperationReceipt{}, ErrConflict
				}
				b, e := json.Marshal(input)
				if e != nil || string(b) != body {
					return OperationReceipt{}, ErrConflict
				}
			} else {
				var input OrderStateInput
				if e := json.Unmarshal([]byte(body), &input); e != nil || input.OperationID != operationID || input.OrderID != orderID || input.ExpectedRevision < 1 || input.ExpectedRevision >= MaxRevision || input.ExpectedRevision+1 != revision || input.Status != status || (status != "approved" && status != "cancelled") || len(input.Reason) == 0 || len(input.Reason) > 255 {
					return OperationReceipt{}, ErrConflict
				}
				b, e := json.Marshal(input)
				if e != nil || string(b) != body {
					return OperationReceipt{}, ErrConflict
				}
			}
		}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return OperationReceipt{}, ErrNotFound
	}
	if err != nil {
		return OperationReceipt{}, err
	}
	out := OperationReceipt{Kind: kind, Input: json.RawMessage(body), Result: json.RawMessage(result)}
	return out, tx.Commit()
}
