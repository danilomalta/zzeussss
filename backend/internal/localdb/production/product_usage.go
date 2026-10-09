package production

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/identity"
)

type UsageProduct struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Unit string `json:"unit"`
}
type ProductUsageLink struct {
	Kind               string `json:"kind"`
	ID                 string `json:"id"`
	RecipeID           string `json:"recipe_id"`
	VersionID          string `json:"version_id"`
	Role               string `json:"role"`
	Unit               string `json:"unit"`
	QuantityMilli      int64  `json:"quantity_milli"`
	QuantityBasis      string `json:"quantity_basis"`
	UnitMatchesCatalog bool   `json:"unit_matches_catalog"`
	LatestVersion      *bool  `json:"latest_version,omitempty"`
	OrderStatus        string `json:"order_status,omitempty"`
	LocationID         string `json:"location_id,omitempty"`
}
type ProductUsage struct {
	Product                  UsageProduct       `json:"product"`
	StoreID                  string             `json:"store_id"`
	Kind                     string             `json:"kind"`
	RecipeVersionCount       int64              `json:"recipe_version_count"`
	LatestRecipeVersionCount int64              `json:"latest_recipe_version_count"`
	OrderCount               int64              `json:"order_count"`
	Offset                   int64              `json:"offset"`
	Limit                    int                `json:"limit"`
	HasMore                  bool               `json:"has_more"`
	Items                    []ProductUsageLink `json:"items"`
}

func usageFields(recipe PublishInput, productID string, batches int64) (string, string, int64, error) {
	if _, _, err := normalize(recipe); err != nil || batches < 1 || batches > MaxQuantity {
		return "", "", 0, ErrTrace
	}
	role, unit, quantity := "", "", int64(0)
	if productID == recipe.OutputProductID {
		role, unit, quantity = "output", recipe.OutputUnit, recipe.YieldMilli
	} else {
		for _, item := range recipe.Ingredients {
			if item.ProductID == productID {
				role, unit, quantity = "ingredient", item.Unit, item.QuantityMilli
				break
			}
		}
	}
	if quantity < 1 || batches > MaxQuantity/quantity {
		return "", "", 0, ErrTrace
	}
	return role, unit, quantity * batches, nil
}

const versionUsageWhere = `tenant_id=? AND store_id=? AND (output_product_id=? OR EXISTS(SELECT 1 FROM production_recipe_ingredients i WHERE i.tenant_id=production_recipe_versions.tenant_id AND i.store_id=production_recipe_versions.store_id AND i.version_id=production_recipe_versions.version_id AND i.product_id=?))`

// Orders refer to immutable versions. Use their scoped product references to
// find candidates, then derive quantities exclusively from each frozen order.
const orderUsageWhere = `tenant_id=? AND store_id=? AND EXISTS(SELECT 1 FROM production_recipe_versions v WHERE v.tenant_id=production_orders.tenant_id AND v.store_id=production_orders.store_id AND v.version_id=production_orders.version_id AND (v.output_product_id=? OR EXISTS(SELECT 1 FROM production_recipe_ingredients i WHERE i.tenant_id=v.tenant_id AND i.store_id=v.store_id AND i.version_id=v.version_id AND i.product_id=?)))`

