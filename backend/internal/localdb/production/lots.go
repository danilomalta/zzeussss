package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"
	"unicode"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var ErrLot = errors.New("lote conflitante ou superior a quantidade produzida disponivel")

type LotInput struct {
	OperationID    string `json:"operation_id"`
	LotID          string `json:"lot_id"`
	ResultID       string `json:"result_id"`
	QuantityMilli  int64  `json:"quantity_milli"`
	Code           string `json:"code"`
	ManufacturedOn string `json:"manufactured_on"`
	ExpiresOn      string `json:"expires_on"`
	Reason         string `json:"reason"`
}

type VoidLotInput struct {
	OperationID      string `json:"operation_id"`
	LotID            string `json:"lot_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

type LotResult struct {
	LotID    string `json:"lot_id"`
	ResultID string `json:"result_id"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
	Repeated bool   `json:"repeated"`
}

type ProductionLot struct {
	ID             string `json:"id"`
	ResultID       string `json:"result_id"`
	ProductID      string `json:"product_id"`
	Unit           string `json:"unit"`
	QuantityMilli  int64  `json:"quantity_milli"`
	Code           string `json:"code"`
	ManufacturedOn string `json:"manufactured_on"`
	ExpiresOn      string `json:"expires_on"`
	Reason         string `json:"reason"`
	Status         string `json:"status"`
	Revision       int64  `json:"revision"`
	CreatedBy      string `json:"created_by"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type LotEvent struct {
	OperationID string `json:"operation_id"`
	DeviceID    string `json:"device_id"`
	ActorID     string `json:"actor_id"`
	Kind        string `json:"kind"`
	Revision    int64  `json:"revision"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}

type ResultLots struct {
	ResultID        string          `json:"result_id"`
	ProductID       string          `json:"product_id"`
	Unit            string          `json:"unit"`
	ProducedMilli   int64           `json:"produced_milli"`
	AssignedMilli   int64           `json:"assigned_milli"`
	UnassignedMilli int64           `json:"unassigned_milli"`
	Items           []ProductionLot `json:"items"`
}

const lotColumns = `id,result_id,product_id,unit,quantity_milli,code,manufactured_on,expires_on,reason,status,revision,created_by,created_at,updated_at`

func scanLot(row interface{ Scan(...any) error }) (ProductionLot, error) {
	var v ProductionLot
	err := row.Scan(&v.ID, &v.ResultID, &v.ProductID, &v.Unit, &v.QuantityMilli, &v.Code, &v.ManufacturedOn, &v.ExpiresOn, &v.Reason, &v.Status, &v.Revision, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProductionLot{}, ErrNotFound
	}
	return v, err
}

func lotReplay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, body string) (LotResult, bool, error) {
	var store, actor, savedKind, request, result string
	err := tx.QueryRowContext(ctx, `SELECT store_id,actor_id,kind,request_json,result_json FROM production_lot_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, op).Scan(&store, &actor, &savedKind, &request, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return LotResult{}, false, nil
	}
	if err != nil {
		return LotResult{}, false, err
	}
	if store != a.StoreID || actor != a.IdentityID || kind != savedKind || body != request {
		return LotResult{}, false, ErrConflict
	}
	var v LotResult
	if err = json.Unmarshal([]byte(result), &v); err != nil {
		return LotResult{}, false, err
	}
	v.Repeated = true
	return v, true, nil
}

// Sum only recorded lot allocations. Arbitrary precision avoids SQL SUM
// overflow. Voided history remains, but does not consume the produced allowance.
func lotSummaryTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (ResultLots, error) {
	out := ResultLots{ResultID: id, Items: []ProductionLot{}}
	err := tx.QueryRowContext(ctx, `SELECT product_id,unit,produced_milli FROM production_results WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.ProductID, &out.Unit, &out.ProducedMilli)
	if errors.Is(err, sql.ErrNoRows) {
		return ResultLots{}, ErrNotFound
	}
	if err != nil {
		return ResultLots{}, err
	}
	if !validUnit(out.Unit) || out.ProducedMilli < 0 || out.ProducedMilli > MaxQuantity {
		return ResultLots{}, ErrLot
	}
	rows, err := tx.QueryContext(ctx, `SELECT product_id,unit,quantity_milli FROM production_lots WHERE tenant_id=? AND store_id=? AND result_id=? AND status='recorded'`, a.TenantID, a.StoreID, id)
	if err != nil {
		return ResultLots{}, err
	}
	defer rows.Close()
	total := new(big.Int)
	for rows.Next() {
		var product, unit string
		var quantity int64
		if err = rows.Scan(&product, &unit, &quantity); err != nil {
			return ResultLots{}, err
		}
		if product != out.ProductID || unit != out.Unit || quantity < 1 || quantity > MaxQuantity {
			return ResultLots{}, ErrLot
		}
		total.Add(total, big.NewInt(quantity))
	}
	if err = rows.Err(); err != nil {
		return ResultLots{}, err
	}
	if !total.IsInt64() || total.Int64() > out.ProducedMilli {
		return ResultLots{}, ErrLot
	}
	out.AssignedMilli = total.Int64()
	out.UnassignedMilli = out.ProducedMilli - out.AssignedMilli
	return out, nil
}

