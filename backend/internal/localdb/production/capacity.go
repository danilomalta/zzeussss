package production

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"time"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stockreservation"
)

var ErrCapacity = errors.New("saldo, unidade ou capacidade fora dos limites")

type CapacityInput struct {
	LocationID string   `json:"location_id"`
	VersionIDs []string `json:"version_ids"`
}

type MaterialCapacity struct {
	ProductID             string `json:"product_id"`
	RecipeUnit            string `json:"recipe_unit"`
	StockUnit             string `json:"stock_unit"`
	RequiredMilli         int64  `json:"required_milli"`
	StockMilli            int64  `json:"stock_milli"`
	ConversionNumerator   int64  `json:"conversion_numerator"`
	ConversionDenominator int64  `json:"conversion_denominator"`
	PossibleBatches       int64  `json:"possible_batches"`
	Limiting              bool   `json:"limiting"`
}

type Capacity struct {
	RecipeID            string             `json:"recipe_id"`
	VersionID           string             `json:"version_id"`
	Revision            int64              `json:"revision"`
	OutputProductID     string             `json:"output_product_id"`
	OutputUnit          string             `json:"output_unit"`
	YieldPerBatchMilli  int64              `json:"yield_per_batch_milli"`
	PossibleBatches     int64              `json:"possible_batches"`
	PossibleOutputMilli int64              `json:"possible_output_milli"`
	LimitingProductIDs  []string           `json:"limiting_product_ids"`
	Materials           []MaterialCapacity `json:"materials"`
}

type CapacityResult struct {
	LocationID                 string     `json:"location_id"`
	MeasuredAt                 string     `json:"measured_at"`
	Basis                      string     `json:"basis"`
	AlternativesIndependent    bool       `json:"alternatives_independent"`
	SimultaneousTotalAvailable bool       `json:"simultaneous_total_available"`
	Alternatives               []Capacity `json:"alternatives"`
}

// UnitRatio converts quantities from stockUnit to recipeUnit as an exact
// rational. Both quantities use thousandths, so the scale cancels. There is
// no conversion between dimensions and no assumed density or packaging.
func UnitRatio(stockUnit, recipeUnit string) (int64, int64, error) {
	if !validUnit(stockUnit) || !validUnit(recipeUnit) {
		return 0, 0, ErrCapacity
	}
	if stockUnit == recipeUnit {
		return 1, 1, nil
	}
	switch {
	case stockUnit == "kg" && recipeUnit == "g", stockUnit == "liter" && recipeUnit == "ml":
		return 1000, 1, nil
	case stockUnit == "g" && recipeUnit == "kg", stockUnit == "ml" && recipeUnit == "liter":
		return 1, 1000, nil
	}
	return 0, 0, ErrCapacity
}

// batchCount floors only at whole reference batches, without first rounding a
// converted balance. Arbitrary precision intermediates avoid multiplication
// overflow; API results remain bounded to exact JS-safe integers.
func batchCount(stockMilli, requiredMilli, numerator, denominator int64) (int64, error) {
	if stockMilli < 0 || stockMilli > MaxQuantity || requiredMilli < 1 || requiredMilli > MaxQuantity || numerator < 1 || denominator < 1 {
		return 0, ErrCapacity
	}
	a := new(big.Int).Mul(big.NewInt(stockMilli), big.NewInt(numerator))
	b := new(big.Int).Mul(big.NewInt(requiredMilli), big.NewInt(denominator))
	q := new(big.Int).Quo(a, b)
	if !q.IsInt64() || q.Int64() > MaxQuantity {
		return 0, ErrCapacity
	}
	return q.Int64(), nil
}

