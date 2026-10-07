package onlinesessions

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"

	"titansystem-backend/db/migrations"
)

var ErrSchema = errors.New("esquema de sessões online ausente ou incompatível; execute titan-online migrate-sessions")

func schemaDigest() string {
	h := sha256.Sum256([]byte(migrations.OnlineSessionsSQL))
	return hex.EncodeToString(h[:])
}

// Migrate installs just the new session schema, atomically and with a checksum.
// It never applies the destructive historical initialization file.
func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrUnavailable
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(748203080)`); err != nil {
		return ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS online_security_migrations (version INTEGER PRIMARY KEY, checksum CHAR(64) NOT NULL, installed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return ErrSchema
	}
	var version int
	var digest string
	err = tx.QueryRowContext(ctx, `SELECT version, checksum FROM online_security_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &digest)
	if err == nil {
		if version != 5 || digest != schemaDigest() {
			return ErrSchema
		}
		return safeCommit(tx)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ErrSchema
	}
	if _, err = tx.ExecContext(ctx, migrations.OnlineSessionsSQL); err != nil {
		return ErrSchema
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO online_security_migrations(version, checksum) VALUES (5, $1)`, schemaDigest())
	if err != nil {
		return ErrSchema
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return ErrSchema
	}
	return safeCommit(tx)
}

func CheckSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrSchema
	}
	var version int
	var digest string
	if err := db.QueryRowContext(ctx, `SELECT version, checksum FROM online_security_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &digest); err != nil || version != 5 || digest != schemaDigest() {
		return ErrSchema
	}
	// Prepare a zero-row query so missing columns/tables fail before serving.
	rows, err := db.QueryContext(ctx, `SELECT s.id, s.tenant_id, s.user_id, s.role, s.expires_at, s.revoked_at, r.digest, r.consumed_at, a.event FROM online_sessions s LEFT JOIN online_refresh_tokens r ON r.session_id = s.id LEFT JOIN online_session_audit a ON a.session_id = s.id WHERE FALSE`)
	if err != nil {
		return ErrSchema
	}
	if rows.Close() != nil {
		return ErrSchema
	}
	return nil
}
