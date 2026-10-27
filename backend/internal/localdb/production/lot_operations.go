package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

// A declaration receipt stays recorded after an audited void. Neither read writes stock.
func getLotOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, op string) (OperationReceipt, error) {
	if !validID(op) {
		return OperationReceipt{}, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return OperationReceipt{}, e
	}
	defer tx.Rollback()
	var body, result, lotID, savedKind, reason string
	var revision int64
	e = tx.QueryRowContext(ctx, `SELECT request_json,result_json,lot_id,kind,revision,reason FROM production_lot_events WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, op).Scan(&body, &result, &lotID, &savedKind, &revision, &reason)
	if errors.Is(e, sql.ErrNoRows) {
		return OperationReceipt{}, ErrNotFound
	}
	if e != nil {
		return OperationReceipt{}, e
	}
	want := "recorded"
	if kind == "lot_void" {
		want = "voided"
	}
	if savedKind != want {
		return OperationReceipt{}, ErrNotFound
	}
	var out LotResult
	if json.Unmarshal([]byte(result), &out) != nil || !exactOperationBody(result, out) || out.LotID != lotID || !validID(lotID) || !validID(out.ResultID) || out.Status != want || out.Revision != revision || out.Repeated || !validOperationReason(reason) {
		return OperationReceipt{}, ErrConflict
	}
	lot, e := scanLot(tx.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, lotID))
	if e != nil {
		return OperationReceipt{}, e
	}
	if lot.ResultID != out.ResultID || lot.Revision < revision {
		return OperationReceipt{}, ErrConflict
	}
	if kind == "lot" {
		var in LotInput
		if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || in.OperationID != op || in.LotID != lotID || in.ResultID != out.ResultID || in.QuantityMilli < 1 || in.QuantityMilli > MaxQuantity || in.QuantityMilli != lot.QuantityMilli || in.Code != lot.Code || in.ManufacturedOn != lot.ManufacturedOn || in.ExpiresOn != lot.ExpiresOn || !validLotMetadata(in.Code, in.ManufacturedOn, in.ExpiresOn) || in.Reason != reason || lot.Reason != reason || revision != 1 {
			return OperationReceipt{}, ErrConflict
		}
	} else {
		var in VoidLotInput
		if json.Unmarshal([]byte(body), &in) != nil || !exactOperationBody(body, in) || in.OperationID != op || in.LotID != lotID || in.ExpectedRevision != 1 || revision != 2 || in.Reason != reason || lot.Status != "voided" {
			return OperationReceipt{}, ErrConflict
		}
	}
	saved, e := traceResultTx(ctx, tx, a, out.ResultID)
	if e != nil {
		return OperationReceipt{}, e
	}
	if lot.ProductID != saved.ProductID || lot.Unit != saved.Unit || lot.QuantityMilli > saved.ProducedMilli || (lot.Unit == "unit" && lot.QuantityMilli%1000 != 0) {
		return OperationReceipt{}, ErrConflict
	}
	return OperationReceipt{Kind: kind, Input: json.RawMessage(body), Result: json.RawMessage(result)}, tx.Commit()
}
