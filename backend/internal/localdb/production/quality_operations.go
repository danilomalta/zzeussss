package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

func getQualityOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, op string) (OperationReceipt, error) {
	if !validID(op) {
		return OperationReceipt{}, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return OperationReceipt{}, e
	}
	defer tx.Rollback()
	var body, result, lotID, status, criterion, reason, snapshot string
	var revision int64
	e = tx.QueryRowContext(ctx, `SELECT request_json,result_json,lot_id,revision,status,criterion,reason,lot_snapshot_json FROM production_quality_reviews WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&body, &result, &lotID, &revision, &status, &criterion, &reason, &snapshot)
	if errors.Is(e, sql.ErrNoRows) {
		return OperationReceipt{}, ErrNotFound
	}
	if e != nil {
		return OperationReceipt{}, e
	}
	var in QualityInput
	var out QualityResult
	var saved ProductionLot
	if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || json.Unmarshal([]byte(result), &out) != nil || !exactOperationBody(result, out) || json.Unmarshal([]byte(snapshot), &saved) != nil || !exactOperationBody(snapshot, saved) {
		return OperationReceipt{}, ErrConflict
	}
	normalized, e := normalizeQuality(in)
	if e != nil || normalized != in || in.OperationID != op || in.LotID != lotID || in.ExpectedRevision+1 != revision || in.Status != status || in.Criterion != criterion || in.Reason != reason || out.LotID != lotID || out.Revision != revision || out.Status != status || out.Repeated || saved.ID != lotID || saved.Status != "recorded" || saved.Revision != 1 || !validLotMetadata(saved.Code, saved.ManufacturedOn, saved.ExpiresOn) {
		return OperationReceipt{}, ErrConflict
	}
	current, e := scanLot(tx.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, lotID))
	if e != nil {
		return OperationReceipt{}, e
	}
	current.Status = saved.Status
	current.Revision = saved.Revision
	current.UpdatedAt = saved.UpdatedAt
	if current != saved {
		return OperationReceipt{}, ErrConflict
	}
	max, e := qualityRevisionTx(ctx, tx, a, lotID)
	if e != nil {
		return OperationReceipt{}, e
	}
	if revision < 1 || revision > max {
		return OperationReceipt{}, ErrConflict
	}
	produced, e := traceResultTx(ctx, tx, a, saved.ResultID)
	if e != nil {
		return OperationReceipt{}, e
	}
	if saved.ProductID != produced.ProductID || saved.Unit != produced.Unit || saved.QuantityMilli < 1 || saved.QuantityMilli > produced.ProducedMilli || (saved.Unit == "unit" && saved.QuantityMilli%1000 != 0) {
		return OperationReceipt{}, ErrConflict
	}
	return OperationReceipt{Kind: "quality", Input: json.RawMessage(body), Result: json.RawMessage(result)}, tx.Commit()
}
