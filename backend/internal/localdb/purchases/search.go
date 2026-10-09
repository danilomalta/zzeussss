package purchases

import (
	"context"
	"database/sql"
	"strings"

	"titansystem-backend/internal/localdb/identity"
)

type SearchFilter struct {
	SupplierID string `json:"supplier_id,omitempty"`
	ProductID  string `json:"product_id,omitempty"`
}
type SearchPage struct {
	Filters    SearchFilter `json:"filters"`
	TotalCount int64        `json:"total_count"`
	Offset     int64        `json:"offset"`
	Limit      int          `json:"limit"`
	HasMore    bool         `json:"has_more"`
	Items      []Order      `json:"items"`
}

func validSearchID(s string) bool {
	return validID(s) && !strings.ContainsAny(s, "\x00\r\n")
}

// Search reads historical order snapshots. It does not refresh them from catalog
// or supplier records, send orders, or advance the license clock.
func Search(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, filter SearchFilter, offset int64) (SearchPage, error) {
	if offset < 0 || offset > MaxQuantity || (filter.SupplierID != "" && !validSearchID(filter.SupplierID)) || (filter.ProductID != "" && !validSearchID(filter.ProductID)) {
		return SearchPage{}, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return SearchPage{}, err
	}
	defer tx.Rollback()
	out := SearchPage{Filters: filter, Offset: offset, Limit: 50, Items: []Order{}}
	where := ` WHERE o.tenant_id=? AND o.store_id=?`
	args := []any{a.TenantID, a.StoreID}
	if filter.SupplierID != "" {
		where += ` AND o.supplier_id=?`
		args = append(args, filter.SupplierID)
	}
	if filter.ProductID != "" {
		where += ` AND EXISTS(SELECT 1 FROM purchase_order_items i WHERE i.tenant_id=o.tenant_id AND i.store_id=o.store_id AND i.order_id=o.id AND i.product_id=?)`
		args = append(args, filter.ProductID)
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_orders o`+where, args...).Scan(&out.TotalCount); err != nil {
		return SearchPage{}, err
	}
	if out.TotalCount < 0 || out.TotalCount > MaxQuantity {
		return SearchPage{}, ErrConflict
	}
	pageArgs := append(append([]any{}, args...), offset)
	rows, err := tx.QueryContext(ctx, `SELECT o.id,o.operation_id,o.supplier_id,o.supplier_name,o.suggestion_id,o.status,o.created_at,o.approved_by,o.approved_at FROM purchase_orders o`+where+` ORDER BY o.created_at DESC,o.id DESC LIMIT 50 OFFSET ?`, pageArgs...)
	if err != nil {
		return SearchPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		v := Order{Items: []Item{}}
		if err = rows.Scan(&v.ID, &v.OperationID, &v.SupplierID, &v.SupplierName, &v.SuggestionID, &v.Status, &v.CreatedAt, &v.ApprovedBy, &v.ApprovedAt); err != nil {
			return SearchPage{}, err
		}
		if v.Status != "local_not_sent" {
			return SearchPage{}, ErrConflict
		}
		out.Items = append(out.Items, v)
	}
	if err = rows.Err(); err != nil {
		return SearchPage{}, err
	}
	// Close the cursor before loading items: local SQLite can use one connection.
	if err = rows.Close(); err != nil {
		return SearchPage{}, err
	}
	for n := range out.Items {
		itemRows, err := tx.QueryContext(ctx, `SELECT product_id,sku,name,unit,quantity_milli FROM purchase_order_items WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY product_id`, a.TenantID, a.StoreID, out.Items[n].ID)
		if err != nil {
			return SearchPage{}, err
		}
		for itemRows.Next() {
			var item Item
			if err = itemRows.Scan(&item.ProductID, &item.SKU, &item.Name, &item.Unit, &item.Quantity); err != nil {
				itemRows.Close()
				return SearchPage{}, err
			}
			if item.Quantity < 1 || item.Quantity > MaxQuantity || !validSearchID(item.ProductID) || (item.Unit != "unit" && item.Unit != "kg" && item.Unit != "g" && item.Unit != "liter" && item.Unit != "ml" && item.Unit != "meter") {
				itemRows.Close()
				return SearchPage{}, ErrConflict
			}
			out.Items[n].Items = append(out.Items[n].Items, item)
		}
		err = itemRows.Err()
		closeErr := itemRows.Close()
		if err != nil {
			return SearchPage{}, err
		}
		if closeErr != nil {
			return SearchPage{}, closeErr
		}
		// Existing local orders have exactly one item (the approved suggestion).
		if len(out.Items[n].Items) != 1 {
			return SearchPage{}, ErrConflict
		}
	}
	out.HasMore = offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-offset
	return out, tx.Commit()
}
