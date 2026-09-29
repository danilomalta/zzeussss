// Package inventory registra a contagem física por localização e aparelho.
package inventory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrInvalid  = errors.New("contagem de estoque inválida")
	ErrConflict = errors.New("ID de contagem reutilizado com outros dados")
)

type Input struct {
	OperationID  string `json:"operation_id"`
	ProductID    string `json:"product_id"`
	LocationID   string `json:"location_id"`
	CountedMilli int64  `json:"counted_milli"`
}

type Result struct {
	OperationID     string
	PreviousMilli   int64
	DifferenceMilli int64
	Repeated        bool
}

// Count registra o número físico, calcula a diferença para o saldo local e
// grava ajuste + outbox no mesmo commit. IDs exigem sessões verificadas.
func Count(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in Input) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponível")
	}
	in.OperationID = strings.TrimSpace(in.OperationID)
	in.ProductID = strings.TrimSpace(in.ProductID)
	in.LocationID = strings.TrimSpace(in.LocationID)
	if in.OperationID == "" || in.ProductID == "" || in.LocationID == "" || in.CountedMilli < 0 {
		return Result{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStock); err != nil {
		return Result{}, err
	}
	var previous, counted, difference int64
	var store, subject, location, by string
	err = tx.QueryRowContext(ctx, `SELECT store_id,product_id,location_id,actor_identity_id,counted_milli,previous_milli,difference_milli
		FROM inventory_counts WHERE tenant_id=? AND device_id=? AND operation_id=?`, actor.TenantID, device.DeviceID, in.OperationID).
		Scan(&store, &subject, &location, &by, &counted, &previous, &difference)
	if err == nil {
		if store != actor.StoreID || by != actor.IdentityID || subject != in.ProductID || location != in.LocationID || counted != in.CountedMilli {
			return Result{}, ErrConflict
		}
		if err = tx.Commit(); err != nil {
			return Result{}, err
		}
		return Result{in.OperationID, previous, difference, true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	var found int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM products p JOIN stock_locations l ON l.tenant_id=p.tenant_id
		WHERE p.tenant_id=? AND p.id=? AND l.store_id=? AND l.id=?`, actor.TenantID, in.ProductID, actor.StoreID, in.LocationID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrInvalid
	}
	if err != nil {
		return Result{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements
		WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, actor.TenantID, actor.StoreID, in.ProductID, in.LocationID).Scan(&previous); err != nil {
		return Result{}, err
	}
	if previous < 0 {
		return Result{}, ErrInvalid
	}
	difference = in.CountedMilli - previous
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var movementID any
	if difference != 0 {
		id, idErr := localdb.NewID()
		if idErr != nil {
			return Result{}, idErr
		}
		movementID = id
		_, err = tx.ExecContext(ctx, `INSERT INTO stock_movements
			(id,tenant_id,store_id,device_id,product_id,location_id,quantity_milli,reason,created_at)
			VALUES (?,?,?,?,?,?,?,?,?)`, id, actor.TenantID, actor.StoreID, device.DeviceID, in.ProductID, in.LocationID, difference, "inventário:"+in.OperationID, now)
		if err != nil {
			return Result{}, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO inventory_counts VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, actor.IdentityID, in.ProductID, in.LocationID, in.CountedMilli, previous, difference, movementID, now)
	if err != nil {
		return Result{}, err
	}
	payload, err := json.Marshal(struct {
		TenantID        string `json:"tenant_id"`
		StoreID         string `json:"store_id"`
		DeviceID        string `json:"device_id"`
		ActorID         string `json:"actor_id"`
		Input           Input  `json:"input"`
		PreviousMilli   int64  `json:"previous_milli"`
		DifferenceMilli int64  `json:"difference_milli"`
	}{actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, in, previous, difference})
	if err != nil {
		return Result{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return Result{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
		VALUES (?,?,?,?,?,?,'stock.count',1,?,?)`, eventID, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.ProductID, string(payload), now)
	if err != nil {
		return Result{}, fmt.Errorf("enfileirar contagem: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{in.OperationID, previous, difference, false}, nil
}
