package stock

import (
	"context"
	"database/sql"
	"errors"

	"titansystem-backend/internal/localdb/identity"
)

type ActiveReservation struct {
	ReservationID string `json:"reservation_id"`
	OrderID       string `json:"order_id"`
	VersionID     string `json:"version_id"`
	ResponsibleID string `json:"responsible_id"`
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
	Unit          string `json:"unit"`
	QuantityMilli int64  `json:"quantity_milli"`
}
type ReservationPage struct {
	Balance    Availability        `json:"balance"`
	TotalCount int64               `json:"total_count"`
	Offset     int64               `json:"offset"`
	Limit      int                 `json:"limit"`
	HasMore    bool                `json:"has_more"`
	Items      []ActiveReservation `json:"items"`
}

const activeReservationFrom = ` FROM production_material_items i JOIN production_material_reservations r ON r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.id=i.reservation_id`
const activeReservationWhere = ` WHERE i.tenant_id=? AND i.store_id=? AND i.product_id=? AND r.location_id=? AND r.status='active'`

func ActiveReservations(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, product, location string, offset int64) (ReservationPage, error) {
	if !validAvailabilityID(product) || !validAvailabilityID(location) || offset < 0 || offset > MaxAvailabilityQuantity {
		return ReservationPage{}, ErrInvalidOperation
	}
	if db == nil {
		return ReservationPage{}, errors.New("banco local indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ReservationPage{}, err
	}
	defer tx.Rollback()
	for _, permission := range []identity.Permission{identity.ManageStock, identity.ManageProduction} {
		if err = identity.CanOperateTx(ctx, tx, a, d, permission); err != nil {
			return ReservationPage{}, err
		}
	}
	out := ReservationPage{Offset: offset, Limit: 50, Items: []ActiveReservation{}}
	out.Balance, err = availableBalanceTx(ctx, tx, a, product, location)
	if err != nil {
		return ReservationPage{}, err
	}
	args := []any{a.TenantID, a.StoreID, product, location}
	if err = tx.QueryRowContext(ctx, `SELECT count(*)`+activeReservationFrom+activeReservationWhere, args...).Scan(&out.TotalCount); err != nil {
		return ReservationPage{}, err
	}
	if out.TotalCount < 0 || out.TotalCount > MaxAvailabilityQuantity || (out.TotalCount == 0 && out.Balance.ReservedMilli != 0) {
		return ReservationPage{}, ErrAvailability
	}
	pageArgs := append(append([]any{}, args...), offset)
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.order_id,o.version_id,o.responsible_id,r.created_by,r.created_at,i.unit,i.quantity_milli,o.location_id,o.status,EXISTS(SELECT 1 FROM production_results p WHERE p.tenant_id=r.tenant_id AND p.store_id=r.store_id AND p.order_id=r.order_id)`+activeReservationFrom+` JOIN production_orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.id=r.order_id`+activeReservationWhere+` ORDER BY r.created_at,r.id LIMIT 50 OFFSET ?`, pageArgs...)
	if err != nil {
		return ReservationPage{}, err
	}
	defer rows.Close()
	pageQuantity := int64(0)
	for rows.Next() {
		var item ActiveReservation
		var plannedLocation, status string
		var completed bool
		if err = rows.Scan(&item.ReservationID, &item.OrderID, &item.VersionID, &item.ResponsibleID, &item.CreatedBy, &item.CreatedAt, &item.Unit, &item.QuantityMilli, &plannedLocation, &status, &completed); err != nil {
			return ReservationPage{}, err
		}
		if plannedLocation != location || status != "approved" || completed || item.Unit != out.Balance.Unit || item.QuantityMilli < 1 || item.QuantityMilli > out.Balance.ReservedMilli-pageQuantity {
			return ReservationPage{}, ErrAvailability
		}
		for _, id := range []string{item.ReservationID, item.OrderID, item.VersionID, item.ResponsibleID, item.CreatedBy} {
			if !validAvailabilityID(id) {
				return ReservationPage{}, ErrAvailability
			}
		}
		pageQuantity += item.QuantityMilli
		out.Items = append(out.Items, item)
	}
	if err = rows.Err(); err != nil {
		return ReservationPage{}, err
	}
	if err = rows.Close(); err != nil {
		return ReservationPage{}, err
	}
	out.HasMore = offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-offset
	return out, tx.Commit()
}
