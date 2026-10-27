package production

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"time"

	"titansystem-backend/internal/localdb/identity"
)

// These are current-catalog material estimates, NEVER historical actual cost,
// full manufacturing cost, profit, or a posting to finance.
type ExactCost struct {
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}
type MaterialCostLine struct {
	ProductID          string    `json:"product_id"`
	Unit               string    `json:"unit"`
	QuantityMilli      int64     `json:"quantity_milli"`
	CatalogCostCents   int64     `json:"catalog_cost_cents"`
	Cost               ExactCost `json:"cost_cents_exact"`
	RequiresCostReview bool      `json:"requires_cost_review"`
}
type MaterialCostEstimate struct {
	Source              string             `json:"source"`
	ValuedAt            string             `json:"valued_at"`
	VersionID           string             `json:"version_id"`
	OrderID             string             `json:"order_id,omitempty"`
	Batches             int64              `json:"batches"`
	OutputUnit          string             `json:"output_unit"`
	OutputQuantityMilli int64              `json:"output_quantity_milli"`
	Lines               []MaterialCostLine `json:"lines"`
	MaterialCost        ExactCost          `json:"material_cost_cents_exact"`
	MaterialCostCents   int64              `json:"material_cost_cents"`
	MaterialUnitCost    ExactCost          `json:"material_unit_cost_cents_exact"`
	Rounding            string             `json:"rounding"`
	LaborCostCents      *int64             `json:"labor_cost_cents"`
	OverheadCostCents   *int64             `json:"overhead_cost_cents"`
	IsComplete          bool               `json:"is_complete"`
}

func exactCost(r *big.Rat) ExactCost { return ExactCost{r.Num().String(), r.Denom().String()} }
func roundedMaterialCost(r *big.Rat) (int64, error) {
	if r.Sign() < 0 {
		return 0, ErrConflict
	}
	doubled := new(big.Int).Mul(r.Num(), big.NewInt(2))
	doubled.Add(doubled, r.Denom())
	denom := new(big.Int).Mul(r.Denom(), big.NewInt(2))
	n := new(big.Int).Quo(doubled, denom)
	if !n.IsInt64() || n.Cmp(big.NewInt(MaxQuantity)) > 0 {
		return 0, ErrInvalid
	}
	return n.Int64(), nil
}
func RecipeMaterialCost(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, batches int64) (MaterialCostEstimate, error) {
	if !validID(id) || batches < 1 || batches > MaxQuantity {
		return MaterialCostEstimate{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return MaterialCostEstimate{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return MaterialCostEstimate{}, err
	}
	v, err := scanVersion(tx.QueryRowContext(ctx, `SELECT request_json,revision,created_at,actor_id FROM production_recipe_versions WHERE tenant_id=? AND store_id=? AND version_id=?`, a.TenantID, a.StoreID, id))
	if err != nil {
		return MaterialCostEstimate{}, err
	}
	if v.VersionID != id {
		return MaterialCostEstimate{}, ErrConflict
	}
	r, err := materialCostTx(ctx, tx, a, v, batches)
	if err != nil {
		return r, err
	}
	return r, tx.Commit()
}
func OrderMaterialCost(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (MaterialCostEstimate, error) {
	if !validID(id) {
		return MaterialCostEstimate{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return MaterialCostEstimate{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return MaterialCostEstimate{}, err
	}
	order, err := scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id))
	if err != nil {
		return MaterialCostEstimate{}, err
	}
	if order.Recipe.VersionID != order.VersionID {
		return MaterialCostEstimate{}, ErrConflict
	}
	r, err := materialCostTx(ctx, tx, a, order.Recipe, order.PlannedBatches)
	if err != nil {
		return r, err
	}
	if r.OutputQuantityMilli != order.PlannedOutputMilli {
		return r, ErrConflict
	}
	r.OrderID = id
	return r, tx.Commit()
}
func materialCostTx(ctx context.Context, tx *sql.Tx, a identity.Scope, v Version, batches int64) (MaterialCostEstimate, error) {
	r := MaterialCostEstimate{Source: "current_catalog_material_estimate", ValuedAt: time.Now().UTC().Format(time.RFC3339Nano), VersionID: v.VersionID, Batches: batches, OutputUnit: v.OutputUnit, Lines: []MaterialCostLine{}, Rounding: "half_up_on_aggregate"}
	normalized, _, err := normalize(v.PublishInput)
	if err != nil || batches < 1 || batches > MaxQuantity {
		return r, ErrConflict
	}
	output := new(big.Int).Mul(big.NewInt(v.YieldMilli), big.NewInt(batches))
	if !output.IsInt64() || output.Sign() <= 0 || output.Cmp(big.NewInt(MaxQuantity)) > 0 {
		return r, ErrInvalid
	}
	r.OutputQuantityMilli = output.Int64()
	var outputUnit string
	if err = tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, v.OutputProductID).Scan(&outputUnit); err != nil {
		return r, err
	}
	if outputUnit != v.OutputUnit {
		return r, ErrConflict
	}
	total := new(big.Rat)
	for _, in := range normalized.Ingredients {
		var unit string
		var cost int64
		err = tx.QueryRowContext(ctx, `SELECT unit,cost_cents FROM products WHERE tenant_id=? AND id=?`, a.TenantID, in.ProductID).Scan(&unit, &cost)
		if errors.Is(err, sql.ErrNoRows) {
			return r, ErrConflict
		}
		if err != nil {
			return r, err
		}
		if unit != in.Unit || cost < 0 || cost > MaxQuantity {
			return r, ErrConflict
		}
		quantity := new(big.Int).Mul(big.NewInt(in.QuantityMilli), big.NewInt(batches))
		if !quantity.IsInt64() || quantity.Cmp(big.NewInt(MaxQuantity)) > 0 {
			return r, ErrInvalid
		}
		value := new(big.Rat).SetFrac(new(big.Int).Mul(quantity, big.NewInt(cost)), big.NewInt(1000))
		total.Add(total, value)
		r.Lines = append(r.Lines, MaterialCostLine{ProductID: in.ProductID, Unit: unit, QuantityMilli: quantity.Int64(), CatalogCostCents: cost, Cost: exactCost(value), RequiresCostReview: cost == 0})
	}
	rounded, err := roundedMaterialCost(total)
	if err != nil {
		return r, err
	}
	r.MaterialCostCents = rounded
	r.MaterialCost = exactCost(total)
	unitCost := new(big.Rat).Mul(total, new(big.Rat).SetFrac(big.NewInt(1000), output))
	r.MaterialUnitCost = exactCost(unitCost)
	return r, nil
}
