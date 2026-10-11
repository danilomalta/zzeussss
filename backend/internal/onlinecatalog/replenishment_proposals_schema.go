package onlinecatalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"titansystem-backend/db/migrations"
)

func replenishmentProposalsChecksum() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(migrations.ReplenishmentProposalsSQL)))
}
func MigrateReplenishmentProposals(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	if CheckStockSettings(ctx, db) != nil {
		return ErrUnavailable
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(748203092)`); e != nil {
		return ErrUnavailable
	}
	if _, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS online_replenishment_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL)`); e != nil {
		return ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT version,checksum FROM online_replenishment_migrations ORDER BY version`)
	if e != nil {
		return ErrUnavailable
	}
	n := 0
	for rows.Next() {
		var v int
		var h string
		if rows.Scan(&v, &h) != nil || v != 19 || h != replenishmentProposalsChecksum() {
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
		if _, e = tx.ExecContext(ctx, migrations.ReplenishmentProposalsSQL); e != nil {
			return ErrUnavailable
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO online_replenishment_migrations(version,checksum) VALUES(19,$1)`, replenishmentProposalsChecksum()); e != nil {
			return ErrUnavailable
		}
	}
	if checkReplenishmentProposals(ctx, tx) != nil {
		return ErrUnavailable
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}
func checkReplenishmentProposals(ctx context.Context, db schemaReader) error {
	var n int
	var v int
	var h string
	if db.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(version),0),COALESCE(min(checksum),'') FROM online_replenishment_migrations`).Scan(&n, &v, &h) != nil || n != 1 || v != 19 || h != replenishmentProposalsChecksum() {
		return ErrUnavailable
	}
	var ok bool
	if db.QueryRowContext(ctx, `SELECT to_regclass('online_replenishment_proposals') IS NOT NULL AND to_regclass('online_replenishment_operations') IS NOT NULL AND to_regclass('online_replenishment_outbox') IS NOT NULL`).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	return nil
}
func CheckReplenishmentProposals(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	return checkReplenishmentProposals(ctx, db)
}
