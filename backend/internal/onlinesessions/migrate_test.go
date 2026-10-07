package onlinesessions

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMigrationIsAtomicAndNeverRunsHistoricalInitialization(t *testing.T) {
	for _, state := range []string{"new", "already", "future", "changed", "ddl-failure", "marker-failure"} {
		t.Run(state, func(t *testing.T) {
			db, m, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			m.ExpectBegin()
			m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_security_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
			rows := sqlmock.NewRows([]string{"version", "checksum"})
			version := 5
			digest := schemaDigest()
			if state == "future" {
				version = 6
			}
			if state == "changed" {
				digest = "changed"
			}
			if state == "already" || state == "future" || state == "changed" {
				rows.AddRow(version, digest)
			}
			m.ExpectQuery(`SELECT version, checksum`).WillReturnRows(rows)
			if state == "new" || state == "ddl-failure" || state == "marker-failure" {
				d := m.ExpectExec(`(?s)^-- Incremental only.*CREATE UNIQUE INDEX online_users_tenant_identity.*CREATE TABLE online_sessions.*CREATE TABLE online_refresh_tokens.*CREATE TABLE online_session_audit`)
				if state == "ddl-failure" {
					d.WillReturnError(errors.New("database private detail"))
				} else {
					d.WillReturnResult(sqlmock.NewResult(0, 1))
					e := m.ExpectExec(`INSERT INTO online_security_migrations`).WithArgs(schemaDigest())
					if state == "marker-failure" {
						e.WillReturnError(errors.New("private"))
					} else {
						e.WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
			}
			valid := state == "new" || state == "already"
			if valid {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			err = Migrate(context.Background(), db)
			if (err == nil) != valid {
				t.Fatal("migração incorreta")
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
