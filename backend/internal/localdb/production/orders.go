package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/localdb/catalog"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type OrderInput struct {
	OperationID    string `json:"operation_id"`
	OrderID        string `json:"order_id"`
	VersionID      string `json:"version_id"`
	LocationID     string `json:"location_id"`
	ResponsibleID  string `json:"responsible_id"`
	PlannedBatches int64  `json:"planned_batches"`
}

type OrderStateInput struct {
	OperationID      string `json:"operation_id"`
	OrderID          string `json:"order_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
}

type OrderResult struct {
	OrderID  string `json:"order_id"`
	Revision int64  `json:"revision"`
	Status   string `json:"status"`
	Repeated bool   `json:"repeated"`
}

type Order struct {
	ID                 string  `json:"id"`
	VersionID          string  `json:"version_id"`
	LocationID         string  `json:"location_id"`
	ResponsibleID      string  `json:"responsible_id"`
	PlannedBatches     int64   `json:"planned_batches"`
	PlannedOutputMilli int64   `json:"planned_output_milli"`
	Recipe             Version `json:"recipe"`
	Status             string  `json:"status"`
	Revision           int64   `json:"revision"`
	CreatedBy          string  `json:"created_by"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	CompletionID       string  `json:"completion_id,omitempty"`
}

type OrderEvent struct {
	OperationID  string `json:"operation_id"`
	ActorID      string `json:"actor_id"`
	Kind         string `json:"kind"`
	BeforeStatus string `json:"before_status"`
	AfterStatus  string `json:"after_status"`
	Revision     int64  `json:"revision"`
	Reason       string `json:"reason"`
	CreatedAt    string `json:"created_at"`
}

func orderReplay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, operationID, kind, body string) (OrderResult, bool, error) {
	var store, actor, savedKind, savedBody, result string
	err := tx.QueryRowContext(ctx, `SELECT store_id,actor_id,kind,request_json,result_json FROM production_order_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, operationID).Scan(&store, &actor, &savedKind, &savedBody, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return OrderResult{}, false, nil
	}
	if err != nil {
		return OrderResult{}, false, err
	}
	if store != a.StoreID || actor != a.IdentityID || kind != savedKind || body != savedBody {
		return OrderResult{}, false, ErrConflict
	}
	var out OrderResult
	if err = json.Unmarshal([]byte(result), &out); err != nil {
		return OrderResult{}, false, err
	}
	out.Repeated = true
	return out, true, nil
}

func recordOrderEvent(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, operationID, kind, body, before, reason, now string, result OrderResult) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = one(ctx, tx, `INSERT INTO production_order_events VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, operationID, a.IdentityID, result.OrderID, kind, body, before, result.Status, result.Revision, reason, string(b), now); err != nil {
		return err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return err
	}
	// Durable event describes an order change, NOT material consumption.
	payload, err := json.Marshal(struct {
		OrderID  string          `json:"order_id"`
		Revision int64           `json:"revision"`
		Status   string          `json:"status"`
		Kind     string          `json:"kind"`
		ActorID  string          `json:"actor_id"`
		Request  json.RawMessage `json:"request"`
	}{result.OrderID, result.Revision, result.Status, kind, a.IdentityID, json.RawMessage(body)})
	if err != nil {
		return err
	}
	return one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, operationID, result.OrderID, "production.order.changed", 1, string(payload), now)
}

