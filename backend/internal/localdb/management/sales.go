package management

import (
	"context"
	"database/sql"
	"math/big"
	"sort"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type accumulator struct {
	row                                      ProductSales
	currentQ, currentR, previousQ, previousR *big.Int
}

// Periods are half-open [from,to), and compared independently when they overlap.
// The coarse SQL boundary is widened by a second; precise Go timestamp checks
// prevent SQLite's millisecond date rounding from assigning boundary rows twice.
func Sales(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in SalesInput) (SalesReport, error) {
	out := SalesReport{Items: []ProductSales{}, Limit: in.Limit, Offset: in.Offset, Basis: "committed_sale_item_totals_not_cash_settlement", ABCBasis: "same_store_current_revenue_desc_ties_product_id_80_95_previous_cumulative"}
	if db == nil || !validWindow(in.Current) || !validWindow(in.Previous) || in.Limit < 1 || in.Limit > 100 || in.Offset < 0 || in.Offset > 1000000 {
		return out, ErrInvalid
	}
	out.From = in.Current.From.UTC().Format(time.RFC3339Nano)
	out.To = in.Current.To.UTC().Format(time.RFC3339Nano)
	out.CompareFrom = in.Previous.From.UTC().Format(time.RFC3339Nano)
	out.CompareTo = in.Previous.To.UTC().Format(time.RFC3339Nano)
	out.CurrentSeconds = windowSeconds(in.Current)
	out.PreviousSeconds = windowSeconds(in.Previous)
	out.EqualDuration = in.Current.To.Sub(in.Current.From) == in.Previous.To.Sub(in.Previous.From)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return out, err
	}
	from, to := in.Current.From, in.Current.To
	if in.Previous.From.Before(from) {
		from = in.Previous.From
	}
	if in.Previous.To.After(to) {
		to = in.Previous.To
	}
	// Stable row order and a hard ceiling give deterministic, bounded scans.
	rows, err := tx.QueryContext(ctx, `SELECT i.product_id,p.sku,p.name,p.unit,i.quantity_milli,i.total_cents,s.committed_at FROM sale_items i JOIN sales s ON s.tenant_id=i.tenant_id AND s.id=i.sale_id JOIN products p ON p.tenant_id=i.tenant_id AND p.id=i.product_id WHERE s.tenant_id=? AND s.store_id=? AND s.status='committed' AND (julianday(s.committed_at) IS NULL OR (julianday(s.committed_at)>=julianday(?) AND julianday(s.committed_at)<=julianday(?))) ORDER BY s.id,i.id LIMIT ?`, a.TenantID, a.StoreID, from.Add(-time.Second).UTC().Format(time.RFC3339Nano), to.Add(time.Second).UTC().Format(time.RFC3339Nano), MaxSourceRows+1)
	if err != nil {
		return out, err
	}
	records := map[string]*accumulator{}
	for rows.Next() {
		out.SourceRows++
		if out.SourceRows > MaxSourceRows {
			rows.Close()
			return out, ErrLimit
		}
		var r ProductSales
		var q, c int64
		var stamp string
		if err = rows.Scan(&r.ProductID, &r.SKU, &r.Name, &r.Unit, &q, &c, &stamp); err != nil {
			rows.Close()
			return out, err
		}
		t, e := time.Parse(time.RFC3339Nano, stamp)
		if e != nil || q < 1 || q > MaxExact || c < 0 || c > MaxExact {
			rows.Close()
			return out, ErrIntegrity
		}
		current := !t.Before(in.Current.From) && t.Before(in.Current.To)
		previous := !t.Before(in.Previous.From) && t.Before(in.Previous.To)
		if !current && !previous {
			continue
		}
		v := records[r.ProductID]
		if v == nil {
			v = &accumulator{r, new(big.Int), new(big.Int), new(big.Int), new(big.Int)}
			records[r.ProductID] = v
		}
		if current {
			v.currentQ.Add(v.currentQ, big.NewInt(q))
			v.currentR.Add(v.currentR, big.NewInt(c))
		}
		if previous {
			v.previousQ.Add(v.previousQ, big.NewInt(q))
			v.previousR.Add(v.previousR, big.NewInt(c))
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	all := make([]ProductSales, 0, len(records))
	currentTotal, previousTotal := new(big.Int), new(big.Int)
	for _, v := range records {
		r := v.row
		r.QuantityMilli, err = exactInt(v.currentQ)
		if err != nil {
			return out, err
		}
		r.RevenueCents, err = exactInt(v.currentR)
		if err != nil {
			return out, err
		}
		r.PreviousQuantityMilli, err = exactInt(v.previousQ)
		if err != nil {
			return out, err
		}
		r.PreviousRevenueCents, err = exactInt(v.previousR)
		if err != nil {
			return out, err
		}
		r.RevenueDeltaCents = r.RevenueCents - r.PreviousRevenueCents
		r.RevenueChangeBasisPoints, r.ChangeBasis = change(r.RevenueCents, r.PreviousRevenueCents)
		if !out.EqualDuration {
			r.RevenueChangeBasisPoints = nil
			r.ChangeBasis = "different_duration_totals_only"
		}
		currentTotal.Add(currentTotal, v.currentR)
		previousTotal.Add(previousTotal, v.previousR)
		all = append(all, r)
	}
	out.RevenueCents, err = exactInt(currentTotal)
	if err != nil {
		return out, err
	}
	out.PreviousRevenueCents, err = exactInt(previousTotal)
	if err != nil {
		return out, err
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].RevenueCents == all[j].RevenueCents {
			return all[i].ProductID < all[j].ProductID
		}
		return all[i].RevenueCents > all[j].RevenueCents
	})
	cumulative := new(big.Int)
	for i := range all {
		if all[i].RevenueCents == 0 {
			all[i].ABCCategory = "no_current_revenue"
		} else {
			percent := new(big.Int).Mul(cumulative, big.NewInt(100))
			if percent.Cmp(new(big.Int).Mul(currentTotal, big.NewInt(80))) < 0 {
				all[i].ABCCategory = "A"
			} else if percent.Cmp(new(big.Int).Mul(currentTotal, big.NewInt(95))) < 0 {
				all[i].ABCCategory = "B"
			} else {
				all[i].ABCCategory = "C"
			}
		}
		cumulative.Add(cumulative, big.NewInt(all[i].RevenueCents))
	}
	out.TotalProducts = len(all)
	if in.Offset < len(all) {
		end := in.Offset + in.Limit
		if end > len(all) {
			end = len(all)
		}
		out.Items = all[in.Offset:end]
	}
	return out, tx.Commit()
}
