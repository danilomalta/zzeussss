package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingBatches(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 10, batchChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestUndoMigrationRejectsFutureOrChangedHistory(t *testing.T) {
	for _, version := range []int{11, 12} {
		s, m := mockStore(t)
		existingBatches(m)
		m.ExpectBegin()
		m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_undo_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(version, "changed"))
		m.ExpectRollback()
		if MigrateUndo(context.Background(), s.DB) != ErrUnavailable {
			t.Fatal("history accepted")
		}
	}
}
func TestUndoMigrationRepeatedDoesNotExecuteDDLAgain(t *testing.T) {
	s, m := mockStore(t)
	existingBatches(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_undo_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(11, undoChecksum()))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 11, undoChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateUndo(context.Background(), s.DB) != nil {
		t.Fatal("repeated migration failed")
	}
}

func TestUndoMigrationFreshIsTransactionalAndVerified(t *testing.T) {
	s, m := mockStore(t)
	existingBatches(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_undo_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}))
	m.ExpectExec(`CREATE TABLE online_catalog_undos`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO online_catalog_undo_migrations`).WithArgs(undoChecksum()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 11, undoChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateUndo(context.Background(), s.DB) != nil {
		t.Fatal("fresh migration failed")
	}
}
