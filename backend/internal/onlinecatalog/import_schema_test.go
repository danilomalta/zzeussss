package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingUndo(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 11, undoChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestImportMigrationRejectsFutureOrChangedHistory(t *testing.T) {
	for _, version := range []int{12, 13} {
		s, m := mockStore(t)
		existingUndo(m)
		m.ExpectBegin()
		m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_import_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(version, "changed"))
		m.ExpectRollback()
		if MigrateImports(context.Background(), s.DB) != ErrUnavailable {
			t.Fatal("history accepted")
		}
	}
}
func TestImportMigrationRepeatedDoesNotExecuteDDLAgain(t *testing.T) {
	s, m := mockStore(t)
	existingUndo(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_import_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(12, importChecksum()))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 12, importChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateImports(context.Background(), s.DB) != nil {
		t.Fatal("repeated migration failed")
	}
}

func TestImportMigrationFreshIsTransactionalAndVerified(t *testing.T) {
	s, m := mockStore(t)
	existingUndo(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_import_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}))
	m.ExpectExec(`CREATE TABLE online_catalog_imports`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO online_catalog_import_migrations`).WithArgs(importChecksum()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 12, importChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateImports(context.Background(), s.DB) != nil {
		t.Fatal("fresh migration failed")
	}
}
