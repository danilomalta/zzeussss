package stock

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type RestockPreviewInput struct {
	ProductID    string `json:"product_id"`
	From         string `json:"from"`
	To           string `json:"to"`
	LeadDays     int64  `json:"lead_days"`
	CoverageDays int64  `json:"coverage_days"`
	SafetyMilli  int64  `json:"safety_milli"`
}
type RestockPreview struct {
	ProductID             string `json:"product_id"`
	Unit                  string `json:"unit"`
	From                  string `json:"from"`
	To                    string `json:"to"`
	MeasuredAt            string `json:"measured_at"`
	Basis                 string `json:"basis"`
	DemandBasis           string `json:"demand_basis"`
	SoldMilli             int64  `json:"sold_milli"`
	DemandNumerator       string `json:"daily_demand_milli_numerator"`
	DemandDenominator     string `json:"daily_demand_milli_denominator"`
	PhysicalMilli         int64  `json:"physical_milli"`
	ReservedMilli         int64  `json:"reserved_milli"`
	FreeMilli             int64  `json:"free_milli"`
	PolicyRevision        int64  `json:"policy_revision"`
	PolicyMinimumMilli    int64  `json:"policy_minimum_milli"`
	PolicyTargetMilli     int64  `json:"policy_target_milli"`
	SuggestedMinimumMilli int64  `json:"suggested_minimum_milli"`
	SuggestedTargetMilli  int64  `json:"suggested_target_milli"`
	SuggestedOrderMilli   int64  `json:"suggested_order_milli"`
	ReviewRequired        bool   `json:"review_required"`
	OrderCreated          bool   `json:"order_created"`
}

func ceilStock(r *big.Rat, unit string) (int64, error) {
	if r == nil || r.Sign() < 0 {
		return 0, ErrInvalidOperation
	}
	scale := int64(1)
	if unit == "unit" {
		scale = 1000
	}
	denominator := new(big.Int).Mul(r.Denom(), big.NewInt(scale))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), denominator, rem)
	if rem.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	q.Mul(q, big.NewInt(scale))
	if q.Sign() < 0 || !q.IsInt64() || q.Cmp(big.NewInt(MaxAvailabilityQuantity)) > 0 {
		return 0, ErrInvalidOperation
	}
	return q.Int64(), nil
}

