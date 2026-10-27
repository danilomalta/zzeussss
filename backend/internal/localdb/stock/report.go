package stock

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type PositionInput struct {
	LocationID string `json:"location_id"`
	ProductID  string `json:"product_id"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}
type StockPosition struct {
	Availability
	SKU                string  `json:"sku"`
	Name               string  `json:"name"`
	CatalogCostCents   *int64  `json:"catalog_cost_cents,omitempty"`
	ValueNumerator     *string `json:"value_cents_numerator,omitempty"`
	ValueDenominator   *string `json:"value_cents_denominator,omitempty"`
	CostRequiresReview *bool   `json:"cost_requires_review,omitempty"`
}
type PositionReport struct {
	MeasuredAt           string          `json:"measured_at"`
	Basis                string          `json:"basis"`
	Items                []StockPosition `json:"items"`
	Total                int64           `json:"total"`
	Limit                int             `json:"limit"`
	Offset               int             `json:"offset"`
	PageValueNumerator   *string         `json:"page_value_cents_numerator,omitempty"`
	PageValueDenominator *string         `json:"page_value_cents_denominator,omitempty"`
	HistoricalCost       bool            `json:"historical_cost"`
}

// Positions requires a location so a large catalogue never expands into the
// Cartesian product of every store location. Values are PAGE totals only.
func Positions(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in PositionInput, valued bool) (PositionReport, error) {
	out := PositionReport{MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano), Basis: "current_physical_reserved_free", Items: []StockPosition{}, Limit: in.Limit, Offset: in.Offset}
	if db == nil || !validAvailabilityID(in.LocationID) || (in.ProductID != "" && !validAvailabilityID(in.ProductID)) || in.Limit < 1 || in.Limit > 100 || in.Offset < 0 || in.Offset > 1000000 {
		return out, ErrInvalidOperation
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
		return out, err
	}
	if valued {
		if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
			return out, err
		}
		out.Basis = "current_catalog_cost_estimate_page_only"
	}
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM stock_locations WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LocationID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrAvailabilityReference
	}
	if err != nil {
		return out, err
	}
	where := `tenant_id=?`
	args := []any{a.TenantID}
	if in.ProductID != "" {
		where += ` AND id=?`
		args = append(args, in.ProductID)
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM products WHERE `+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	params := append(append([]any{}, args...), in.Limit, in.Offset)
	// Do not read costs at all in the non-financial path.
	cost := "0"
	if valued {
		cost = "cost_cents"
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,sku,name,`+cost+` FROM products WHERE `+where+` ORDER BY id LIMIT ? OFFSET ?`, params...)
	if err != nil {
		return out, err
	}
	type product struct {
		id, sku, name string
		cost          int64
	}
	products := []product{}
	for rows.Next() {
		var p product
		if err = rows.Scan(&p.id, &p.sku, &p.name, &p.cost); err != nil {
			rows.Close()
			return out, err
		}
		products = append(products, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	total := new(big.Rat)
	for _, p := range products {
		balance, e := availableBalanceTx(ctx, tx, a, p.id, in.LocationID)
		if e != nil {
			return out, e
		}
		item := StockPosition{Availability: balance, SKU: p.sku, Name: p.name}
		if valued {
			if p.cost < 0 || p.cost > MaxAvailabilityQuantity {
				return out, ErrAvailability
			}
			value := new(big.Rat).SetFrac(new(big.Int).Mul(big.NewInt(balance.PhysicalMilli), big.NewInt(p.cost)), big.NewInt(1000))
			item.CatalogCostCents = &p.cost
			num, den := value.Num().String(), value.Denom().String()
			item.ValueNumerator = &num
			item.ValueDenominator = &den
			review := p.cost == 0
			item.CostRequiresReview = &review
			total.Add(total, value)
		}
		out.Items = append(out.Items, item)
	}
	if valued {
		n, d := total.Num().String(), total.Denom().String()
		out.PageValueNumerator = &n
		out.PageValueDenominator = &d
	}
	return out, tx.Commit()
}
