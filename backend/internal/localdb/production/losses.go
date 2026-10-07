package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var ErrLoss = errors.New("perda conflitante ou superior a diferenca de rendimento disponivel")

type LossInput struct {
	OperationID   string `json:"operation_id"`
	LossID        string `json:"loss_id"`
	ResultID      string `json:"result_id"`
	QuantityMilli int64  `json:"quantity_milli"`
	Reason        string `json:"reason"`
}

type VoidLossInput struct {
	OperationID      string `json:"operation_id"`
	LossID           string `json:"loss_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type LossResult struct {
	LossID   string `json:"loss_id"`
	ResultID string `json:"result_id"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
	Repeated bool   `json:"repeated"`
}

type ProductionLoss struct {
	ID            string `json:"id"`
	ResultID      string `json:"result_id"`
	ProductID     string `json:"product_id"`
	Unit          string `json:"unit"`
	QuantityMilli int64  `json:"quantity_milli"`
	Reason        string `json:"reason"`
	Status        string `json:"status"`
	Revision      int64  `json:"revision"`
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type LossEvent struct {
	OperationID string `json:"operation_id"`
	DeviceID    string `json:"device_id"`
	ActorID     string `json:"actor_id"`
	Kind        string `json:"kind"`
	Revision    int64  `json:"revision"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}

type ResultLosses struct {
	ResultID                   string           `json:"result_id"`
	ProductID                  string           `json:"product_id"`
	Unit                       string           `json:"unit"`
	PlannedMilli               int64            `json:"planned_milli"`
	ProducedMilli              int64            `json:"produced_milli"`
	ShortfallMilli             int64            `json:"shortfall_milli"`
	RecordedLossMilli          int64            `json:"recorded_loss_milli"`
	UnclassifiedShortfallMilli int64            `json:"unclassified_shortfall_milli"`
	Items                      []ProductionLoss `json:"items"`
}

const lossColumns = `id,result_id,product_id,unit,quantity_milli,reason,status,revision,created_by,created_at,updated_at`

func scanLoss(row interface{ Scan(...any) error }) (ProductionLoss, error) {
	var v ProductionLoss
	err := row.Scan(&v.ID, &v.ResultID, &v.ProductID, &v.Unit, &v.QuantityMilli, &v.Reason, &v.Status, &v.Revision, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProductionLoss{}, ErrNotFound
	}
	return v, err
}

func lossReplay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, body string) (LossResult, bool, error) {
	var store, actor, savedKind, request, result string
	err := tx.QueryRowContext(ctx, `SELECT store_id,actor_id,kind,request_json,result_json FROM production_loss_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, op).Scan(&store, &actor, &savedKind, &request, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return LossResult{}, false, nil
	}
	if err != nil {
		return LossResult{}, false, err
	}
	if store != a.StoreID || actor != a.IdentityID || kind != savedKind || body != request {
		return LossResult{}, false, ErrConflict
	}
	var v LossResult
	if err = json.Unmarshal([]byte(result), &v); err != nil {
		return LossResult{}, false, err
	}
	v.Repeated = true
	return v, true, nil
}

