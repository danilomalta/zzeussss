package onlinecatalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"titansystem-backend/db/migrations"
)

func supplierOffersChecksum() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(migrations.SupplierOffersSQL)))
}
func MigrateSupplierOffers(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	if CheckSuppliers(ctx, db) != nil {
		return ErrUnavailable
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(748203094)`); e != nil {
		return ErrUnavailable
	}
	if _, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS online_supplier_offer_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL)`); e != nil {
		return ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT version,checksum FROM online_supplier_offer_migrations ORDER BY version`)
	if e != nil {
		return ErrUnavailable
	}
	n := 0
	for rows.Next() {
		var v int
		var h string
		if rows.Scan(&v, &h) != nil || v != 21 || h != supplierOffersChecksum() {
			rows.Close()
			return ErrUnavailable
		}
		n++
	}
	e = rows.Err()
	rows.Close()
	if e != nil || n > 1 {
		return ErrUnavailable
	}
	if n == 0 {
		if _, e = tx.ExecContext(ctx, migrations.SupplierOffersSQL); e != nil {
			return ErrUnavailable
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO online_supplier_offer_migrations(version,checksum) VALUES(21,$1)`, supplierOffersChecksum()); e != nil {
			return ErrUnavailable
		}
	}
	if checkSupplierOffers(ctx, tx) != nil {
		return ErrUnavailable
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}
func checkSupplierOffers(ctx context.Context, db schemaReader) error {
	var n int
	var v int
	var h string
	if db.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(version),0),COALESCE(min(checksum),'') FROM online_supplier_offer_migrations`).Scan(&n, &v, &h) != nil || n != 1 || v != 21 || h != supplierOffersChecksum() {
		return ErrUnavailable
	}
	var ok bool
	if db.QueryRowContext(ctx, `SELECT to_regclass('online_supplier_offers') IS NOT NULL AND to_regclass('online_supplier_offer_operations') IS NOT NULL AND to_regclass('online_supplier_offer_outbox') IS NOT NULL`).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	return nil
}
func CheckSupplierOffers(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	return checkSupplierOffers(ctx, db)
}
