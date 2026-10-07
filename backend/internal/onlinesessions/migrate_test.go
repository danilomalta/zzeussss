package onlinesessions

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

func TestMigrationPreservesFiveAndAppliesSixAtomically(t *testing.T) {
	for _, state := range []string{"new", "upgrade", "already", "future", "changed-five", "changed-six", "gap", "ddl-failure", "marker-failure", "zero-marker", "commit-failure"} {
		t.Run(state, func(t *testing.T) {
			db, m, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			m.ExpectBegin()
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec("CREATE TABLE IF NOT EXISTS online_security_migrations").WillReturnResult(sqlmock.NewResult(0, 1))
			rows := sqlmock.NewRows([]string{"version", "checksum"})
			count := 0
			switch state {
			case "upgrade", "ddl-failure", "marker-failure", "zero-marker", "commit-failure":
				rows.AddRow(5, migrationDigest(securityMigrations[0].sql))
				count = 1
			case "already", "future", "changed-six":
				rows.AddRow(5, migrationDigest(securityMigrations[0].sql))
				hash := schemaDigest()
				if state == "changed-six" {
					hash = "changed"
				}
				rows.AddRow(6, hash)
				count = 2
				if state == "future" {
					rows.AddRow(7, "future")
				}
			case "changed-five":
				rows.AddRow(5, "changed")
			case "gap":
				rows.AddRow(6, schemaDigest())
			}
			m.ExpectQuery("SELECT version, checksum FROM online_security_migrations ORDER BY version").WillReturnRows(rows)
			validHistory := state != "future" && state != "changed-five" && state != "changed-six" && state != "gap"
			if validHistory {
				for _, step := range securityMigrations[count:] {
					ddl := m.ExpectExec(regexp.QuoteMeta(step.sql))
					if state == "ddl-failure" {
						ddl.WillReturnError(errors.New("private"))
						break
					}
					ddl.WillReturnResult(sqlmock.NewResult(0, 1))
					marker := m.ExpectExec("INSERT INTO online_security_migrations").WithArgs(step.version, migrationDigest(step.sql))
					if state == "marker-failure" {
						marker.WillReturnError(errors.New("private"))
						break
					}
					n := int64(1)
					if state == "zero-marker" {
						n = 0
					}
					marker.WillReturnResult(sqlmock.NewResult(0, n))
					if n == 0 {
						break
					}
				}
			}
			good := state == "new" || state == "upgrade" || state == "already"
			if good {
				m.ExpectCommit()
			} else if state == "commit-failure" {
				m.ExpectCommit().WillReturnError(errors.New("private"))
			} else {
				m.ExpectRollback()
			}
			err = Migrate(context.Background(), db)
			if (err == nil) != good {
				t.Fatal("migration outcome incorrect", err)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestCheckSchemaRequiresCompleteHistoryAndPasswordTable(t *testing.T) {
	for _, state := range []string{"complete", "only-five", "changed-five", "missing-table"} {
		t.Run(state, func(t *testing.T) {
			db, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			rows := sqlmock.NewRows([]string{"version", "checksum"})
			hash := migrationDigest(securityMigrations[0].sql)
			if state == "changed-five" {
				hash = "changed"
			}
			rows.AddRow(5, hash)
			if state != "only-five" {
				rows.AddRow(6, schemaDigest())
			}
			m.ExpectQuery("SELECT version, checksum").WillReturnRows(rows)
			if state == "complete" || state == "missing-table" {
				q := m.ExpectQuery("SELECT .*online_password_changes")
				if state == "missing-table" {
					q.WillReturnError(errors.New("private"))
				} else {
					q.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				}
			}
			err := CheckSchema(context.Background(), db)
			if (err == nil) != (state == "complete") {
				t.Fatal("schema outcome incorrect")
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
