package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

// A declaration receipt stays recorded after an audited void. Neither read writes stock.
func getLossOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, op string) (OperationReceipt, error) {
	if !validID(op) {
		return OperationReceipt{}, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return OperationReceipt{}, e
	}
	defer tx.Rollback()
	var body, result, lossID, savedKind, reason string
	var revision int64
	e = tx.QueryRowContext(ctx, `SELECT request_json,result_json,loss_id,kind,revision,reason FROM production_loss_events WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&body, &result, &lossID, &savedKind, &revision, &reason)
	if errors.Is(e, sql.ErrNoRows) {
		return OperationReceipt{}, ErrNotFound
	}
	if e != nil {
		return OperationReceipt{}, e
	}
	want := "recorded"
	if kind == "loss_void" {
		want = "voided"
	}
	if savedKind != want {
		return OperationReceipt{}, ErrNotFound
	}
	var out LossResult
	if json.Unmarshal([]byte(result), &out) != nil || !exactOperationBody(result, out) || out.LossID != lossID || !validID(lossID) || !validID(out.ResultID) || out.Status != want || out.Revision != revision || out.Repeated || !validOperationReason(reason) {
		return OperationReceipt{}, ErrConflict
	}
	loss, e := scanLoss(tx.QueryRowContext(ctx, `SELECT `+lossColumns+` FROM production_losses WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, lossID))
	if e != nil {
		return OperationReceipt{}, e
	}
	if loss.ResultID != out.ResultID || loss.Revision < revision {
		return OperationReceipt{}, ErrConflict
	}
	if kind == "loss" {
		var in LossInput
		if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || in.OperationID != op || in.LossID != lossID || in.ResultID != out.ResultID || in.QuantityMilli < 1 || in.QuantityMilli > MaxQuantity || in.QuantityMilli != loss.QuantityMilli || in.Reason != reason || loss.Reason != reason || revision != 1 {
			return OperationReceipt{}, ErrConflict
		}
	} else {
		var in VoidLossInput
		if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || in.OperationID != op || in.LossID != lossID || in.ExpectedRevision != 1 || revision != 2 || in.Reason != reason || loss.Status != "voided" {
			return OperationReceipt{}, ErrConflict
		}
	}
	saved, e := traceResultTx(ctx, tx, a, out.ResultID)
	if e != nil {
		return OperationReceipt{}, e
	}
	if loss.ProductID != saved.ProductID || loss.Unit != saved.Unit || loss.QuantityMilli > saved.ShortfallMilli || (loss.Unit == "unit" && loss.QuantityMilli%1000 != 0) {
		return OperationReceipt{}, ErrConflict
	}
	return OperationReceipt{Kind: kind, Input: json.RawMessage(body), Result: json.RawMessage(result)}, tx.Commit()
}
