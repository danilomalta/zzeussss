package stock

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stockreservation"
)

var (
	ErrInvalidOperation  = errors.New("movimentação inválida")
	ErrInsufficientStock = errors.New("saldo local insuficiente")
	ErrOperationConflict = errors.New("ID de operação reutilizado com dados diferentes")
	ErrStockOverflow     = errors.New("saldo excede limite inteiro local")
)

type Input struct {
	OperationID    string `json:"operation_id"`
	Kind           string `json:"kind"`
	ProductID      string `json:"product_id"`
	FromLocationID string `json:"from_location_id,omitempty"`
	ToLocationID   string `json:"to_location_id,omitempty"`
	QuantityMilli  int64  `json:"quantity_milli"`
	Reason         string `json:"reason"`
}

type Result struct {
	OperationID string `json:"operation_id"`
	Repeated    bool   `json:"repeated"`
}

type stockAuthorization func(context.Context, *sql.Tx, identity.Scope, identity.DeviceContext) error

// Record preserves the legacy permission-only path. New routes must use
// RecordWithContract, which also validates the inventory entitlement.
func Record(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in Input) (Result, error) {
	return record(ctx, db, actor, device, in, func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStock)
	})
}

func record(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in Input, authorize stockAuthorization) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponível")
	}
	in.OperationID = strings.TrimSpace(in.OperationID)
	in.ProductID = strings.TrimSpace(in.ProductID)
	in.Reason = strings.TrimSpace(in.Reason)
	if len(in.OperationID) > 128 || len(in.ProductID) > 128 || len(in.FromLocationID) > 128 || len(in.ToLocationID) > 128 {
		return Result{}, ErrInvalidOperation
	}
	if in.OperationID == "" || in.ProductID == "" || in.QuantityMilli <= 0 ||
		in.Reason == "" || len(in.Reason) > 255 || !validLocations(in) {
		return Result{}, ErrInvalidOperation
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err := authorize(ctx, tx, actor, device); err != nil {
		return Result{}, err
	}
	var existing struct {
		tenant, store, device, actor, kind, product, reason string
		from, to                                            sql.NullString
		quantity                                            int64
	}
	err = tx.QueryRowContext(ctx, `SELECT tenant_id, store_id, device_id, actor_identity_id,
		kind, product_id, from_location_id, to_location_id, quantity_milli, reason
		FROM stock_operations WHERE id = ?`, in.OperationID).
		Scan(&existing.tenant, &existing.store, &existing.device, &existing.actor, &existing.kind,
			&existing.product, &existing.from, &existing.to, &existing.quantity, &existing.reason)
	if err == nil {
		if existing.tenant != actor.TenantID || existing.store != actor.StoreID ||
			existing.device != device.DeviceID || existing.actor != actor.IdentityID ||
			existing.kind != in.Kind || existing.product != in.ProductID ||
			existing.from.String != in.FromLocationID || existing.to.String != in.ToLocationID ||
			existing.quantity != in.QuantityMilli || existing.reason != in.Reason {
			return Result{}, ErrOperationConflict
		}
		if err := tx.Commit(); err != nil {
			return Result{}, err
		}
		return Result{OperationID: in.OperationID, Repeated: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	if in.FromLocationID != "" {
		balance, err := balanceTx(ctx, tx, actor, in.ProductID, in.FromLocationID)
		if err != nil {
			return Result{}, err
		}
		free, err := stockreservation.FreeTx(ctx, tx, actor.TenantID, actor.StoreID, in.ProductID, in.FromLocationID, balance)
		if errors.Is(err, stockreservation.ErrUnavailable) {
			return Result{}, ErrInsufficientStock
		}
		if err != nil {
			return Result{}, err
		}
		if free < in.QuantityMilli {
			return Result{}, ErrInsufficientStock
		}
	}
	if in.ToLocationID != "" {
		balance, err := balanceTx(ctx, tx, actor, in.ProductID, in.ToLocationID)
		if err != nil {
			return Result{}, err
		}
		if balance > math.MaxInt64-in.QuantityMilli {
			return Result{}, ErrStockOverflow
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO stock_operations
		(id, tenant_id, store_id, device_id, actor_identity_id, kind, product_id,
		 from_location_id, to_location_id, quantity_milli, reason, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.OperationID, actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID,
		in.Kind, in.ProductID, nullable(in.FromLocationID), nullable(in.ToLocationID), in.QuantityMilli, in.Reason, now)
	if err != nil {
		return Result{}, fmt.Errorf("registrar operação de estoque: %w", err)
	}
	if in.FromLocationID != "" {
		if err := addMovement(ctx, tx, actor, device, in, in.FromLocationID, -in.QuantityMilli, now); err != nil {
			return Result{}, err
		}
	}
	if in.ToLocationID != "" {
		if err := addMovement(ctx, tx, actor, device, in, in.ToLocationID, in.QuantityMilli, now); err != nil {
			return Result{}, err
		}
	}
	payload, err := json.Marshal(struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		DeviceID string `json:"device_id"`
		ActorID  string `json:"actor_id"`
		Input
	}{device.TenantID, device.StoreID, device.DeviceID, actor.IdentityID, in})
	if err != nil {
		return Result{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return Result{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox
		(event_id, tenant_id, store_id, device_id, operation_id, aggregate_id,
		 event_type, schema_version, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'stock.operation', 1, ?, ?)`,
		eventID, actor.TenantID, actor.StoreID, device.DeviceID,
		in.OperationID, in.ProductID, string(payload), now)
	if err != nil {
		return Result{}, fmt.Errorf("registrar outbox: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{OperationID: in.OperationID}, nil
}

func Balance(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, productID, locationID string) (int64, error) {
	if db == nil {
		return 0, errors.New("banco local indisponível")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStock); err != nil {
		return 0, err
	}
	balance, err := balanceTx(ctx, tx, actor, productID, locationID)
	if err != nil {
		return 0, err
	}
	return balance, tx.Commit()
}

func balanceTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, productID, locationID string) (int64, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM products p JOIN stock_locations l ON l.tenant_id = p.tenant_id
		WHERE p.tenant_id = ? AND p.id = ? AND l.store_id = ? AND l.id = ?`,
		actor.TenantID, productID, actor.StoreID, locationID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidOperation
	}
	if err != nil {
		return 0, err
	}
	var balance int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity_milli), 0) FROM stock_movements
		WHERE tenant_id = ? AND store_id = ? AND product_id = ? AND location_id = ?`,
		actor.TenantID, actor.StoreID, productID, locationID).Scan(&balance)
	return balance, err
}

func addMovement(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext,
	in Input, locationID string, quantity int64, now string) error {
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO stock_movements
		(id, tenant_id, store_id, device_id, product_id, location_id, quantity_milli, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, actor.TenantID, actor.StoreID, device.DeviceID, in.ProductID, locationID, quantity, in.Reason, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO stock_operation_movements VALUES (?, ?)", in.OperationID, id)
	return err
}

func validLocations(in Input) bool {
	switch in.Kind {
	case "entry":
		return in.FromLocationID == "" && in.ToLocationID != ""
	case "loss":
		return in.FromLocationID != "" && in.ToLocationID == ""
	case "transfer":
		return in.FromLocationID != "" && in.ToLocationID != "" && in.FromLocationID != in.ToLocationID
	default:
		return false
	}
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
