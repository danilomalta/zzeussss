package stock

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

var ErrOperationMissing = errors.New("operacao inexistente neste escopo")

type ReceiptInput struct {
	OperationID    string `json:"operation_id"`
	Kind           string `json:"kind"`
	ExpectedUnit   string `json:"expected_unit"`
	ProductID      string `json:"product_id"`
	FromLocationID string `json:"from_location_id"`
	ToLocationID   string `json:"to_location_id"`
	QuantityMilli  int64  `json:"quantity_milli"`
	Reason         string `json:"reason"`
}
type OperationReceipt struct {
	Input      ReceiptInput `json:"input"`
	ActorID    string       `json:"actor_id"`
	DeviceID   string       `json:"device_id"`
	OccurredAt string       `json:"occurred_at"`
	Unit       string       `json:"unit"`
}
type OperationHistory struct {
	ProductID  string             `json:"product_id"`
	LocationID string             `json:"location_id"`
	Items      []OperationReceipt `json:"items"`
	TotalCount int64              `json:"total_count"`
	Offset     int64              `json:"offset"`
	Limit      int                `json:"limit"`
	HasMore    bool               `json:"has_more"`
}

const operationColumns = `o.id,o.kind,o.product_id,COALESCE(o.from_location_id,''),COALESCE(o.to_location_id,''),o.quantity_milli,o.reason,o.actor_identity_id,o.device_id,o.occurred_at,p.unit`

type operationScanner interface{ Scan(...any) error }

func scanOperation(row operationScanner) (OperationReceipt, error) {
	var v OperationReceipt
	err := row.Scan(&v.Input.OperationID, &v.Input.Kind, &v.Input.ProductID, &v.Input.FromLocationID, &v.Input.ToLocationID, &v.Input.QuantityMilli, &v.Input.Reason, &v.ActorID, &v.DeviceID, &v.OccurredAt, &v.Unit)
	if err != nil {
		return v, err
	}
	v.Input.ExpectedUnit = v.Unit
	i := Input(v.Input)
	if !validAvailabilityID(i.OperationID) || !validAvailabilityID(i.ProductID) || !validAvailabilityID(v.ActorID) || !validAvailabilityID(v.DeviceID) || !validAvailabilityUnit(v.Unit) || !validLocations(i) || i.QuantityMilli <= 0 || i.QuantityMilli > MaxAvailabilityQuantity || len(i.Reason) == 0 || len(i.Reason) > 255 || strings.TrimSpace(i.Reason) != i.Reason {
		return v, ErrAvailability
	}
	if i.FromLocationID != "" && !validAvailabilityID(i.FromLocationID) || i.ToLocationID != "" && !validAvailabilityID(i.ToLocationID) {
		return v, ErrAvailability
	}
	if _, err = time.Parse(time.RFC3339Nano, v.OccurredAt); err != nil {
		return v, ErrAvailability
	}
	return v, nil
}

// Receipt is scoped to the original operator and device, independent of the current balance.
func Receipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, operation string) (OperationReceipt, error) {
	if !validAvailabilityID(operation) {
		return OperationReceipt{}, ErrInvalidOperation
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return OperationReceipt{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
		return OperationReceipt{}, err
	}
	v, err := scanOperation(tx.QueryRowContext(ctx, `SELECT `+operationColumns+` FROM stock_operations o JOIN products p ON p.tenant_id=o.tenant_id AND p.id=o.product_id WHERE o.id=? AND o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.actor_identity_id=?`, operation, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID))
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrOperationMissing
	}
	if err != nil {
		return v, err
	}
	return v, tx.Commit()
}

// History includes all operators of the current company/store, never other stores.
// It is not a complete ledger: sales, purchases and production have their own records.
func History(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, product, location string, offset int64) (OperationHistory, error) {
	out := OperationHistory{ProductID: product, LocationID: location, Items: []OperationReceipt{}, Offset: offset, Limit: 50}
	if !validAvailabilityID(product) || !validAvailabilityID(location) || offset < 0 || offset > MaxAvailabilityQuantity {
		return out, ErrInvalidOperation
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
		return out, err
	}
	var found int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM products p JOIN stock_locations l ON l.tenant_id=p.tenant_id WHERE p.tenant_id=? AND p.id=? AND l.store_id=? AND l.id=?`, a.TenantID, product, a.StoreID, location).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrOperationMissing
	}
	if err != nil {
		return out, err
	}
	clause := ` FROM stock_operations o JOIN products p ON p.tenant_id=o.tenant_id AND p.id=o.product_id WHERE o.tenant_id=? AND o.store_id=? AND o.product_id=? AND (o.from_location_id=? OR o.to_location_id=?)`
	args := []any{a.TenantID, a.StoreID, product, location, location}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*)`+clause, args...).Scan(&out.TotalCount); err != nil {
		return out, err
	}
	if out.TotalCount > MaxAvailabilityQuantity {
		return out, ErrAvailability
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+operationColumns+clause+` ORDER BY o.occurred_at DESC,o.id DESC LIMIT 50 OFFSET ?`, append(args, offset)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		v, e := scanOperation(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.HasMore = offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-offset
	return out, tx.Commit()
}