func GetProductUsage(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id, kind string, offset int64) (ProductUsage, error) {
	if !validID(id) || (kind != "versions" && kind != "orders") || offset < 0 || offset > MaxQuantity {
		return ProductUsage{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return ProductUsage{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewCatalog); err != nil {
		return ProductUsage{}, err
	}
	out := ProductUsage{StoreID: a.StoreID, Kind: kind, Offset: offset, Limit: 50, Items: []ProductUsageLink{}}
	err = tx.QueryRowContext(ctx, `SELECT id,name,unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, id).Scan(&out.Product.ID, &out.Product.Name, &out.Product.Unit)
	if err == sql.ErrNoRows {
		return ProductUsage{}, ErrNotFound
	}
	if err != nil {
		return ProductUsage{}, err
	}
	args := []any{a.TenantID, a.StoreID, id, id}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_recipe_versions WHERE `+versionUsageWhere, args...).Scan(&out.RecipeVersionCount); err != nil {
		return ProductUsage{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_recipe_versions WHERE `+versionUsageWhere+` AND revision=(SELECT h.revision FROM production_recipes h WHERE h.tenant_id=production_recipe_versions.tenant_id AND h.store_id=production_recipe_versions.store_id AND h.id=production_recipe_versions.recipe_id)`, args...).Scan(&out.LatestRecipeVersionCount); err != nil {
		return ProductUsage{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_orders WHERE `+orderUsageWhere, args...).Scan(&out.OrderCount); err != nil {
		return ProductUsage{}, err
	}
	for _, n := range []int64{out.RecipeVersionCount, out.LatestRecipeVersionCount, out.OrderCount} {
		if n < 0 || n > MaxQuantity {
			return ProductUsage{}, ErrTrace
		}
	}
	if out.LatestRecipeVersionCount > out.RecipeVersionCount {
		return ProductUsage{}, ErrTrace
	}
	pageArgs := append(append([]any{}, args...), offset)
	if kind == "versions" {
		rows, e := tx.QueryContext(ctx, `SELECT request_json,revision,created_at,actor_id FROM production_recipe_versions WHERE `+versionUsageWhere+` ORDER BY recipe_id,revision LIMIT 50 OFFSET ?`, pageArgs...)
		if e != nil {
			return ProductUsage{}, e
		}
		defer rows.Close()
		versions := []Version{}
		for rows.Next() {
			v, e := scanVersion(rows)
			if e != nil {
				return ProductUsage{}, e
			}
			versions = append(versions, v)
		}
		if e = rows.Err(); e != nil {
			return ProductUsage{}, e
		}
		if e = rows.Close(); e != nil {
			return ProductUsage{}, e
		}
		for _, v := range versions {
			role, unit, quantity, e := usageFields(v.PublishInput, id, 1)
			if e != nil {
				return ProductUsage{}, e
			}
			var current int64
			if e = tx.QueryRowContext(ctx, `SELECT revision FROM production_recipes WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, v.RecipeID).Scan(&current); e != nil {
				return ProductUsage{}, e
			}
			latest := current == v.Revision
			out.Items = append(out.Items, ProductUsageLink{Kind: kind, ID: v.VersionID, RecipeID: v.RecipeID, VersionID: v.VersionID, Role: role, Unit: unit, QuantityMilli: quantity, QuantityBasis: "per_batch", UnitMatchesCatalog: unit == out.Product.Unit, LatestVersion: &latest})
		}
	} else {
		rows, e := tx.QueryContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE `+orderUsageWhere+` ORDER BY created_at DESC,id DESC LIMIT 50 OFFSET ?`, pageArgs...)
		if e != nil {
			return ProductUsage{}, e
		}
		defer rows.Close()
		orders := []Order{}
		for rows.Next() {
			v, e := scanOrder(rows)
			if e != nil {
				return ProductUsage{}, e
			}
			orders = append(orders, v)
		}
		if e = rows.Err(); e != nil {
			return ProductUsage{}, e
		}
		if e = rows.Close(); e != nil {
			return ProductUsage{}, e
		}
		for _, v := range orders {
			if _, e = plannedTraceIngredients(v); e != nil {
				return ProductUsage{}, e
			}
			role, unit, quantity, e := usageFields(v.Recipe.PublishInput, id, v.PlannedBatches)
			if e != nil {
				return ProductUsage{}, e
			}
			out.Items = append(out.Items, ProductUsageLink{Kind: kind, ID: v.ID, RecipeID: v.Recipe.RecipeID, VersionID: v.VersionID, Role: role, Unit: unit, QuantityMilli: quantity, QuantityBasis: "planned_total", UnitMatchesCatalog: unit == out.Product.Unit, OrderStatus: v.Status, LocationID: v.LocationID})
		}
	}
	total := out.RecipeVersionCount
	if kind == "orders" {
		total = out.OrderCount
	}
	out.HasMore = offset < total && int64(len(out.Items)) < total-offset
	return out, tx.Commit()
}
