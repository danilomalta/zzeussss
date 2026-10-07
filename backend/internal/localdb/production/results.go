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

var ErrResult = errors.New("resultado de producao ou consumo incompatível")

type ResultInput struct {
	OperationID      string `json:"operation_id"`
	ResultID         string `json:"result_id"`
	OrderID          string `json:"order_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	ProducedMilli    int64  `json:"produced_milli"`
	Reason           string `json:"reason"`
}

type CompletionResult struct {
	ResultID       string `json:"result_id"`
	OrderID        string `json:"order_id"`
	ReservationID  string `json:"reservation_id"`
	ProductID      string `json:"product_id"`
	LocationID     string `json:"location_id"`
	Unit           string `json:"unit"`
	PlannedMilli   int64  `json:"planned_milli"`
	ProducedMilli  int64  `json:"produced_milli"`
	ShortfallMilli int64  `json:"shortfall_milli"`
	Revision       int64  `json:"revision"`
	Status         string `json:"status"`
	Repeated       bool   `json:"repeated"`
}

type ProductionResult struct {
	CompletionResult
	OperationID string `json:"operation_id"`
	DeviceID    string `json:"device_id"`
	ActorID     string `json:"actor_id"`
	Reason      string `json:"reason"`
	MovementID  string `json:"movement_id,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func resultReplay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, body string) (CompletionResult, bool, error) {
	var store, actor, request, result string
	err := tx.QueryRowContext(ctx, `SELECT store_id,actor_id,request_json,result_json FROM production_results WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, op).Scan(&store, &actor, &request, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return CompletionResult{}, false, nil
	}
	if err != nil {
		return CompletionResult{}, false, err
	}
	if store != a.StoreID || actor != a.IdentityID || request != body {
		return CompletionResult{}, false, ErrConflict
	}
	var out CompletionResult
	if err = json.Unmarshal([]byte(result), &out); err != nil {
		return CompletionResult{}, false, err
	}
	out.Repeated = true
	return out, true, nil
}

// Verify the consumed ingredient snapshot and the actual negative movements,
// rather than accepting the reservation status alone as proof of consumption.
func consumedForOrder(ctx context.Context, tx *sql.Tx, a identity.Scope, order Order) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM production_material_reservations WHERE tenant_id=? AND store_id=? AND order_id=? AND status='consumed'`, a.TenantID, a.StoreID, order.ID)
	if err != nil {
		return "", err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return "", err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	if len(ids) != 1 {
		return "", ErrResult
	}
	reservation, err := materialGetTx(ctx, tx, a, ids[0])
	if err != nil {
		return "", err
	}
	if reservation.LocationID != order.LocationID || len(reservation.Items) != len(order.Recipe.Ingredients) {
		return "", ErrResult
	}
	expected := map[string]Ingredient{}
	for _, item := range order.Recipe.Ingredients {
		if order.PlannedBatches > MaxQuantity/item.QuantityMilli {
			return "", ErrResult
		}
		item.QuantityMilli *= order.PlannedBatches
		expected[item.ProductID] = item
	}
	for _, item := range reservation.Items {
		want, ok := expected[item.ProductID]
		if !ok || item != want {
			return "", ErrResult
		}
		var tenant, store, product, location string
		var quantity int64
		err = tx.QueryRowContext(ctx, `SELECT m.tenant_id,m.store_id,m.product_id,m.location_id,m.quantity_milli FROM production_material_movements p JOIN stock_movements m ON m.id=p.movement_id WHERE p.tenant_id=? AND p.store_id=? AND p.reservation_id=? AND p.product_id=?`, a.TenantID, a.StoreID, reservation.ID, item.ProductID).Scan(&tenant, &store, &product, &location, &quantity)
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrResult
		}
		if err != nil {
			return "", err
		}
		if tenant != a.TenantID || store != a.StoreID || product != item.ProductID || location != order.LocationID || quantity != -item.QuantityMilli {
			return "", ErrResult
		}
	}
	return reservation.ID, nil
}

