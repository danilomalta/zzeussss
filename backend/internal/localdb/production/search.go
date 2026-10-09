package production

import (
	"context"
	"database/sql"

	"titansystem-backend/internal/localdb/identity"
)

type OrderSearch struct {
	Status        string `json:"status,omitempty"`
	LocationID    string `json:"location_id,omitempty"`
	ResponsibleID string `json:"responsible_id,omitempty"`
	VersionID     string `json:"version_id,omitempty"`
	Offset        int64  `json:"offset"`
}
type OrderSearchPage struct {
	Filters    OrderSearch `json:"filters"`
	TotalCount int64       `json:"total_count"`
	Limit      int         `json:"limit"`
	HasMore    bool        `json:"has_more"`
	Items      []Order     `json:"items"`
}

func validOrderSearch(in OrderSearch) bool {
	if in.Offset < 0 || in.Offset > MaxQuantity {
		return false
	}
	switch in.Status {
	case "", "planned", "approved", "cancelled", "completed":
	default:
		return false
	}
	for _, id := range []string{in.LocationID, in.ResponsibleID, in.VersionID} {
		if id != "" && !validID(id) {
			return false
		}
	}
	return true
}

// Completion is represented by a production_results record, not an update of
// the physical orders.status column. Both count and page use this same predicate.
func orderSearchPredicate(a identity.Scope, in OrderSearch) (string, []any) {
	where := `tenant_id=? AND store_id=?`
	args := []any{a.TenantID, a.StoreID}
	completed := `EXISTS(SELECT 1 FROM production_results r WHERE r.tenant_id=production_orders.tenant_id AND r.store_id=production_orders.store_id AND r.order_id=production_orders.id)`
	if in.Status == "completed" {
		where += " AND " + completed
	} else if in.Status != "" {
		where += " AND status=? AND NOT " + completed
		args = append(args, in.Status)
	}
	for _, pair := range []struct{ column, value string }{{"location_id", in.LocationID}, {"responsible_id", in.ResponsibleID}, {"version_id", in.VersionID}} {
		if pair.value != "" {
			where += " AND " + pair.column + "=?"
			args = append(args, pair.value)
		}
	}
	return where, args
}

func SearchOrders(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in OrderSearch) (OrderSearchPage, error) {
	if !validOrderSearch(in) {
		return OrderSearchPage{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return OrderSearchPage{}, err
	}
	defer tx.Rollback()
	out := OrderSearchPage{Filters: in, Limit: 50, Items: []Order{}}
	where, args := orderSearchPredicate(a, in)
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM production_orders WHERE "+where, args...).Scan(&out.TotalCount); err != nil {
		return OrderSearchPage{}, err
	}
	if out.TotalCount < 0 || out.TotalCount > MaxQuantity {
		return OrderSearchPage{}, ErrTrace
	}
	pageArgs := append(append([]any{}, args...), in.Offset)
	rows, err := tx.QueryContext(ctx, "SELECT "+orderColumns+" FROM production_orders WHERE "+where+" ORDER BY created_at DESC,id DESC LIMIT 50 OFFSET ?", pageArgs...)
	if err != nil {
		return OrderSearchPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		v, e := scanOrder(rows)
		if e != nil {
			return OrderSearchPage{}, e
		}
		if _, e = plannedTraceIngredients(v); e != nil {
			return OrderSearchPage{}, e
		}
		out.Items = append(out.Items, v)
	}
	if err = rows.Err(); err != nil {
		return OrderSearchPage{}, err
	}
	if err = rows.Close(); err != nil {
		return OrderSearchPage{}, err
	}
	out.HasMore = in.Offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-in.Offset
	return out, tx.Commit()
}
