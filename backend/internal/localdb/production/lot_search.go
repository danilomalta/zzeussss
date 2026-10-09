package production

import (
	"context"
	"database/sql"
	"time"

	"titansystem-backend/internal/localdb/identity"
)

type LotSearch struct {
	Status      string `json:"status,omitempty"`
	Quality     string `json:"quality,omitempty"`
	ProductID   string `json:"product_id,omitempty"`
	LocationID  string `json:"location_id,omitempty"`
	Expiry      string `json:"expiry,omitempty"`
	ExpiresFrom string `json:"expires_from,omitempty"`
	ExpiresTo   string `json:"expires_to,omitempty"`
	Offset      int64  `json:"offset"`
}
type LotSearchItem struct {
	Lot        ProductionLot `json:"lot"`
	OrderID    string        `json:"order_id"`
	LocationID string        `json:"location_id"`
	Quality    LotQuality    `json:"quality"`
}
type LotSearchPage struct {
	Filters    LotSearch       `json:"filters"`
	TotalCount int64           `json:"total_count"`
	Limit      int             `json:"limit"`
	HasMore    bool            `json:"has_more"`
	Items      []LotSearchItem `json:"items"`
}

func validSearchDate(value string) bool {
	t, err := time.Parse("2006-01-02", value)
	return err == nil && t.Year() >= 1 && t.Format("2006-01-02") == value
}
func validLotSearch(in LotSearch) bool {
	if in.Offset < 0 || in.Offset > MaxQuantity {
		return false
	}
	switch in.Status {
	case "", "recorded", "voided":
	default:
		return false
	}
	switch in.Quality {
	case "", "not_assessed", "passed", "failed":
	default:
		return false
	}
	switch in.Expiry {
	case "", "dated", "undated":
	default:
		return false
	}
	for _, id := range []string{in.ProductID, in.LocationID} {
		if id != "" && !validID(id) {
			return false
		}
	}
	for _, date := range []string{in.ExpiresFrom, in.ExpiresTo} {
		if date != "" && !validSearchDate(date) {
			return false
		}
	}
	if in.ExpiresFrom != "" && in.ExpiresTo != "" && in.ExpiresFrom > in.ExpiresTo {
		return false
	}
	return in.Expiry != "undated" || (in.ExpiresFrom == "" && in.ExpiresTo == "")
}
func lotSearchPredicate(a identity.Scope, in LotSearch) (string, []any) {
	where := `tenant_id=? AND store_id=?`
	args := []any{a.TenantID, a.StoreID}
	for _, pair := range []struct{ column, value string }{{"status", in.Status}, {"product_id", in.ProductID}} {
		if pair.value != "" {
			where += " AND " + pair.column + "=?"
			args = append(args, pair.value)
		}
	}
	if in.LocationID != "" {
		where += ` AND EXISTS(SELECT 1 FROM production_results r WHERE r.tenant_id=production_lots.tenant_id AND r.store_id=production_lots.store_id AND r.id=production_lots.result_id AND r.location_id=?)`
		args = append(args, in.LocationID)
	}
	if in.Quality != "" {
		where += ` AND COALESCE((SELECT q.status FROM production_quality_reviews q WHERE q.tenant_id=production_lots.tenant_id AND q.store_id=production_lots.store_id AND q.lot_id=production_lots.id ORDER BY q.revision DESC LIMIT 1),'not_assessed')=?`
		args = append(args, in.Quality)
	}
	if in.Expiry == "undated" {
		where += ` AND expires_on=''`
	} else if in.Expiry == "dated" || in.ExpiresFrom != "" || in.ExpiresTo != "" {
		where += ` AND expires_on<>''`
	}
	if in.ExpiresFrom != "" {
		where += ` AND expires_on>=?`
		args = append(args, in.ExpiresFrom)
	}
	if in.ExpiresTo != "" {
		where += ` AND expires_on<=?`
		args = append(args, in.ExpiresTo)
	}
	return where, args
}

// The lot remains a historical classification of a result, not a physical
// balance or a sales approval. Date bounds are supplied calendar dates.
func SearchProductionLots(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in LotSearch) (LotSearchPage, error) {
	if !validLotSearch(in) {
		return LotSearchPage{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return LotSearchPage{}, err
	}
	defer tx.Rollback()
	out := LotSearchPage{Filters: in, Limit: 50, Items: []LotSearchItem{}}
	where, args := lotSearchPredicate(a, in)
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_lots WHERE `+where, args...).Scan(&out.TotalCount); err != nil {
		return LotSearchPage{}, err
	}
	if out.TotalCount < 0 || out.TotalCount > MaxQuantity {
		return LotSearchPage{}, ErrTrace
	}
	pageArgs := append(append([]any{}, args...), in.Offset)
	rows, err := tx.QueryContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE `+where+` ORDER BY (expires_on=''),expires_on,created_at,id LIMIT 50 OFFSET ?`, pageArgs...)
	if err != nil {
		return LotSearchPage{}, err
	}
	defer rows.Close()
	lots := []ProductionLot{}
	for rows.Next() {
		lot, e := scanLot(rows)
		if e != nil {
			return LotSearchPage{}, e
		}
		if !validLotMetadata(lot.Code, lot.ManufacturedOn, lot.ExpiresOn) || !validUnit(lot.Unit) || lot.QuantityMilli < 1 || lot.QuantityMilli > MaxQuantity || (lot.Unit == "unit" && lot.QuantityMilli%1000 != 0) {
			return LotSearchPage{}, ErrTrace
		}
		lots = append(lots, lot)
	}
	if err = rows.Err(); err != nil {
		return LotSearchPage{}, err
	}
	if err = rows.Close(); err != nil {
		return LotSearchPage{}, err
	}
	for _, lot := range lots {
		item := LotSearchItem{Lot: lot}
		var product, unit string
		if err = tx.QueryRowContext(ctx, `SELECT order_id,location_id,product_id,unit FROM production_results WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, lot.ResultID).Scan(&item.OrderID, &item.LocationID, &product, &unit); err != nil {
			return LotSearchPage{}, err
		}
		if product != lot.ProductID || unit != lot.Unit {
			return LotSearchPage{}, ErrTrace
		}
		item.Quality, err = traceQualityTx(ctx, tx, a, lot)
		if err != nil {
			return LotSearchPage{}, err
		}
		out.Items = append(out.Items, item)
	}
	out.HasMore = in.Offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-in.Offset
	return out, tx.Commit()
}