// locationBalance avoids SQLite SUM overflow, including large cancelling
// movements. Negative final balances are inconsistencies, not free material.
func locationBalance(ctx context.Context, tx *sql.Tx, actor identity.Scope, productID, locationID string) (int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT quantity_milli FROM stock_movements WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, actor.TenantID, actor.StoreID, productID, locationID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	total := new(big.Int)
	for rows.Next() {
		var quantity int64
		if err = rows.Scan(&quantity); err != nil {
			return 0, err
		}
		total.Add(total, big.NewInt(quantity))
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	if !total.IsInt64() || total.Sign() < 0 || total.Int64() > MaxQuantity {
		return 0, ErrCapacity
	}
	return total.Int64(), nil
}

func capacityTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, locationID string, version Version) (Capacity, error) {
	// Validate stored snapshots before dividing; corrupt or malformed historic
	// content must not yield a plausible capacity.
	if _, _, err := normalize(version.PublishInput); err != nil || version.Revision < 1 || version.Revision > MaxRevision {
		return Capacity{}, ErrCapacity
	}
	out := Capacity{RecipeID: version.RecipeID, VersionID: version.VersionID, Revision: version.Revision, OutputProductID: version.OutputProductID, OutputUnit: version.OutputUnit, YieldPerBatchMilli: version.YieldMilli, PossibleBatches: MaxQuantity, Materials: []MaterialCapacity{}, LimitingProductIDs: []string{}}
	for _, item := range version.Ingredients {
		var unit string
		if err := tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, actor.TenantID, item.ProductID).Scan(&unit); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Capacity{}, ErrCapacity
			}
			return Capacity{}, err
		}
		// Stock movements do not snapshot their unit in this schema. If a
		// product's unit changed since publication, the physical interpretation
		// of old movements is unknown; a compatible dimension alone is not
		// enough to safely reinterpret that historic balance.
		if unit != item.Unit {
			return Capacity{}, ErrCapacity
		}
		n, d, err := UnitRatio(unit, item.Unit)
		if err != nil {
			return Capacity{}, err
		}
		balance, err := locationBalance(ctx, tx, actor, item.ProductID, locationID)
		if err != nil {
			return Capacity{}, err
		}
		balance, err = stockreservation.FreeTx(ctx, tx, actor.TenantID, actor.StoreID, item.ProductID, locationID, balance)
		if err != nil {
			return Capacity{}, ErrCapacity
		}
		batches, err := batchCount(balance, item.QuantityMilli, n, d)
		if err != nil {
			return Capacity{}, err
		}
		out.Materials = append(out.Materials, MaterialCapacity{ProductID: item.ProductID, RecipeUnit: item.Unit, StockUnit: unit, RequiredMilli: item.QuantityMilli, StockMilli: balance, ConversionNumerator: n, ConversionDenominator: d, PossibleBatches: batches})
		if batches < out.PossibleBatches {
			out.PossibleBatches = batches
		}
	}
	for i := range out.Materials {
		if out.Materials[i].PossibleBatches == out.PossibleBatches {
			out.Materials[i].Limiting = true
			out.LimitingProductIDs = append(out.LimitingProductIDs, out.Materials[i].ProductID)
		}
	}
	if out.PossibleBatches > MaxQuantity/version.YieldMilli {
		return Capacity{}, ErrCapacity
	}
	out.PossibleOutputMilli = out.PossibleBatches * version.YieldMilli
	return out, nil
}

// Capacities evaluates every requested version against the SAME transactional
// stock snapshot. Alternatives share stock and are never summed or reserved.
// Historical reads use P02's current human/device permission, even after
// contract expiry. No entity, stock, license clock, audit or outbox is written.
func Capacities(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in CapacityInput) (CapacityResult, error) {
	if !validID(in.LocationID) || len(in.VersionIDs) < 1 || len(in.VersionIDs) > 20 {
		return CapacityResult{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range in.VersionIDs {
		if !validID(id) || seen[id] {
			return CapacityResult{}, ErrInvalid
		}
		seen[id] = true
	}
	tx, err := readTx(ctx, db, actor, device)
	if err != nil {
		return CapacityResult{}, err
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM stock_locations WHERE tenant_id=? AND store_id=? AND id=?`, actor.TenantID, actor.StoreID, in.LocationID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return CapacityResult{}, ErrNotFound
	}
	if err != nil {
		return CapacityResult{}, err
	}
	out := CapacityResult{LocationID: in.LocationID, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano), Basis: "local_available_balance_after_reservations", AlternativesIndependent: true, SimultaneousTotalAvailable: false, Alternatives: []Capacity{}}
	for _, id := range in.VersionIDs {
		version, e := scanVersion(tx.QueryRowContext(ctx, `SELECT request_json,revision,created_at,actor_id FROM production_recipe_versions WHERE tenant_id=? AND store_id=? AND version_id=?`, actor.TenantID, actor.StoreID, id))
		if e != nil {
			return CapacityResult{}, e
		}
		// Guard relational IDs as well as the stored JSON request.
		if version.VersionID != id {
			return CapacityResult{}, ErrCapacity
		}
		result, e := capacityTx(ctx, tx, actor, in.LocationID, version)
		if e != nil {
			return CapacityResult{}, e
		}
		out.Alternatives = append(out.Alternatives, result)
	}
	if err = tx.Commit(); err != nil {
		return CapacityResult{}, err
	}
	return out, nil
}
