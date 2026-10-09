package onlinecatalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"titansystem-backend/db/migrations"
)

// Migrate is explicit maintenance, never an HTTP startup side effect.
// Its history is independent of the authentication migration sequence.
func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(748203081)`); err != nil {
		return ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS online_catalog_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,installed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp())`); err != nil {
		return ErrUnavailable
	}
	rows, err := tx.QueryContext(ctx, `SELECT version,checksum FROM online_catalog_migrations ORDER BY version`)
	if err != nil {
		return ErrUnavailable
	}
	count := 0
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(migrations.CatalogManagementSQL)))
	for rows.Next() {
		var v int
		var h string
		if rows.Scan(&v, &h) != nil || v != 8 || h != checksum {
			rows.Close()
			return ErrUnavailable
		}
		count++
	}
	err = rows.Err()
	rows.Close()
	if err != nil || count > 1 {
		return ErrUnavailable
	}
	if count == 0 {
		if _, err = tx.ExecContext(ctx, migrations.CatalogManagementSQL); err != nil {
			return ErrUnavailable
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO online_catalog_migrations(version,checksum) VALUES(8,$1)`, checksum); err != nil {
			return ErrUnavailable
		}
	}
	if err = check(ctx, tx); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}

type schemaReader interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func check(ctx context.Context, db schemaReader) error {
	var count int
	var checksum string
	if db.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(checksum),'') FROM online_catalog_migrations WHERE version=8`).Scan(&count, &checksum) != nil || count != 1 || checksum != fmt.Sprintf("%x", sha256.Sum256([]byte(migrations.CatalogManagementSQL))) {
		return ErrUnavailable
	}
	var total int
	if db.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_migrations`).Scan(&total) != nil || total != 1 {
		return ErrUnavailable
	}
	var ok bool
	if db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='products' AND column_name='catalog_version' AND data_type='bigint') AND to_regclass('online_catalog_operations') IS NOT NULL AND to_regclass('online_catalog_outbox') IS NOT NULL`).Scan(&ok) != nil || !ok {
		return ErrUnavailable
	}
	return nil
}
func CheckSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	return check(ctx, db)
}
