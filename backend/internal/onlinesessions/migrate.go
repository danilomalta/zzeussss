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

type securityMigration struct {
	version int
	sql     string
}

var securityMigrations = []securityMigration{{5, migrations.OnlineSessionsSQL}, {6, migrations.OnlinePasswordChangesSQL}, {7, migrations.OnlineRecoveryKeysSQL}}

func migrationDigest(script string) string {
	h := sha256.Sum256([]byte(script))
	return hex.EncodeToString(h[:])
}
func schemaDigest() string { return migrationDigest(migrations.OnlinePasswordChangesSQL) }

// readHistory requires an exact prefix of the embedded incremental migrations.
// A missing earlier marker, changed checksum, or unknown future version fails.
func readHistory(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) (int, error) {
	rows, err := q.QueryContext(ctx, `SELECT version, checksum FROM online_security_migrations ORDER BY version`)
	if err != nil {
		return 0, ErrSchema
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var v int
		var hash string
		if rows.Scan(&v, &hash) != nil || count >= len(securityMigrations) {
			return 0, ErrSchema
		}
		expected := securityMigrations[count]
		if v != expected.version || hash != migrationDigest(expected.sql) {
			return 0, ErrSchema
		}
		count++
	}
	if rows.Err() != nil {
		return 0, ErrSchema
	}
	return count, nil
}

// Migrate applies only the explicitly embedded additive security migrations.
// All missing steps and their markers commit together; old checksums are fixed.
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
	count, err := readHistory(ctx, tx)
	if err != nil {
		return err
	}
	for _, step := range securityMigrations[count:] {
		if _, err = tx.ExecContext(ctx, step.sql); err != nil {
			return ErrSchema
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO online_security_migrations(version, checksum) VALUES ($1, $2)`, step.version, migrationDigest(step.sql))
		if err != nil {
			return ErrSchema
		}
		if n, e := result.RowsAffected(); e != nil || n != 1 {
			return ErrSchema
		}
	}
	return safeCommit(tx)
}

func CheckSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return ErrSchema
	}
	count, err := readHistory(ctx, db)
	if err != nil || count != len(securityMigrations) {
		return ErrSchema
	}
	// Prepare a zero-row query so missing columns/tables fail before serving.
	rows, err := db.QueryContext(ctx, `SELECT s.id, s.tenant_id, s.user_id, s.role, s.expires_at, s.revoked_at, r.digest, r.consumed_at, a.event, p.revoked_count FROM online_sessions s LEFT JOIN online_refresh_tokens r ON r.session_id = s.id LEFT JOIN online_session_audit a ON a.session_id = s.id LEFT JOIN online_password_changes p ON p.actor_session_id = s.id WHERE FALSE`)
	if err != nil {
		return ErrSchema
	}
	if rows.Close() != nil {
		return ErrSchema
	}
	rows, err = db.QueryContext(ctx, `SELECT k.digest, k.credential_digest, k.expires_at, k.consumed_at, a.event FROM online_recovery_keys k LEFT JOIN online_recovery_audit a ON a.key_id=k.id AND a.tenant_id=k.tenant_id AND a.user_id=k.user_id WHERE FALSE`)
	if err != nil {
		return ErrSchema
	}
	if rows.Close() != nil {
		return ErrSchema
	}
	return nil
}