// Uses one product's observed committed quantities and the existing human
// policy. This is a deterministic what-if, not a forecast guarantee or order.
func PreviewRestock(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in RestockPreviewInput) (RestockPreview, error) {
	out := RestockPreview{ProductID: in.ProductID, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano), Basis: "same_store_demand_and_all_local_locations", ReviewRequired: true}
	from, e1 := time.Parse(time.RFC3339Nano, in.From)
	to, e2 := time.Parse(time.RFC3339Nano, in.To)
	if db == nil || !validAvailabilityID(in.ProductID) || e1 != nil || e2 != nil || !from.Before(to) || from.Year() < 2000 || to.Year() > 2200 || to.Sub(from) < time.Hour || to.Sub(from) > 366*24*time.Hour || in.LeadDays < 0 || in.LeadDays > 365 || in.CoverageDays < 1 || in.CoverageDays > 365 || in.SafetyMilli < 0 || in.SafetyMilli > MaxAvailabilityQuantity {
		return out, ErrInvalidOperation
	}
	out.From = from.UTC().Format(time.RFC3339Nano)
	out.To = to.UTC().Format(time.RFC3339Nano)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	for _, permission := range []identity.Permission{identity.ManageStock, identity.ManageReplenishment} {
		if err = identity.CanOperateTx(ctx, tx, a, d, permission); err != nil {
			return out, err
		}
	}
	err = tx.QueryRowContext(ctx, `SELECT p.unit,r.minimum_milli,r.target_milli,r.revision FROM products p JOIN restock_policies r ON r.tenant_id=p.tenant_id AND r.product_id=p.id WHERE p.tenant_id=? AND r.store_id=? AND p.id=?`, a.TenantID, a.StoreID, in.ProductID).Scan(&out.Unit, &out.PolicyMinimumMilli, &out.PolicyTargetMilli, &out.PolicyRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrAvailabilityReference
	}
	if err != nil {
		return out, err
	}
	if !validAvailabilityUnit(out.Unit) || out.PolicyMinimumMilli < 0 || out.PolicyTargetMilli < out.PolicyMinimumMilli || out.PolicyTargetMilli > MaxAvailabilityQuantity || out.PolicyRevision < 1 || (out.Unit == "unit" && (in.SafetyMilli%1000 != 0 || out.PolicyMinimumMilli%1000 != 0 || out.PolicyTargetMilli%1000 != 0)) {
		return out, ErrAvailability
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM stock_locations WHERE tenant_id=? AND store_id=? ORDER BY id LIMIT 501`, a.TenantID, a.StoreID)
	if err != nil {
		return out, err
	}
	locations := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		locations = append(locations, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(locations) > 500 {
		return out, ErrInvalidOperation
	}
	physical, reserved := new(big.Int), new(big.Int)
	for _, id := range locations {
		v, e := availableBalanceTx(ctx, tx, a, in.ProductID, id)
		if e != nil {
			return out, e
		}
		physical.Add(physical, big.NewInt(v.PhysicalMilli))
		reserved.Add(reserved, big.NewInt(v.ReservedMilli))
	}
	out.PhysicalMilli, err = boundedAvailabilityTotal(physical)
	if err != nil {
		return out, err
	}
	out.ReservedMilli, err = boundedAvailabilityTotal(reserved)
	if err != nil {
		return out, err
	}
	if out.ReservedMilli > out.PhysicalMilli {
		return out, ErrAvailability
	}
	out.FreeMilli = out.PhysicalMilli - out.ReservedMilli
	rows, err = tx.QueryContext(ctx, `SELECT i.quantity_milli,s.committed_at FROM sale_items i JOIN sales s ON s.tenant_id=i.tenant_id AND s.id=i.sale_id WHERE i.tenant_id=? AND s.store_id=? AND i.product_id=? AND s.status='committed' AND (julianday(s.committed_at) IS NULL OR (julianday(s.committed_at)>=julianday(?) AND julianday(s.committed_at)<=julianday(?))) LIMIT 100001`, a.TenantID, a.StoreID, in.ProductID, from.Add(-time.Second).UTC().Format(time.RFC3339Nano), to.Add(time.Second).UTC().Format(time.RFC3339Nano))
	if err != nil {
		return out, err
	}
	sold := new(big.Int)
	n := 0
	for rows.Next() {
		n++
		if n > 100000 {
			rows.Close()
			return out, ErrInvalidOperation
		}
		var q int64
		var stamp string
		if err = rows.Scan(&q, &stamp); err != nil {
			rows.Close()
			return out, err
		}
		t, e := time.Parse(time.RFC3339Nano, stamp)
		if e != nil || q < 1 || q > MaxAvailabilityQuantity || (out.Unit == "unit" && q%1000 != 0) {
			rows.Close()
			return out, ErrAvailability
		}
		if !t.Before(from) && t.Before(to) {
			sold.Add(sold, big.NewInt(q))
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.SoldMilli, err = boundedAvailabilityTotal(sold)
	if err != nil {
		return out, err
	}
	daily := new(big.Rat).SetFrac(new(big.Int).Mul(sold, big.NewInt(int64(24*time.Hour))), big.NewInt(int64(to.Sub(from))))
	out.DemandNumerator, out.DemandDenominator = daily.Num().String(), daily.Denom().String()
	out.DemandBasis = "observed_committed_sales_not_future_guarantee"
	if sold.Sign() == 0 {
		out.DemandBasis = "no_sales_in_selected_period_policy_only"
	}
	minimum := new(big.Rat).Add(new(big.Rat).Mul(daily, new(big.Rat).SetInt64(in.LeadDays)), new(big.Rat).SetInt64(in.SafetyMilli))
	target := new(big.Rat).Add(new(big.Rat).Mul(daily, new(big.Rat).SetInt64(in.LeadDays+in.CoverageDays)), new(big.Rat).SetInt64(in.SafetyMilli))
	out.SuggestedMinimumMilli, err = ceilStock(minimum, out.Unit)
	if err != nil {
		return out, err
	}
	out.SuggestedTargetMilli, err = ceilStock(target, out.Unit)
	if err != nil {
		return out, err
	}
	if out.SuggestedMinimumMilli < out.PolicyMinimumMilli {
		out.SuggestedMinimumMilli = out.PolicyMinimumMilli
	}
	if out.SuggestedTargetMilli < out.PolicyTargetMilli {
		out.SuggestedTargetMilli = out.PolicyTargetMilli
	}
	if out.FreeMilli <= out.SuggestedMinimumMilli && out.FreeMilli < out.SuggestedTargetMilli {
		out.SuggestedOrderMilli = out.SuggestedTargetMilli - out.FreeMilli
		if out.Unit == "unit" {
			out.SuggestedOrderMilli, err = ceilStock(new(big.Rat).SetInt64(out.SuggestedOrderMilli), out.Unit)
			if err != nil {
				return out, err
			}
		}
	}
	return out, tx.Commit()
}