func recordLotEvent(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, reason, body, now string, result LotResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = one(ctx, tx, `INSERT INTO production_lot_events VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID, result.LotID, kind, result.Revision, reason, body, string(encoded), now); err != nil {
		return err
	}
	lot, err := scanLot(tx.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, result.LotID))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		ActorID string          `json:"actor_id"`
		Kind    string          `json:"kind"`
		Request json.RawMessage `json:"request"`
		Lot     ProductionLot   `json:"lot"`
	}{a.IdentityID, kind, json.RawMessage(body), lot})
	if err != nil {
		return err
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	return one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, op, result.LotID, "production.lot.changed", 1, string(payload), now)
}

// Dates are explicit calendar declarations, never derived from retention rules.
// An empty expiry is unknown, not an unlimited shelf-life guarantee.
func validLotMetadata(code, made, expiry string) bool {
	if len(code) < 1 || len(code) > 64 || strings.TrimSpace(code) != code {
		return false
	}
	for _, r := range code {
		if unicode.IsControl(r) {
			return false
		}
	}
	date, err := time.Parse("2006-01-02", made)
	if err != nil || date.Format("2006-01-02") != made || date.Year() < 1 {
		return false
	}
	if expiry == "" {
		return true
	}
	end, err := time.Parse("2006-01-02", expiry)
	return err == nil && end.Format("2006-01-02") == expiry && !end.Before(date)
}

func RecordProductionLot(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in LotInput) (LotResult, error) {
	if db == nil {
		return LotResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	in.Code = strings.TrimSpace(in.Code)
	if !validID(in.OperationID) || !validID(in.LotID) || !validID(in.ResultID) || in.QuantityMilli < 1 || in.QuantityMilli > MaxQuantity || len(in.Reason) < 1 || len(in.Reason) > 255 || !validLotMetadata(in.Code, in.ManufacturedOn, in.ExpiresOn) {
		return LotResult{}, ErrInvalid
	}
	body, err := json.Marshal(in)
	if err != nil {
		return LotResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return LotResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return LotResult{}, err
	}
	result, repeated, err := lotReplay(ctx, tx, a, d, in.OperationID, "recorded", string(body))
	if err != nil {
		return LotResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_lots WHERE tenant_id=? AND store_id=? AND (id=? OR code=?)`, a.TenantID, a.StoreID, in.LotID, in.Code).Scan(&count); err != nil {
		return LotResult{}, err
	}
	if count != 0 {
		return LotResult{}, ErrConflict
	}
	summary, err := lotSummaryTx(ctx, tx, a, in.ResultID)
	if err != nil {
		return LotResult{}, err
	}
	if summary.Unit == "unit" && in.QuantityMilli%1000 != 0 {
		return LotResult{}, ErrInvalid
	}
	if in.QuantityMilli > summary.UnassignedMilli {
		return LotResult{}, ErrLot
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO production_lots VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.LotID, in.ResultID, summary.ProductID, summary.Unit, in.QuantityMilli, in.Code, in.ManufacturedOn, in.ExpiresOn, in.Reason, "recorded", 1, a.IdentityID, now, now); err != nil {
		return LotResult{}, err
	}
	result = LotResult{LotID: in.LotID, ResultID: in.ResultID, Status: "recorded", Revision: 1}
	if err = recordLotEvent(ctx, tx, a, d, in.OperationID, "recorded", in.Reason, string(body), now, result); err != nil {
		return LotResult{}, err
	}
	return result, tx.Commit()
}

func VoidProductionLot(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in VoidLotInput) (LotResult, error) {
	if db == nil {
		return LotResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.LotID) || in.ExpectedRevision < 1 || in.ExpectedRevision > MaxRevision || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return LotResult{}, ErrInvalid
	}
	body, err := json.Marshal(in)
	if err != nil {
		return LotResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return LotResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return LotResult{}, err
	}
	result, repeated, err := lotReplay(ctx, tx, a, d, in.OperationID, "voided", string(body))
	if err != nil {
		return LotResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	lot, err := scanLot(tx.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LotID))
	if err != nil {
		return LotResult{}, err
	}
	if lot.Status != "recorded" || lot.Revision != 1 || lot.Revision != in.ExpectedRevision {
		return LotResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `UPDATE production_lots SET status='voided',revision=2,updated_at=? WHERE tenant_id=? AND store_id=? AND id=? AND status='recorded' AND revision=1`, now, a.TenantID, a.StoreID, in.LotID); err != nil {
		return LotResult{}, err
	}
	result = LotResult{LotID: lot.ID, ResultID: lot.ResultID, Status: "voided", Revision: 2}
	if err = recordLotEvent(ctx, tx, a, d, in.OperationID, "voided", in.Reason, string(body), now, result); err != nil {
		return LotResult{}, err
	}
	return result, tx.Commit()
}

func GetProductionLot(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (ProductionLot, error) {
	if !validID(id) {
		return ProductionLot{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return ProductionLot{}, err
	}
	defer tx.Rollback()
	out, err := scanLot(tx.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id))
	if err != nil {
		return ProductionLot{}, err
	}
	return out, tx.Commit()
}

func ListProductionLots(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int) (ResultLots, error) {
	if !validID(id) || offset < 0 {
		return ResultLots{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return ResultLots{}, err
	}
	defer tx.Rollback()
	out, err := lotSummaryTx(ctx, tx, a, id)
	if err != nil {
		return ResultLots{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND result_id=? ORDER BY created_at,id LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, id, offset)
	if err != nil {
		return ResultLots{}, err
	}
	defer rows.Close()
	for rows.Next() {
		item, e := scanLot(rows)
		if e != nil {
			return ResultLots{}, e
		}
		out.Items = append(out.Items, item)
	}
	if err = rows.Err(); err != nil {
		return ResultLots{}, err
	}
	if err = rows.Close(); err != nil {
		return ResultLots{}, err
	}
	return out, tx.Commit()
}

func ProductionLotHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) ([]LotEvent, error) {
	if !validID(id) {
		return nil, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = scanLot(tx.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id)); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id,device_id,actor_id,kind,revision,reason,created_at FROM production_lot_events WHERE tenant_id=? AND store_id=? AND lot_id=? ORDER BY revision`, a.TenantID, a.StoreID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LotEvent{}
	for rows.Next() {
		var v LotEvent
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
