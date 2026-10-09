package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingImports(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 12, importChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestAdjustmentMigrationRejectsFutureOrChangedHistory(t *testing.T) {
	for _, version := range []int{13, 14} {
		s, m := mockStore(t)
		existingImports(m)
		m.ExpectBegin()
		m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_adjustment_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(version, "changed"))
		m.ExpectRollback()
		if MigrateAdjustments(context.Background(), s.DB) != ErrUnavailable {
			t.Fatal("history accepted")
		}
	}
}
func TestAdjustmentMigrationRepeatedDoesNotExecuteDDLAgain(t *testing.T) {
	s, m := mockStore(t)
	existingImports(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_adjustment_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(13, adjustmentChecksum()))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 13, adjustmentChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateAdjustments(context.Background(), s.DB) != nil {
		t.Fatal("repeated migration failed")
	}
}

func TestAdjustmentMigrationFreshIsTransactionalAndVerified(t *testing.T) {
	s, m := mockStore(t)
	existingImports(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_adjustment_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}))
	m.ExpectExec(`CREATE TABLE online_catalog_adjustments`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO online_catalog_adjustment_migrations`).WithArgs(adjustmentChecksum()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 13, adjustmentChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateAdjustments(context.Background(), s.DB) != nil {
		t.Fatal("fresh migration failed")
	}
}
