package onlinecatalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"titansystem-backend/db/migrations"
)

func purchaseDraftsChecksum() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(migrations.PurchaseDraftsSQL)))
}
func MigratePurchaseDrafts(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	if CheckSupplierOffers(ctx, db) != nil {
		return ErrUnavailable
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(748203095)`); e != nil {
		return ErrUnavailable
	}
	if _, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS online_purchase_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL)`); e != nil {
		return ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT version,checksum FROM online_purchase_migrations ORDER BY version`)
	if e != nil {
		return ErrUnavailable
	}
	n := 0
	for rows.Next() {
		var v int
		var h string
		if rows.Scan(&v, &h) != nil || v != 22 || h != purchaseDraftsChecksum() {
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
		if _, e = tx.ExecContext(ctx, migrations.PurchaseDraftsSQL); e != nil {
			return ErrUnavailable
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO online_purchase_migrations(version,checksum) VALUES(22,$1)`, purchaseDraftsChecksum()); e != nil {
			return ErrUnavailable
		}
	}
	if checkPurchaseDrafts(ctx, tx) != nil {
		return ErrUnavailable
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}
func checkPurchaseDrafts(ctx context.Context, db schemaReader) error {
	var n int
	var v int
	var h string
	if db.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(version),0),COALESCE(min(checksum),'') FROM online_purchase_migrations`).Scan(&n, &v, &h) != nil || n != 1 || v != 22 || h != purchaseDraftsChecksum() {
		return ErrUnavailable
	}
	var ok bool
	if db.QueryRowContext(ctx, `SELECT to_regclass('online_purchase_drafts') IS NOT NULL AND to_regclass('online_purchase_operations') IS NOT NULL AND to_regclass('online_purchase_outbox') IS NOT NULL`).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	return nil
}
func CheckPurchaseDrafts(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	return checkPurchaseDrafts(ctx, db)
}