// Sum only recorded declarations. Arbitrary precision avoids SQL SUM overflow;
// voided history is retained but cannot consume the classification allowance.
func lossSummaryTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (ResultLosses, error) {
	out := ResultLosses{ResultID: id, Items: []ProductionLoss{}}
	err := tx.QueryRowContext(ctx, `SELECT product_id,unit,planned_milli,produced_milli,shortfall_milli FROM production_results WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.ProductID, &out.Unit, &out.PlannedMilli, &out.ProducedMilli, &out.ShortfallMilli)
	if errors.Is(err, sql.ErrNoRows) {
		return ResultLosses{}, ErrNotFound
	}
	if err != nil {
		return ResultLosses{}, err
	}
	if !validUnit(out.Unit) || out.PlannedMilli < 1 || out.PlannedMilli > MaxQuantity || out.ProducedMilli < 0 || out.ProducedMilli > out.PlannedMilli || out.ShortfallMilli != out.PlannedMilli-out.ProducedMilli {
		return ResultLosses{}, ErrLoss
	}
	rows, err := tx.QueryContext(ctx, `SELECT product_id,unit,quantity_milli FROM production_losses WHERE tenant_id=? AND store_id=? AND result_id=? AND status='recorded'`, a.TenantID, a.StoreID, id)
	if err != nil {
		return ResultLosses{}, err
	}
	defer rows.Close()
	total := new(big.Int)
	for rows.Next() {
		var product, unit string
		var quantity int64
		if err = rows.Scan(&product, &unit, &quantity); err != nil {
			return ResultLosses{}, err
		}
		if product != out.ProductID || unit != out.Unit || quantity < 1 || quantity > MaxQuantity {
			return ResultLosses{}, ErrLoss
		}
		total.Add(total, big.NewInt(quantity))
	}
	if err = rows.Err(); err != nil {
		return ResultLosses{}, err
	}
	if !total.IsInt64() || total.Int64() > out.ShortfallMilli {
		return ResultLosses{}, ErrLoss
	}
	out.RecordedLossMilli = total.Int64()
	out.UnclassifiedShortfallMilli = out.ShortfallMilli - out.RecordedLossMilli
	return out, nil
}

func recordLossEvent(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, reason, body, now string, result LossResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = one(ctx, tx, `INSERT INTO production_loss_events VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID, result.LossID, kind, result.Revision, reason, body, string(encoded), now); err != nil {
		return err
	}
	loss, err := scanLoss(tx.QueryRowContext(ctx, `SELECT `+lossColumns+` FROM production_losses WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, result.LossID))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		ActorID string          `json:"actor_id"`
		Kind    string          `json:"kind"`
		Request json.RawMessage `json:"request"`
		Loss    ProductionLoss  `json:"loss"`
	}{a.IdentityID, kind, json.RawMessage(body), loss})
	if err != nil {
		return err
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	return one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, op, result.LossID, "production.loss.changed", 1, string(payload), now)
}

func RecordProductionLoss(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in LossInput) (LossResult, error) {
	if db == nil {
		return LossResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.LossID) || !validID(in.ResultID) || in.QuantityMilli < 1 || in.QuantityMilli > MaxQuantity || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return LossResult{}, ErrInvalid
	}
	body, err := json.Marshal(in)
	if err != nil {
		return LossResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return LossResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return LossResult{}, err
	}
	result, repeated, err := lossReplay(ctx, tx, a, d, in.OperationID, "recorded", string(body))
	if err != nil {
		return LossResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_losses WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LossID).Scan(&count); err != nil {
		return LossResult{}, err
	}
	if count != 0 {
		return LossResult{}, ErrConflict
	}
	summary, err := lossSummaryTx(ctx, tx, a, in.ResultID)
	if err != nil {
		return LossResult{}, err
	}
	if summary.Unit == "unit" && in.QuantityMilli%1000 != 0 {
		return LossResult{}, ErrInvalid
	}
	if in.QuantityMilli > summary.UnclassifiedShortfallMilli {
		return LossResult{}, ErrLoss
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO production_losses VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.LossID, in.ResultID, summary.ProductID, summary.Unit, in.QuantityMilli, in.Reason, "recorded", 1, a.IdentityID, now, now); err != nil {
		return LossResult{}, err
	}
	result = LossResult{LossID: in.LossID, ResultID: in.ResultID, Status: "recorded", Revision: 1}
	if err = recordLossEvent(ctx, tx, a, d, in.OperationID, "recorded", in.Reason, string(body), now, result); err != nil {
		return LossResult{}, err
	}
	return result, tx.Commit()
}

func VoidProductionLoss(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in VoidLossInput) (LossResult, error) {
	if db == nil {
		return LossResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.LossID) || in.ExpectedRevision < 1 || in.ExpectedRevision > MaxRevision || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return LossResult{}, ErrInvalid
	}
	body, err := json.Marshal(in)
	if err != nil {
		return LossResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return LossResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return LossResult{}, err
	}
	result, repeated, err := lossReplay(ctx, tx, a, d, in.OperationID, "voided", string(body))
	if err != nil {
		return LossResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	loss, err := scanLoss(tx.QueryRowContext(ctx, `SELECT `+lossColumns+` FROM production_losses WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LossID))
	if err != nil {
		return LossResult{}, err
	}
	if loss.Status != "recorded" || loss.Revision != 1 || loss.Revision != in.ExpectedRevision {
		return LossResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `UPDATE production_losses SET status='voided',revision=2,updated_at=? WHERE tenant_id=? AND store_id=? AND id=? AND status='recorded' AND revision=1`, now, a.TenantID, a.StoreID, in.LossID); err != nil {
		return LossResult{}, err
	}
	result = LossResult{LossID: loss.ID, ResultID: loss.ResultID, Status: "voided", Revision: 2}
	if err = recordLossEvent(ctx, tx, a, d, in.OperationID, "voided", in.Reason, string(body), now, result); err != nil {
		return LossResult{}, err
	}
	return result, tx.Commit()
}

func GetProductionLoss(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (ProductionLoss, error) {
	if !validID(id) {
		return ProductionLoss{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return ProductionLoss{}, err
	}
	defer tx.Rollback()
	out, err := scanLoss(tx.QueryRowContext(ctx, `SELECT `+lossColumns+` FROM production_losses WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id))
	if err != nil {
		return ProductionLoss{}, err
	}
	return out, tx.Commit()
}

func ListProductionLosses(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int) (ResultLosses, error) {
	if !validID(id) || offset < 0 {
		return ResultLosses{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return ResultLosses{}, err
	}
	defer tx.Rollback()
	out, err := lossSummaryTx(ctx, tx, a, id)
	if err != nil {
		return ResultLosses{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+lossColumns+` FROM production_losses WHERE tenant_id=? AND store_id=? AND result_id=? ORDER BY created_at,id LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, id, offset)
	if err != nil {
		return ResultLosses{}, err
	}
	defer rows.Close()
	for rows.Next() {
		item, e := scanLoss(rows)
		if e != nil {
			return ResultLosses{}, e
		}
		out.Items = append(out.Items, item)
	}
	if err = rows.Err(); err != nil {
		return ResultLosses{}, err
	}
	if err = rows.Close(); err != nil {
		return ResultLosses{}, err
	}
	return out, tx.Commit()
}

func ProductionLossHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) ([]LossEvent, error) {
	if !validID(id) {
		return nil, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = scanLoss(tx.QueryRowContext(ctx, `SELECT `+lossColumns+` FROM production_losses WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id)); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id,device_id,actor_id,kind,revision,reason,created_at FROM production_loss_events WHERE tenant_id=? AND store_id=? AND loss_id=? ORDER BY revision`, a.TenantID, a.StoreID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LossEvent{}
	for rows.Next() {
		var v LossEvent
		if err = rows.Scan(&v.OperationID, &v.DeviceID, &v.ActorID, &v.Kind, &v.Revision, &v.Reason, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
