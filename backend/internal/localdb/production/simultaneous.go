package production

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"sort"
	"time"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stockreservation"
)

// A simultaneous plan shares each ingredient once across all requested recipes.
// It never creates an order or a reservation and carries no later write authority.
type PlanRecipe struct {
	VersionID string `json:"version_id"`
	Batches   int64  `json:"batches"`
}
type SimultaneousInput struct {
	LocationID string       `json:"location_id"`
	Recipes    []PlanRecipe `json:"recipes"`
}
type PlannedOutput struct {
	VersionID   string `json:"version_id"`
	ProductID   string `json:"product_id"`
	Unit        string `json:"unit"`
	Batches     int64  `json:"batches"`
	OutputMilli int64  `json:"output_milli"`
}
type SharedMaterial struct {
	ProductID      string `json:"product_id"`
	Unit           string `json:"unit"`
	RequiredMilli  int64  `json:"required_milli"`
	PhysicalMilli  int64  `json:"physical_milli"`
	ReservedMilli  int64  `json:"reserved_milli"`
	FreeMilli      int64  `json:"free_milli"`
	ShortfallMilli int64  `json:"shortfall_milli"`
	RemainingMilli int64  `json:"remaining_milli"`
}
type SimultaneousPlan struct {
	LocationID string           `json:"location_id"`
	MeasuredAt string           `json:"measured_at"`
	Basis      string           `json:"basis"`
	Feasible   bool             `json:"feasible"`
	Reserved   bool             `json:"reserved"`
	Outputs    []PlannedOutput  `json:"outputs"`
	Materials  []SharedMaterial `json:"materials"`
}

func boundedPositiveProduct(a, b int64) (int64, error) {
	if a < 1 || b < 1 || a > MaxQuantity || b > MaxQuantity {
		return 0, ErrInvalid
	}
	x := new(big.Int).Mul(big.NewInt(a), big.NewInt(b))
	if !x.IsInt64() || x.Cmp(big.NewInt(MaxQuantity)) > 0 {
		return 0, ErrInvalid
	}
	return x.Int64(), nil
}
func Simultaneous(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in SimultaneousInput) (SimultaneousPlan, error) {
	out := SimultaneousPlan{LocationID: in.LocationID, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano), Basis: "shared_current_free_stock_not_reserved", Feasible: true, Outputs: []PlannedOutput{}, Materials: []SharedMaterial{}}
	if db == nil || !validID(in.LocationID) || len(in.Recipes) < 1 || len(in.Recipes) > 20 {
		return out, ErrInvalid
	}
	seen := map[string]bool{}
	for _, r := range in.Recipes {
		if !validID(r.VersionID) || r.Batches < 1 || r.Batches > MaxQuantity || seen[r.VersionID] {
			return out, ErrInvalid
		}
		seen[r.VersionID] = true
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM stock_locations WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LocationID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	required := map[string]*SharedMaterial{}
	for _, request := range in.Recipes {
		v, e := scanVersion(tx.QueryRowContext(ctx, `SELECT request_json,revision,created_at,actor_id FROM production_recipe_versions WHERE tenant_id=? AND store_id=? AND version_id=?`, a.TenantID, a.StoreID, request.VersionID))
		if e != nil {
			return out, e
		}
		if v.VersionID != request.VersionID {
			return out, ErrCapacity
		}
		norm, _, e := normalize(v.PublishInput)
		if e != nil {
			return out, ErrCapacity
		}
		var outputUnit string
		if e = tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, v.OutputProductID).Scan(&outputUnit); e != nil {
			return out, e
		}
		if outputUnit != v.OutputUnit {
			return out, ErrCapacity
		}
		q, e := boundedPositiveProduct(v.YieldMilli, request.Batches)
		if e != nil {
			return out, e
		}
		out.Outputs = append(out.Outputs, PlannedOutput{VersionID: v.VersionID, ProductID: v.OutputProductID, Unit: v.OutputUnit, Batches: request.Batches, OutputMilli: q})
		for _, ingredient := range norm.Ingredients {
			q, e = boundedPositiveProduct(ingredient.QuantityMilli, request.Batches)
			if e != nil {
				return out, e
			}
			row := required[ingredient.ProductID]
			if row == nil {
				row = &SharedMaterial{ProductID: ingredient.ProductID, Unit: ingredient.Unit}
				required[ingredient.ProductID] = row
			}
			if row.Unit != ingredient.Unit || row.RequiredMilli > MaxQuantity-q {
				return out, ErrCapacity
			}
			row.RequiredMilli += q
		}
	}
	ids := make([]string, 0, len(required))
	for id := range required {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		row := required[id]
		var unit string
		if err = tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, id).Scan(&unit); err != nil {
			return out, err
		}
		if unit != row.Unit {
			return out, ErrCapacity
		}
		row.PhysicalMilli, err = locationBalance(ctx, tx, a, id, in.LocationID)
		if err != nil {
			return out, err
		}
		row.ReservedMilli, err = stockreservation.HeldTx(ctx, tx, a.TenantID, a.StoreID, id, in.LocationID)
		if err != nil {
			return out, err
		}
		if row.ReservedMilli < 0 || row.ReservedMilli > row.PhysicalMilli {
			return out, ErrCapacity
		}
		row.FreeMilli = row.PhysicalMilli - row.ReservedMilli
		if row.RequiredMilli > row.FreeMilli {
			row.ShortfallMilli = row.RequiredMilli - row.FreeMilli
			out.Feasible = false
		} else {
			row.RemainingMilli = row.FreeMilli - row.RequiredMilli
		}
		out.Materials = append(out.Materials, *row)
	}
	return out, tx.Commit()
}
