package stock

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"strings"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stockreservation"
)

var ErrAvailability = errors.New("saldo ou unidade de estoque inconsistente")
var ErrAvailabilityReference = errors.New("produto ou local inexistente no escopo")

const MaxAvailabilityQuantity int64 = 9007199254740991

type Availability struct {
	ProductID     string `json:"product_id"`
	LocationID    string `json:"location_id"`
	Unit          string `json:"unit"`
	PhysicalMilli int64  `json:"physical_milli"`
	ReservedMilli int64  `json:"reserved_milli"`
	FreeMilli     int64  `json:"free_milli"`
}

func validAvailabilityID(id string) bool {
	return len(id) > 0 && len(id) <= 128 && strings.TrimSpace(id) == id && !strings.ContainsAny(id, "\x00\r\n")
}
func validAvailabilityUnit(unit string) bool {
	switch unit {
	case "unit", "kg", "g", "liter", "ml", "meter":
		return true
	}
	return false
}
func boundedAvailabilityTotal(total *big.Int) (int64, error) {
	if !total.IsInt64() || total.Sign() < 0 || total.Int64() > MaxAvailabilityQuantity {
		return 0, ErrAvailability
	}
	return total.Int64(), nil
}
func physicalAvailabilityTx(ctx context.Context, tx *sql.Tx, a identity.Scope, product, location string) (int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT quantity_milli FROM stock_movements WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, a.TenantID, a.StoreID, product, location)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	total := new(big.Int)
	for rows.Next() {
		var q int64
		if err = rows.Scan(&q); err != nil {
			return 0, err
		}
		total.Add(total, big.NewInt(q))
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	return boundedAvailabilityTotal(total)
}

// All components are read in one authorized transaction. This is an observation,
// never a reservation or an authorization for a later stock write.
func AvailableBalance(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, product, location string) (Availability, error) {
	if !validAvailabilityID(product) || !validAvailabilityID(location) {
		return Availability{}, ErrInvalidOperation
	}
	if db == nil {
		return Availability{}, errors.New("banco local indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Availability{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
		return Availability{}, err
	}
	out, err := availableBalanceTx(ctx, tx, a, product, location)
	if err != nil {
		return Availability{}, err
	}
	return out, tx.Commit()
}

// Caller owns the transaction and must authorize before reading references.
func availableBalanceTx(ctx context.Context, tx *sql.Tx, a identity.Scope, product, location string) (Availability, error) {
	out := Availability{ProductID: product, LocationID: location}
	err := tx.QueryRowContext(ctx, `SELECT p.unit FROM products p JOIN stock_locations l ON l.tenant_id=p.tenant_id WHERE p.tenant_id=? AND p.id=? AND l.store_id=? AND l.id=?`, a.TenantID, product, a.StoreID, location).Scan(&out.Unit)
	if errors.Is(err, sql.ErrNoRows) {
		return Availability{}, ErrAvailabilityReference
	}
	if err != nil {
		return Availability{}, err
	}
	if !validAvailabilityUnit(out.Unit) {
		return Availability{}, ErrAvailability
	}
	// Movements do not snapshot units. At minimum, reject contradictions with
	// preserved production units instead of reinterpreting known history.
	var mismatch bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM production_material_items i JOIN production_material_reservations r ON r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.id=i.reservation_id WHERE i.tenant_id=? AND i.store_id=? AND i.product_id=? AND r.location_id=? AND r.status IN ('active','consumed') AND i.unit<>?)`, a.TenantID, a.StoreID, product, location, out.Unit).Scan(&mismatch); err != nil {
		return Availability{}, err
	}
	if mismatch {
		return Availability{}, ErrAvailability
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM production_results WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=? AND unit<>?)`, a.TenantID, a.StoreID, product, location, out.Unit).Scan(&mismatch); err != nil {
		return Availability{}, err
	}
	if mismatch {
		return Availability{}, ErrAvailability
	}
	out.PhysicalMilli, err = physicalAvailabilityTx(ctx, tx, a, product, location)
	if err != nil {
		return Availability{}, err
	}
	out.ReservedMilli, err = stockreservation.HeldTx(ctx, tx, a.TenantID, a.StoreID, product, location)
	if errors.Is(err, stockreservation.ErrUnavailable) {
		return Availability{}, ErrAvailability
	}
	if err != nil {
		return Availability{}, err
	}
	if out.ReservedMilli < 0 || out.ReservedMilli > out.PhysicalMilli {
		return Availability{}, ErrAvailability
	}
	out.FreeMilli = out.PhysicalMilli - out.ReservedMilli
	return out, nil
}
