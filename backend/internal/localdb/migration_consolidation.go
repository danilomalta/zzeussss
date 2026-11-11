package localdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"titansystem-backend/internal/sqlitewrite"
)

// The alternative branch shipped the exact origins script as 86 while the
// consolidated branch used 86/87 for auth budgets/supplier policy. No general
// checksum bypass exists: validateSchemaHistory accepts only that complete,
// byte-identical 1..86 history. Renumber its last ledger entry and install the
// two missing scripts in ONE writer transaction; retain the original date and
// origins table/data. Unknown histories and future schemas still fail closed.
func consolidateOriginsFork(ctx context.Context, db *sql.DB) error {
	legacy, err := validateSchemaHistory(ctx, db)
	if err != nil || !legacy {
		return err
	}
	return sqlitewrite.Run(ctx, db, errors.New("conflito de revisão do esquema"), func(tx *sql.Tx) error {
		legacy, err := validateSchemaHistory(ctx, tx)
		if err != nil || !legacy {
			return err
		}
		origin, err := migrations.ReadFile("migrations/0088_fiscal_nfe_origins.sql")
		if err != nil {
			return err
		}
		fingerprint := fmt.Sprintf("%x", sha256.Sum256(origin))
		result, err := tx.ExecContext(ctx, `UPDATE schema_migrations SET version=88 WHERE version=86 AND checksum=?`, fingerprint)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			return errors.New("conflito de revisão do esquema")
		}
		for _, script := range []struct {
			version int
			name    string
		}{{86, "0086_auth_attempt_budgets.sql"}, {87, "0087_supplier_contact_policy.sql"}} {
			body, err := migrations.ReadFile("migrations/" + script.name)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, string(body)); err != nil {
				return err
			}
			checksum := fmt.Sprintf("%x", sha256.Sum256(body))
			if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,checksum,applied_at) VALUES(?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`, script.version, checksum); err != nil {
				return err
			}
		}
		_, err = validateSchemaHistory(ctx, tx)
		return err
	})
}
