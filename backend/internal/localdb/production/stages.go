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

var ErrStages = errors.New("etapas de producao incompletas ou incompativeis")

type StageDefinition struct {
	StageID       string `json:"stage_id"`
	Name          string `json:"name"`
	ResponsibleID string `json:"responsible_id"`
}
type StagePlanInput struct {
	OperationID string            `json:"operation_id"`
	OrderID     string            `json:"order_id"`
	Stages      []StageDefinition `json:"stages"`
	Reason      string            `json:"reason"`
}
type StageStateInput struct {
	OperationID      string `json:"operation_id"`
	OrderID          string `json:"order_id"`
	StageID          string `json:"stage_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
}
type StageResult struct {
	OrderID  string `json:"order_id"`
	StageID  string `json:"stage_id,omitempty"`
	Revision int64  `json:"revision"`
	Status   string `json:"status"`
	Repeated bool   `json:"repeated"`
}
type ProductionStage struct {
	StageDefinition
	Position  int    `json:"position"`
	Status    string `json:"status"`
	Revision  int64  `json:"revision"`
	UpdatedAt string `json:"updated_at"`
}
type StagePlan struct {
	OrderID   string            `json:"order_id"`
	CreatedBy string            `json:"created_by"`
	CreatedAt string            `json:"created_at"`
	Items     []ProductionStage `json:"items"`
}
type StageEvent struct {
	Sequence    int64           `json:"sequence"`
	OperationID string          `json:"operation_id"`
	DeviceID    string          `json:"device_id"`
	ActorID     string          `json:"actor_id"`
	Kind        string          `json:"kind"`
	Reason      string          `json:"reason"`
	Request     json.RawMessage `json:"request"`
	Result      json.RawMessage `json:"result"`
	CreatedAt   string          `json:"created_at"`
}

func normalizeStagePlan(in StagePlanInput) (StagePlanInput, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.OrderID) || len(in.Reason) < 1 || len(in.Reason) > 255 || len(in.Stages) < 1 || len(in.Stages) > 20 {
		return StagePlanInput{}, ErrInvalid
	}
	in.Stages = append([]StageDefinition(nil), in.Stages...)
	seen := map[string]bool{}
	for i := range in.Stages {
		v := &in.Stages[i]
		v.Name = strings.TrimSpace(v.Name)
		if !validID(v.StageID) || !validID(v.ResponsibleID) || len(v.Name) < 1 || len(v.Name) > 120 || seen[v.StageID] {
			return StagePlanInput{}, ErrInvalid
		}
		seen[v.StageID] = true
	}
	return in, nil
}
func stageReplay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, body string) (StageResult, bool, error) {
	var store, actor, savedKind, request, result string
	err := tx.QueryRowContext(ctx, `SELECT store_id,actor_id,kind,request_json,result_json FROM production_stage_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, op).Scan(&store, &actor, &savedKind, &request, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return StageResult{}, false, nil
	}
	if err != nil {
		return StageResult{}, false, err
	}
	if store != a.StoreID || actor != a.IdentityID || savedKind != kind || request != body {
		return StageResult{}, false, ErrConflict
	}
	var out StageResult
	if err = json.Unmarshal([]byte(result), &out); err != nil {
		return StageResult{}, false, err
	}
	out.Repeated = true
	return out, true, nil
}
func stageEvent(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, reason, body, now string, result StageResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = one(ctx, tx, `INSERT INTO production_stage_events(tenant_id,store_id,order_id,device_id,operation_id,actor_id,kind,reason,request_json,result_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, result.OrderID, d.DeviceID, op, a.IdentityID, kind, reason, body, string(encoded), now); err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		ActorID string          `json:"actor_id"`
		Request json.RawMessage `json:"request"`
		Result  StageResult     `json:"result"`
	}{a.IdentityID, json.RawMessage(body), result})
	if err != nil {
		return err
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	return one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, op, result.OrderID, "production.stage."+kind, 1, string(payload), now)
}

func ConfigureStagePlan(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in StagePlanInput) (StageResult, error) {
	if db == nil {
		return StageResult{}, errors.New("banco local indisponivel")
	}
	in, err := normalizeStagePlan(in)
	if err != nil {
		return StageResult{}, err
	}
	body, err := json.Marshal(in)
	if err != nil {
		return StageResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return StageResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return StageResult{}, err
	}
	result, repeated, err := stageReplay(ctx, tx, a, d, in.OperationID, "configured", string(body))
	if err != nil {
		return StageResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	order, err := scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID))
	if err != nil {
		return StageResult{}, err
	}
	if order.Status != "planned" && order.Status != "approved" {
		return StageResult{}, ErrStages
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_stage_plans WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, order.ID).Scan(&count); err != nil {
		return StageResult{}, err
	}
	if count != 0 {
		return StageResult{}, ErrConflict
	}
	// Definition cannot be added after reserving/consuming ingredients, avoiding
	// retroactive process requirements. Released holds do not block a new plan.
	if err = guardMaterialCancellation(ctx, tx, a, order.ID, "cancelled"); err != nil {
		return StageResult{}, err
	}
	for _, stage := range in.Stages {
		responsible := a
		responsible.IdentityID = stage.ResponsibleID
		if err = identity.CanOperateTx(ctx, tx, responsible, d, identity.ManageProduction); err != nil {
			return StageResult{}, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO production_stage_plans VALUES(?,?,?,?,?,?)`, a.TenantID, a.StoreID, order.ID, len(in.Stages), a.IdentityID, now); err != nil {
		return StageResult{}, err
	}
	for i, stage := range in.Stages {
		if err = one(ctx, tx, `INSERT INTO production_stages VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, order.ID, stage.StageID, i+1, stage.Name, stage.ResponsibleID, "pending", 1, now); err != nil {
			return StageResult{}, err
		}
	}
	result = StageResult{OrderID: order.ID, Revision: 1, Status: "configured"}
	if err = stageEvent(ctx, tx, a, d, in.OperationID, "configured", in.Reason, string(body), now, result); err != nil {
		return StageResult{}, err
	}
	return result, tx.Commit()
}

const stageColumns = `id,name,responsible_id,position,status,revision,updated_at`

func scanStage(row interface{ Scan(...any) error }) (ProductionStage, error) {
	var out ProductionStage
	err := row.Scan(&out.StageID, &out.Name, &out.ResponsibleID, &out.Position, &out.Status, &out.Revision, &out.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProductionStage{}, ErrNotFound
	}
	return out, err
}
func stagePlanTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (StagePlan, error) {
	out := StagePlan{OrderID: id, Items: []ProductionStage{}}
	var count int
	err := tx.QueryRowContext(ctx, `SELECT stage_count,created_by,created_at FROM production_stage_plans WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, id).Scan(&count, &out.CreatedBy, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return StagePlan{}, ErrNotFound
	}
	if err != nil {
		return StagePlan{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+stageColumns+` FROM production_stages WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY position`, a.TenantID, a.StoreID, id)
	if err != nil {
		return StagePlan{}, err
	}
	defer rows.Close()
	for rows.Next() {
		v, e := scanStage(rows)
		if e != nil {
			return StagePlan{}, e
		}
		out.Items = append(out.Items, v)
	}
	if err = rows.Err(); err != nil {
		return StagePlan{}, err
	}
	if err = rows.Close(); err != nil {
		return StagePlan{}, err
	}
	if count < 1 || count > 20 || len(out.Items) != count {
		return StagePlan{}, ErrStages
	}
	for i, v := range out.Items {
		if v.Position != i+1 {
			return StagePlan{}, ErrStages
		}
	}
	return out, nil
}

// Orders without a stage plan retain the P06 completion contract. A configured
// plan must be complete, including its entire contiguous immutable definition.
func guardStagesCompleted(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) error {
	plan, err := stagePlanTx(ctx, tx, a, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, v := range plan.Items {
		if v.Status != "completed" || v.Revision != 3 {
			return ErrStages
		}
	}
	return nil
}
func ChangeStageState(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in StageStateInput) (StageResult, error) {
	if db == nil {
		return StageResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.OrderID) || !validID(in.StageID) || in.ExpectedRevision < 1 || in.ExpectedRevision > MaxRevision || (in.Status != "running" && in.Status != "completed") || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return StageResult{}, ErrInvalid
	}
	body, err := json.Marshal(in)
	if err != nil {
		return StageResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return StageResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return StageResult{}, err
	}
	result, repeated, err := stageReplay(ctx, tx, a, d, in.OperationID, in.Status, string(body))
	if err != nil {
		return StageResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	order, err := scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID))
	if err != nil {
		return StageResult{}, err
	}
	if order.Status != "approved" {
		return StageResult{}, ErrStages
	}
	plan, err := stagePlanTx(ctx, tx, a, order.ID)
	if err != nil {
		return StageResult{}, err
	}
	var stage ProductionStage
	found := false
	for _, v := range plan.Items {
		if v.StageID == in.StageID {
			stage = v
			found = true
		}
	}
	if !found {
		return StageResult{}, ErrNotFound
	}
	if stage.ResponsibleID != a.IdentityID {
		return StageResult{}, identity.ErrDenied
	}
	if stage.Revision != in.ExpectedRevision || (in.Status == "running" && stage.Status != "pending") || (in.Status == "completed" && stage.Status != "running") {
		return StageResult{}, ErrConflict
	}
	for _, v := range plan.Items {
		if v.Position < stage.Position && (v.Status != "completed" || v.Revision != 3) {
			return StageResult{}, ErrStages
		}
	}
	// Stages never consume stock. P05 must have already recorded the actual
	// consumption proof; the same proof is required again by P06 completion.
	if _, err = consumedForOrder(ctx, tx, a, order); err != nil {
		return StageResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `UPDATE production_stages SET status=?,revision=?,updated_at=? WHERE tenant_id=? AND store_id=? AND order_id=? AND id=? AND status=? AND revision=?`, in.Status, stage.Revision+1, now, a.TenantID, a.StoreID, order.ID, stage.StageID, stage.Status, stage.Revision); err != nil {
		return StageResult{}, err
	}
	result = StageResult{OrderID: order.ID, StageID: stage.StageID, Revision: stage.Revision + 1, Status: in.Status}
	if err = stageEvent(ctx, tx, a, d, in.OperationID, in.Status, in.Reason, string(body), now, result); err != nil {
		return StageResult{}, err
	}
	return result, tx.Commit()
}
func GetStagePlan(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (StagePlan, error) {
	if !validID(id) {
		return StagePlan{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return StagePlan{}, err
	}
	defer tx.Rollback()
	out, err := stagePlanTx(ctx, tx, a, id)
	if err != nil {
		return StagePlan{}, err
	}
	return out, tx.Commit()
}
func StageHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int) ([]StageEvent, error) {
	if !validID(id) || offset < 0 {
		return nil, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = stagePlanTx(ctx, tx, a, id); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT sequence,operation_id,device_id,actor_id,kind,reason,request_json,result_json,created_at FROM production_stage_events WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY sequence LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, id, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StageEvent{}
	for rows.Next() {
		var v StageEvent
		var req, res string
		if err = rows.Scan(&v.Sequence, &v.OperationID, &v.DeviceID, &v.ActorID, &v.Kind, &v.Reason, &req, &res, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Request = json.RawMessage(req)
		v.Result = json.RawMessage(res)
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
