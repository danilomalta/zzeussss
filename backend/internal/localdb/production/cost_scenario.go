package production

import (
	"context"
	"database/sql"
	"math/big"
	"titansystem-backend/internal/localdb/identity"
)

type LaborScenario struct {
	Minutes         int64 `json:"minutes"`
	Workers         int64 `json:"workers"`
	HourlyCostCents int64 `json:"hourly_cost_cents"`
}
type CostScenarioInput struct {
	Batches        int64          `json:"batches"`
	Labor          *LaborScenario `json:"labor"`
	OverheadCents  *int64         `json:"overhead_cents"`
	SalePriceCents *int64         `json:"sale_price_cents"`
}
type CostScenario struct {
	Materials                      MaterialCostEstimate `json:"materials"`
	Basis                          string               `json:"basis"`
	Labor                          *ExactCost           `json:"labor_cents_exact"`
	OverheadCents                  *int64               `json:"overhead_cents"`
	KnownCost                      ExactCost            `json:"known_cost_cents_exact"`
	KnownCostCents                 int64                `json:"known_cost_cents"`
	KnownUnitCost                  ExactCost            `json:"known_unit_cost_cents_exact"`
	Revenue                        *ExactCost           `json:"scenario_revenue_cents_exact"`
	Contribution                   *ExactCost           `json:"scenario_contribution_cents_exact"`
	MarginBasisPoints              *int64               `json:"scenario_margin_basis_points"`
	AllRequestedComponentsProvided bool                 `json:"all_requested_components_provided"`
	HistoricalCost                 bool                 `json:"historical_cost"`
	Profit                         bool                 `json:"profit"`
}

// Scenario inputs describe the entire requested run, not a per-batch labor or
// overhead charge. Null is unknown; explicit zero means the caller supplied zero.
func RecipeCostScenario(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, in CostScenarioInput) (CostScenario, error) {
	var out CostScenario
	if db == nil || !validID(id) || in.Batches < 1 || in.Batches > MaxQuantity {
		return out, ErrInvalid
	}
	for _, v := range []*int64{in.OverheadCents, in.SalePriceCents} {
		if v != nil && (*v < 0 || *v > MaxQuantity) {
			return out, ErrInvalid
		}
	}
	if in.Labor != nil {
		l := in.Labor
		if l.Minutes < 0 || l.Minutes > 525600 || l.Workers < 1 || l.Workers > 10000 || l.HourlyCostCents < 0 || l.HourlyCostCents > MaxQuantity {
			return out, ErrInvalid
		}
	}
	m, err := RecipeMaterialCost(ctx, db, a, d, id, in.Batches)
	if err != nil {
		return out, err
	}
	return calculateScenario(m, in)
}
func calculateScenario(m MaterialCostEstimate, in CostScenarioInput) (CostScenario, error) {
	out := CostScenario{Materials: m, Basis: "current_materials_and_user_supplied_run_assumptions", OverheadCents: in.OverheadCents, AllRequestedComponentsProvided: in.Labor != nil && in.OverheadCents != nil}
	total, ok := new(big.Rat).SetString(m.MaterialCost.Numerator + "/" + m.MaterialCost.Denominator)
	if !ok || total.Sign() < 0 || m.OutputQuantityMilli < 1 {
		return out, ErrConflict
	}
	if in.Labor != nil {
		l := in.Labor
		n := new(big.Int).Mul(big.NewInt(l.Minutes), big.NewInt(l.Workers))
		n.Mul(n, big.NewInt(l.HourlyCostCents))
		value := new(big.Rat).SetFrac(n, big.NewInt(60))
		x := exactCost(value)
		out.Labor = &x
		total.Add(total, value)
	}
	if in.OverheadCents != nil {
		total.Add(total, new(big.Rat).SetInt64(*in.OverheadCents))
	}
	out.KnownCost = exactCost(total)
	rounded, err := roundedMaterialCost(total)
	if err != nil {
		return out, err
	}
	out.KnownCostCents = rounded
	out.KnownUnitCost = exactCost(new(big.Rat).Mul(total, new(big.Rat).SetFrac(big.NewInt(1000), big.NewInt(m.OutputQuantityMilli))))
	if in.SalePriceCents != nil {
		revenue := new(big.Rat).SetFrac(new(big.Int).Mul(big.NewInt(*in.SalePriceCents), big.NewInt(m.OutputQuantityMilli)), big.NewInt(1000))
		if _, err = roundedMaterialCost(revenue); err != nil {
			return out, err
		}
		x := exactCost(revenue)
		out.Revenue = &x
		// Missing components must never produce a plausible full contribution.
		if out.AllRequestedComponentsProvided {
			c := new(big.Rat).Sub(revenue, total)
			v := exactCost(c)
			out.Contribution = &v
			if revenue.Sign() > 0 {
				ratio := new(big.Rat).Mul(new(big.Rat).Quo(c, revenue), big.NewRat(10000, 1))
				n := new(big.Int).Quo(ratio.Num(), ratio.Denom())
				if n.IsInt64() && n.Cmp(big.NewInt(-MaxQuantity)) >= 0 && n.Cmp(big.NewInt(MaxQuantity)) <= 0 {
					value := n.Int64()
					out.MarginBasisPoints = &value
				}
			}
		}
	}
	return out, nil
}
