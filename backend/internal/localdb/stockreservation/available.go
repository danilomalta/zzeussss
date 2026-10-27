// Package stockreservation provides one transactional reservation rule to all
// stock writers. It neither authenticates callers nor writes reservations.
package stockreservation

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
)

var ErrUnavailable = errors.New("saldo comprometido por reservas ou inconsistente")

func HeldTx(ctx context.Context, tx *sql.Tx, tenant, store, product, location string) (int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT i.quantity_milli FROM production_material_items i JOIN production_material_reservations r ON r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.id=i.reservation_id WHERE i.tenant_id=? AND i.store_id=? AND i.product_id=? AND r.location_id=? AND r.status='active'`, tenant, store, product, location)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	total := new(big.Int)
	for rows.Next() {
		var quantity int64
		if err = rows.Scan(&quantity); err != nil {
			return 0, err
		}
		if quantity < 1 {
			return 0, ErrUnavailable
		}
		total.Add(total, big.NewInt(quantity))
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	if !total.IsInt64() {
		return 0, ErrUnavailable
	}
	return total.Int64(), nil
}

func FreeTx(ctx context.Context, tx *sql.Tx, tenant, store, product, location string, recorded int64) (int64, error) {
	held, err := HeldTx(ctx, tx, tenant, store, product, location)
	if err != nil {
		return 0, err
	}
	if recorded < 0 || held > recorded {
		return 0, ErrUnavailable
	}
	return recorded - held, nil
}
