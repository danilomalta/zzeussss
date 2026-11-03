package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var ErrQuality = errors.New("avaliacao de qualidade ou historico incompatível")

type QualityInput struct {
	OperationID      string `json:"operation_id"`
	LotID            string `json:"lot_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Status           string `json:"status"`
	Criterion        string `json:"criterion"`
	Reason           string `json:"reason"`
}
type QualityResult struct {
	LotID    string `json:"lot_id"`
	Revision int64  `json:"revision"`
	Status   string `json:"status"`
	Repeated bool   `json:"repeated"`
}
type QualityReview struct {
	LotID       string        `json:"lot_id"`
	Revision    int64         `json:"revision"`
	OperationID string        `json:"operation_id"`
	DeviceID    string        `json:"device_id"`
	ActorID     string        `json:"actor_id"`
	Status      string        `json:"status"`
	Criterion   string        `json:"criterion"`
	Reason      string        `json:"reason"`
	LotSnapshot ProductionLot `json:"lot_snapshot"`
	CreatedAt   string        `json:"created_at"`
}
type LotQuality struct {
	LotID     string         `json:"lot_id"`
	LotStatus string         `json:"lot_status"`
	Revision  int64          `json:"revision"`
	Status    string         `json:"status"`
	Latest    *QualityReview `json:"latest"`
}

func normalizeQuality(in QualityInput) (QualityInput, error) {
	in.Criterion = strings.TrimSpace(in.Criterion)
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.LotID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= MaxRevision || (in.Status != "passed" && in.Status != "failed") || len(in.Criterion) < 1 || len(in.Criterion) > 255 || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return QualityInput{}, ErrInvalid
	}
	return in, nil
}
func qualityReplay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, body string) (QualityResult, bool, error) {
	var store, actor, request, result string
	err := tx.QueryRowContext(ctx, `SELECT store_id,actor_id,request_json,result_json FROM production_quality_reviews WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, op).Scan(&store, &actor, &request, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return QualityResult{}, false, nil
	}
	if err != nil {
		return QualityResult{}, false, err
	}
	if store != a.StoreID || actor != a.IdentityID || request != body {
		return QualityResult{}, false, ErrConflict
	}
	var out QualityResult
	if err = json.Unmarshal([]byte(result), &out); err != nil {
		return QualityResult{}, false, err
	}
	out.Repeated = true
	return out, true, nil
}

const qualityColumns = `lot_id,revision,operation_id,device_id,actor_id,status,criterion,reason,lot_snapshot_json,created_at`

func scanQuality(row interface{ Scan(...any) error }) (QualityReview, error) {
	var out QualityReview
	var snapshot string
	err := row.Scan(&out.LotID, &out.Revision, &out.OperationID, &out.DeviceID, &out.ActorID, &out.Status, &out.Criterion, &out.Reason, &snapshot, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return QualityReview{}, ErrNotFound
	}
	if err != nil {
		return QualityReview{}, err
	}
	if err = json.Unmarshal([]byte(snapshot), &out.LotSnapshot); err != nil {
		return QualityReview{}, err
	}
	return out, nil
}
func qualityRevisionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (int64, error) {
	var count, revision int64
	err := tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(max(revision),0) FROM production_quality_reviews WHERE tenant_id=? AND store_id=? AND lot_id=?`, a.TenantID, a.StoreID, id).Scan(&count, &revision)
	if err != nil {
		return 0, err
	}
	if revision != count || revision < 0 || revision > MaxRevision {
		return 0, ErrQuality
	}
	return revision, nil
}
func RecordQualityReview(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in QualityInput) (QualityResult, error) {
	if db == nil {
		return QualityResult{}, errors.New("banco local indisponivel")
	}
	in, err := normalizeQuality(in)
	if err != nil {
		return QualityResult{}, err
	}
	body, err := json.Marshal(in)
	if err != nil {
		return QualityResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return QualityResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return QualityResult{}, err
	}
	result, repeated, err := qualityReplay(ctx, tx, a, d, in.OperationID, string(body))
	if err != nil {
		return QualityResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	lot, err := scanLot(tx.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LotID))
	if err != nil {
		return QualityResult{}, err
	}
	if lot.Status != "recorded" || lot.Revision != 1 {
		return QualityResult{}, ErrQuality
	}
	revision, err := qualityRevisionTx(ctx, tx, a, lot.ID)
	if err != nil {
		return QualityResult{}, err
	}
	if revision != in.ExpectedRevision {
		return QualityResult{}, ErrConflict
	}
	// Validate the immutable metadata before snapshotting a verdict. No current
	// catalog lookup: a historic unit must never be silently reinterpreted.
	if !validUnit(lot.Unit) || lot.QuantityMilli < 1 || lot.QuantityMilli > MaxQuantity || (lot.Unit == "unit" && lot.QuantityMilli%1000 != 0) || !validLotMetadata(lot.Code, lot.ManufacturedOn, lot.ExpiresOn) {
		return QualityResult{}, ErrQuality
	}
	snapshot, err := json.Marshal(lot)
	if err != nil {
		return QualityResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result = QualityResult{LotID: lot.ID, Revision: revision + 1, Status: in.Status}
	encoded, err := json.Marshal(result)
	if err != nil {
		return QualityResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO production_quality_reviews VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, lot.ID, result.Revision, d.DeviceID, in.OperationID, a.IdentityID, in.Status, in.Criterion, in.Reason, string(snapshot), string(body), string(encoded), now); err != nil {
		return QualityResult{}, err
	}
	payload, err := json.Marshal(struct {
		ActorID     string          `json:"actor_id"`
		Request     json.RawMessage `json:"request"`
		Result      QualityResult   `json:"result"`
		LotSnapshot ProductionLot   `json:"lot_snapshot"`
	}{a.IdentityID, json.RawMessage(body), result, lot})
	if err != nil {
		return QualityResult{}, err
	}
	id, err := localdb.NewID()
	if err != nil {
		return QualityResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, lot.ID, "production.quality.reviewed", 1, string(payload), now); err != nil {
		return QualityResult{}, err
	}
	return result, tx.Commit()
}
func GetLotQuality(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (LotQuality, error) {
	if !validID(id) {
		return LotQuality{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return LotQuality{}, err
	}
	defer tx.Rollback()
	lot, err := scanLot(tx.QueryRowContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id))
	if err != nil {
		return LotQuality{}, err
	}
	revision, err := qualityRevisionTx(ctx, tx, a, id)
	if err != nil {
		return LotQuality{}, err
	}
	out := LotQuality{LotID: id, LotStatus: lot.Status, Revision: revision, Status: "not_assessed"}
	if revision > 0 {
		v, e := scanQuality(tx.QueryRowContext(ctx, `SELECT `+qualityColumns+` FROM production_quality_reviews WHERE tenant_id=? AND store_id=? AND lot_id=? AND revision=?`, a.TenantID, a.StoreID, id, revision))
		if e != nil {
			return LotQuality{}, e
		}
		out.Latest = &v
		out.Status = v.Status
	}
	return out, tx.Commit()
}
func LotQualityHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int) ([]QualityReview, error) {
	if !validID(id) || offset < 0 {
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
	if _, err = qualityRevisionTx(ctx, tx, a, id); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+qualityColumns+` FROM production_quality_reviews WHERE tenant_id=? AND store_id=? AND lot_id=? ORDER BY revision LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, id, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QualityReview{}
	for rows.Next() {
		v, e := scanQuality(rows)
		if e != nil {
			return nil, e
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
