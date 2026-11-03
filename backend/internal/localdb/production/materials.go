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
	"titansystem-backend/internal/localdb/stockreservation"
)

var ErrMaterials = errors.New("materiais indisponiveis ou estado de reserva conflitante")

type ReserveInput struct {
	OperationID   string `json:"operation_id"`
	ReservationID string `json:"reservation_id"`
	OrderID       string `json:"order_id"`
	Reason        string `json:"reason"`
}

type MaterialChangeInput struct {
	OperationID   string `json:"operation_id"`
	ReservationID string `json:"reservation_id"`
	Action        string `json:"action"`
	Reason        string `json:"reason"`
}

type MaterialResult struct {
	ReservationID string `json:"reservation_id"`
	OrderID       string `json:"order_id"`
	Status        string `json:"status"`
	Repeated      bool   `json:"repeated"`
}

type MaterialReservation struct {
	ID         string       `json:"id"`
	OrderID    string       `json:"order_id"`
	LocationID string       `json:"location_id"`
	Status     string       `json:"status"`
	CreatedBy  string       `json:"created_by"`
	CreatedAt  string       `json:"created_at"`
	UpdatedAt  string       `json:"updated_at"`
	Items      []Ingredient `json:"items"`
}

func materialReplay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, body string) (MaterialResult, bool, error) {
	var actor, store, savedKind, request, result string
	err := tx.QueryRowContext(ctx, `SELECT actor_id,store_id,kind,request_json,result_json FROM production_material_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, op).Scan(&actor, &store, &savedKind, &request, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return MaterialResult{}, false, nil
	}
	if err != nil {
		return MaterialResult{}, false, err
	}
	if actor != a.IdentityID || store != a.StoreID || kind != savedKind || request != body {
		return MaterialResult{}, false, ErrConflict
	}
	var v MaterialResult
	if err = json.Unmarshal([]byte(result), &v); err != nil {
		return MaterialResult{}, false, err
	}
	v.Repeated = true
	return v, true, nil
}

func materialEvent(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, kind, body, now string, result MaterialResult) error {
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = one(ctx, tx, `INSERT INTO production_material_events VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID, result.ReservationID, kind, body, string(b), now); err != nil {
		return err
	}
	reservation, err := materialGetTx(ctx, tx, a, result.ReservationID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		Kind        string              `json:"kind"`
		ActorID     string              `json:"actor_id"`
		Request     json.RawMessage     `json:"request"`
		Reservation MaterialReservation `json:"reservation"`
	}{kind, a.IdentityID, json.RawMessage(body), reservation})
	if err != nil {
		return err
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	return one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, op, result.ReservationID, "production.materials.changed", 1, string(payload), now)
}

func materialGetTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (MaterialReservation, error) {
	v := MaterialReservation{Items: []Ingredient{}}
	err := tx.QueryRowContext(ctx, `SELECT id,order_id,location_id,status,created_by,created_at,updated_at FROM production_material_reservations WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&v.ID, &v.OrderID, &v.LocationID, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return MaterialReservation{}, ErrNotFound
	}
	if err != nil {
		return MaterialReservation{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT product_id,unit,quantity_milli FROM production_material_items WHERE tenant_id=? AND store_id=? AND reservation_id=? ORDER BY product_id`, a.TenantID, a.StoreID, id)
	if err != nil {
		return MaterialReservation{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Ingredient
		if err = rows.Scan(&item.ProductID, &item.Unit, &item.QuantityMilli); err != nil {
			return MaterialReservation{}, err
		}
		v.Items = append(v.Items, item)
	}
	if err = rows.Err(); err != nil {
		return MaterialReservation{}, err
	}
	if len(v.Items) < 1 {
		return MaterialReservation{}, ErrMaterials
	}
	return v, nil
}

func validateMaterialUnit(ctx context.Context, tx *sql.Tx, a identity.Scope, item Ingredient) error {
	var unit string
	err := tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, item.ProductID).Scan(&unit)
	if err != nil {
		return err
	}
	if unit != item.Unit {
		return ErrMaterials
	}
	return nil
}

func ReserveMaterials(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in ReserveInput) (MaterialResult, error) {
	if db == nil {
		return MaterialResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.ReservationID) || !validID(in.OrderID) || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return MaterialResult{}, ErrInvalid
	}
	body, err := json.Marshal(in)
	if err != nil {
		return MaterialResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return MaterialResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return MaterialResult{}, err
	}
	result, repeated, err := materialReplay(ctx, tx, a, d, in.OperationID, "reserve", string(body))
	if err != nil {
		return MaterialResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	order, err := scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID))
	if err != nil {
		return MaterialResult{}, err
	}
	if order.Status != "approved" {
		return MaterialResult{}, ErrMaterials
	}
	responsible := a
	responsible.IdentityID = order.ResponsibleID
	if err = identity.CanOperateTx(ctx, tx, responsible, d, identity.ManageProduction); err != nil {
		return MaterialResult{}, err
	}
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_material_reservations WHERE tenant_id=? AND store_id=? AND (id=? OR (order_id=? AND status IN ('active','consumed')))`, a.TenantID, a.StoreID, in.ReservationID, in.OrderID).Scan(&exists); err != nil {
		return MaterialResult{}, err
	}
	if exists != 0 {
		return MaterialResult{}, ErrMaterials
	}
	if _, _, err = normalize(order.Recipe.PublishInput); err != nil || order.PlannedBatches < 1 {
		return MaterialResult{}, ErrMaterials
	}
	items := append([]Ingredient(nil), order.Recipe.Ingredients...)
	for i := range items {
		if order.PlannedBatches > MaxQuantity/items[i].QuantityMilli {
			return MaterialResult{}, ErrMaterials
		}
		items[i].QuantityMilli *= order.PlannedBatches
		if err = validateMaterialUnit(ctx, tx, a, items[i]); err != nil {
			return MaterialResult{}, err
		}
		balance, e := locationBalance(ctx, tx, a, items[i].ProductID, order.LocationID)
		if e != nil {
			return MaterialResult{}, e
		}
		free, e := stockreservation.FreeTx(ctx, tx, a.TenantID, a.StoreID, items[i].ProductID, order.LocationID, balance)
		if e != nil {
			return MaterialResult{}, e
		}
		if free < items[i].QuantityMilli {
			return MaterialResult{}, ErrMaterials
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO production_material_reservations VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.ReservationID, in.OrderID, order.LocationID, "active", a.IdentityID, now, now); err != nil {
		return MaterialResult{}, err
	}
	for _, item := range items {
		if err = one(ctx, tx, `INSERT INTO production_material_items VALUES(?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.ReservationID, item.ProductID, item.Unit, item.QuantityMilli); err != nil {
			return MaterialResult{}, err
		}
	}
	result = MaterialResult{ReservationID: in.ReservationID, OrderID: in.OrderID, Status: "active"}
	if err = materialEvent(ctx, tx, a, d, in.OperationID, "reserve", string(body), now, result); err != nil {
		return MaterialResult{}, err
	}
	return result, tx.Commit()
}