func CompleteProduction(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in ResultInput) (CompletionResult, error) {
	if db == nil {
		return CompletionResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.ResultID) || !validID(in.OrderID) || in.ExpectedRevision < 1 || in.ExpectedRevision >= MaxRevision || in.ProducedMilli < 0 || in.ProducedMilli > MaxQuantity || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return CompletionResult{}, ErrInvalid
	}
	body, err := json.Marshal(in)
	if err != nil {
		return CompletionResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CompletionResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return CompletionResult{}, err
	}
	result, repeated, err := resultReplay(ctx, tx, a, d, in.OperationID, string(body))
	if err != nil {
		return CompletionResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_results WHERE tenant_id=? AND store_id=? AND (id=? OR order_id=?)`, a.TenantID, a.StoreID, in.ResultID, in.OrderID).Scan(&count); err != nil {
		return CompletionResult{}, err
	}
	if count != 0 {
		return CompletionResult{}, ErrConflict
	}
	order, err := scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID))
	if err != nil {
		return CompletionResult{}, err
	}
	if order.Status != "approved" || order.Revision != in.ExpectedRevision {
		return CompletionResult{}, ErrConflict
	}
	if _, _, err = normalize(order.Recipe.PublishInput); err != nil || order.PlannedBatches < 1 || order.PlannedBatches > MaxQuantity/order.Recipe.YieldMilli || order.PlannedOutputMilli != order.PlannedBatches*order.Recipe.YieldMilli {
		return CompletionResult{}, ErrResult
	}
	responsible := a
	responsible.IdentityID = order.ResponsibleID
	if err = identity.CanOperateTx(ctx, tx, responsible, d, identity.ManageProduction); err != nil {
		return CompletionResult{}, err
	}
	if in.ProducedMilli > order.PlannedOutputMilli || (order.Recipe.OutputUnit == "unit" && in.ProducedMilli%1000 != 0) {
		return CompletionResult{}, ErrInvalid
	}
	if err = validateMaterialUnit(ctx, tx, a, Ingredient{ProductID: order.Recipe.OutputProductID, Unit: order.Recipe.OutputUnit}); err != nil {
		if errors.Is(err, ErrMaterials) || errors.Is(err, sql.ErrNoRows) {
			return CompletionResult{}, ErrResult
		}
		return CompletionResult{}, err
	}
	reservationID, err := consumedForOrder(ctx, tx, a, order)
	if err != nil {
		return CompletionResult{}, err
	}
	balance, err := locationBalance(ctx, tx, a, order.Recipe.OutputProductID, order.LocationID)
	if err != nil {
		return CompletionResult{}, err
	}
	if balance > MaxQuantity-in.ProducedMilli {
		return CompletionResult{}, ErrResult
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var movement any
	if in.ProducedMilli > 0 {
		id, e := localdb.NewID()
		if e != nil {
			return CompletionResult{}, e
		}
		movement = id
		if err = one(ctx, tx, `INSERT INTO stock_movements VALUES(?,?,?,?,?,?,?,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, order.Recipe.OutputProductID, order.LocationID, in.ProducedMilli, "production-result:"+in.ResultID, now); err != nil {
			return CompletionResult{}, err
		}
	}
	result = CompletionResult{ResultID: in.ResultID, OrderID: order.ID, ReservationID: reservationID, ProductID: order.Recipe.OutputProductID, LocationID: order.LocationID, Unit: order.Recipe.OutputUnit, PlannedMilli: order.PlannedOutputMilli, ProducedMilli: in.ProducedMilli, ShortfallMilli: order.PlannedOutputMilli - in.ProducedMilli, Revision: order.Revision + 1, Status: "completed"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return CompletionResult{}, err
	}
	// Historical status CHECK from migration 0029 is preserved. An immutable
	// result is the durable completion state, projected in order reads/history.
	if err = one(ctx, tx, `UPDATE production_orders SET revision=?,updated_at=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=? AND status='approved'`, result.Revision, now, a.TenantID, a.StoreID, order.ID, order.Revision); err != nil {
		return CompletionResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO production_results VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.ResultID, order.ID, reservationID, d.DeviceID, in.OperationID, a.IdentityID, result.ProductID, result.LocationID, result.Unit, result.PlannedMilli, result.ProducedMilli, result.ShortfallMilli, result.Revision, result.Status, in.Reason, movement, string(body), string(encoded), now); err != nil {
		return CompletionResult{}, err
	}
	payload, err := json.Marshal(struct {
		ActorID    string           `json:"actor_id"`
		Request    json.RawMessage  `json:"request"`
		Result     CompletionResult `json:"result"`
		MovementID any              `json:"movement_id"`
	}{a.IdentityID, json.RawMessage(body), result, movement})
	if err != nil {
		return CompletionResult{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return CompletionResult{}, err
	}
	if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, in.ResultID, "production.result.completed", 1, string(payload), now); err != nil {
		return CompletionResult{}, err
	}
	return result, tx.Commit()
}

func GetProductionResult(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (ProductionResult, error) {
	if !validID(id) {
		return ProductionResult{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return ProductionResult{}, err
	}
	defer tx.Rollback()
	var out ProductionResult
	var movement sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,order_id,reservation_id,product_id,location_id,unit,planned_milli,produced_milli,shortfall_milli,order_revision,status,operation_id,device_id,actor_id,reason,movement_id,created_at FROM production_results WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.ResultID, &out.OrderID, &out.ReservationID, &out.ProductID, &out.LocationID, &out.Unit, &out.PlannedMilli, &out.ProducedMilli, &out.ShortfallMilli, &out.Revision, &out.Status, &out.OperationID, &out.DeviceID, &out.ActorID, &out.Reason, &movement, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProductionResult{}, ErrNotFound
	}
	if err != nil {
		return ProductionResult{}, err
	}
	out.MovementID = movement.String
	return out, tx.Commit()
}