func CreateOrder(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in OrderInput) (OrderResult, error) {
	if db == nil {
		return OrderResult{}, errors.New("banco local indisponivel")
	}
	if !validID(in.OperationID) || !validID(in.OrderID) || !validID(in.VersionID) || !validID(in.LocationID) || !validID(in.ResponsibleID) || in.PlannedBatches < 1 || in.PlannedBatches > MaxQuantity {
		return OrderResult{}, ErrInvalid
	}
	request, err := json.Marshal(in)
	if err != nil {
		return OrderResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return OrderResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return OrderResult{}, err
	}
	result, repeated, err := orderReplay(ctx, tx, a, d, in.OperationID, "created", string(request))
	if err != nil {
		return OrderResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&count); err != nil {
		return OrderResult{}, err
	}
	if count != 0 {
		return OrderResult{}, ErrConflict
	}
	v, err := scanVersion(tx.QueryRowContext(ctx, `SELECT request_json,revision,created_at,actor_id FROM production_recipe_versions WHERE tenant_id=? AND store_id=? AND version_id=?`, a.TenantID, a.StoreID, in.VersionID))
	if err != nil {
		return OrderResult{}, err
	}
	if v.VersionID != in.VersionID {
		return OrderResult{}, ErrConflict
	}
	state, err := recipeStateTx(ctx, tx, a, v.RecipeID)
	if err != nil {
		return OrderResult{}, err
	}
	if state.Status != "active" {
		return OrderResult{}, ErrConflict
	}
	if _, _, err = normalize(v.PublishInput); err != nil {
		return OrderResult{}, err
	}
	// Existing orders keep their snapshots. A NEW plan must not introduce
	// quantities interpreted with a catalog unit different from its recipe.
	units := append([]Ingredient{{ProductID: v.OutputProductID, Unit: v.OutputUnit}}, v.Ingredients...)
	for _, item := range units {
		var unit string
		err = tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, item.ProductID).Scan(&unit)
		if errors.Is(err, sql.ErrNoRows) {
			return OrderResult{}, ErrConflict
		}
		if err != nil {
			return OrderResult{}, err
		}
		if e := catalog.RequireActiveProductTx(ctx, tx, a.TenantID, item.ProductID); e != nil {
			return OrderResult{}, e
		}
		if unit != item.Unit {
			return OrderResult{}, ErrConflict
		}
	}
	if in.PlannedBatches > MaxQuantity/v.YieldMilli {
		return OrderResult{}, ErrInvalid
	}
	for _, item := range v.Ingredients {
		if in.PlannedBatches > MaxQuantity/item.QuantityMilli {
			return OrderResult{}, ErrInvalid
		}
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_locations WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LocationID).Scan(&count); err != nil {
		return OrderResult{}, err
	}
	if count != 1 {
		return OrderResult{}, ErrNotFound
	}
	// Assignment checks the target's current store permission. It is not a
	// signature/action by that person; the authenticated actor remains the author.
	responsible := a
	responsible.IdentityID = in.ResponsibleID
	if err = identity.CanOperateTx(ctx, tx, responsible, d, identity.ManageProduction); err != nil {
		return OrderResult{}, err
	}
	snapshot, err := json.Marshal(v)
	if err != nil {
		return OrderResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result = OrderResult{OrderID: in.OrderID, Revision: 1, Status: "planned"}
	if err = one(ctx, tx, `INSERT INTO production_orders VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.OrderID, in.VersionID, in.LocationID, in.ResponsibleID, in.PlannedBatches, in.PlannedBatches*v.YieldMilli, string(snapshot), result.Status, result.Revision, a.IdentityID, now, now); err != nil {
		return OrderResult{}, err
	}
	if err = recordOrderEvent(ctx, tx, a, d, in.OperationID, "created", string(request), "", "", now, result); err != nil {
		return OrderResult{}, err
	}
	return result, tx.Commit()
}

// Approval authorizes a PLAN, not material availability. Execution/completion
// remain unavailable until material reservation/consumption/results exist.
func ChangeOrderState(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in OrderStateInput) (OrderResult, error) {
	if db == nil {
		return OrderResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.OrderID) || in.ExpectedRevision < 1 || in.ExpectedRevision >= MaxRevision || (in.Status != "approved" && in.Status != "cancelled") || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return OrderResult{}, ErrInvalid
	}
	request, err := json.Marshal(in)
	if err != nil {
		return OrderResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return OrderResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return OrderResult{}, err
	}
	result, repeated, err := orderReplay(ctx, tx, a, d, in.OperationID, "state", string(request))
	if err != nil {
		return OrderResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	var status string
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT status,revision FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&status, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return OrderResult{}, ErrNotFound
	}
	if err != nil {
		return OrderResult{}, err
	}
	if revision != in.ExpectedRevision || !(status == "planned" || (status == "approved" && in.Status == "cancelled")) {
		return OrderResult{}, ErrConflict
	}
	if in.Status == "approved" {
		responsible := a
		if err = tx.QueryRowContext(ctx, `SELECT responsible_id FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&responsible.IdentityID); err != nil {
			return OrderResult{}, err
		}
		if err = identity.CanOperateTx(ctx, tx, responsible, d, identity.ManageProduction); err != nil {
			return OrderResult{}, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = guardMaterialCancellation(ctx, tx, a, in.OrderID, in.Status); err != nil {
		return OrderResult{}, err
	}
	result = OrderResult{OrderID: in.OrderID, Revision: revision + 1, Status: in.Status}
	if err = one(ctx, tx, `UPDATE production_orders SET status=?,revision=?,updated_at=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=? AND status=?`, result.Status, result.Revision, now, a.TenantID, a.StoreID, in.OrderID, revision, status); err != nil {
		return OrderResult{}, err
	}
	if err = recordOrderEvent(ctx, tx, a, d, in.OperationID, "state", string(request), status, in.Reason, now, result); err != nil {
		return OrderResult{}, err
	}
	return result, tx.Commit()
}

const orderColumns = `id,version_id,location_id,responsible_id,planned_batches,planned_output_milli,recipe_json,
CASE WHEN EXISTS(SELECT 1 FROM production_results r WHERE r.tenant_id=production_orders.tenant_id AND r.store_id=production_orders.store_id AND r.order_id=production_orders.id) THEN 'completed' ELSE status END,
revision,created_by,created_at,updated_at,
COALESCE((SELECT r.id FROM production_results r WHERE r.tenant_id=production_orders.tenant_id AND r.store_id=production_orders.store_id AND r.order_id=production_orders.id),'')`

func scanOrder(row interface{ Scan(...any) error }) (Order, error) {
	var out Order
	var recipe string
	err := row.Scan(&out.ID, &out.VersionID, &out.LocationID, &out.ResponsibleID, &out.PlannedBatches, &out.PlannedOutputMilli, &recipe, &out.Status, &out.Revision, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt, &out.CompletionID)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	if err = json.Unmarshal([]byte(recipe), &out.Recipe); err != nil {
		return Order{}, err
	}
	return out, nil
}

func GetOrder(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Order, error) {
	if !validID(id) {
		return Order{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	out, err := scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id))
	if err != nil {
		return Order{}, err
	}
	return out, tx.Commit()
}

func ListOrders(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, offset int) ([]Order, error) {
	if offset < 0 {
		return nil, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE tenant_id=? AND store_id=? ORDER BY created_at DESC,id DESC LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		v, e := scanOrder(rows)
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

func OrderHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int) ([]OrderEvent, error) {
	if !validID(id) || offset < 0 {
		return nil, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id,actor_id,kind,before_status,after_status,revision,reason,created_at FROM production_order_events WHERE tenant_id=? AND store_id=? AND order_id=?
UNION ALL SELECT operation_id,actor_id,'completed','approved','completed',order_revision,reason,created_at FROM production_results WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY revision LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, id, a.TenantID, a.StoreID, id, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrderEvent{}
	for rows.Next() {
		var e OrderEvent
		if err = rows.Scan(&e.OperationID, &e.ActorID, &e.Kind, &e.BeforeStatus, &e.AfterStatus, &e.Revision, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