func ChangeMaterials(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in MaterialChangeInput) (MaterialResult, error) {
	if db == nil {
		return MaterialResult{}, errors.New("banco local indisponivel")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.ReservationID) || (in.Action != "release" && in.Action != "consume") || len(in.Reason) < 1 || len(in.Reason) > 255 {
		return MaterialResult{}, ErrInvalid
	}
	body, err := json.Marshal(in)
	if err != nil {
		return MaterialResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return MaterialResult{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return MaterialResult{}, err
	}
	result, repeated, err := materialReplay(ctx, tx, a, d, in.OperationID, in.Action, string(body))
	if err != nil {
		return MaterialResult{}, err
	}
	if repeated {
		return result, tx.Commit()
	}
	reservation, err := materialGetTx(ctx, tx, a, in.ReservationID)
	if err != nil {
		return MaterialResult{}, err
	}
	if reservation.Status != "active" {
		return MaterialResult{}, ErrMaterials
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := "released"
	if in.Action == "consume" {
		var orderStatus, responsibleID string
		if err = tx.QueryRowContext(ctx, `SELECT status,responsible_id FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, reservation.OrderID).Scan(&orderStatus, &responsibleID); err != nil {
			return MaterialResult{}, err
		}
		if orderStatus != "approved" {
			return MaterialResult{}, ErrMaterials
		}
		responsible := a
		responsible.IdentityID = responsibleID
		if err = identity.CanOperateTx(ctx, tx, responsible, d, identity.ManageProduction); err != nil {
			return MaterialResult{}, err
		}
		for _, item := range reservation.Items {
			if err = validateMaterialUnit(ctx, tx, a, item); err != nil {
				return MaterialResult{}, err
			}
			balance, e := locationBalance(ctx, tx, a, item.ProductID, reservation.LocationID)
			if e != nil {
				return MaterialResult{}, e
			}
			// All active reservations, including this one, must fit the physical
			// recorded balance. Consuming this group then releases only its hold.
			if _, e = stockreservation.FreeTx(ctx, tx, a.TenantID, a.StoreID, item.ProductID, reservation.LocationID, balance); e != nil {
				return MaterialResult{}, e
			}
			id, e := localdb.NewID()
			if e != nil {
				return MaterialResult{}, e
			}
			if err = one(ctx, tx, `INSERT INTO stock_movements VALUES(?,?,?,?,?,?,?,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, item.ProductID, reservation.LocationID, -item.QuantityMilli, "production-consume:"+reservation.ID, now); err != nil {
				return MaterialResult{}, err
			}
			if err = one(ctx, tx, `INSERT INTO production_material_movements VALUES(?,?,?,?,?)`, a.TenantID, a.StoreID, reservation.ID, item.ProductID, id); err != nil {
				return MaterialResult{}, err
			}
		}
		status = "consumed"
	}
	if err = one(ctx, tx, `UPDATE production_material_reservations SET status=?,updated_at=? WHERE tenant_id=? AND store_id=? AND id=? AND status='active'`, status, now, a.TenantID, a.StoreID, reservation.ID); err != nil {
		return MaterialResult{}, err
	}
	result = MaterialResult{ReservationID: reservation.ID, OrderID: reservation.OrderID, Status: status}
	if err = materialEvent(ctx, tx, a, d, in.OperationID, in.Action, string(body), now, result); err != nil {
		return MaterialResult{}, err
	}
	return result, tx.Commit()
}

func GetMaterials(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (MaterialReservation, error) {
	if !validID(id) {
		return MaterialReservation{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return MaterialReservation{}, err
	}
	defer tx.Rollback()
	v, err := materialGetTx(ctx, tx, a, id)
	if err != nil {
		return MaterialReservation{}, err
	}
	return v, tx.Commit()
}

// Existing order cancellation cannot erase a live hold or already consumed
// materials. Release first; consumed materials require a future compensation.
func guardMaterialCancellation(ctx context.Context, tx *sql.Tx, a identity.Scope, orderID, target string) error {
	if target != "cancelled" {
		return nil
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM production_material_reservations WHERE tenant_id=? AND store_id=? AND order_id=? AND status IN ('active','consumed')`, a.TenantID, a.StoreID, orderID).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return ErrMaterials
	}
	return nil
}
