package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingCreation(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 9, creationChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestBatchesMigrationRejectsFutureOrChangedHistory(t *testing.T) {
	for _, version := range []int{10, 11} {
		s, m := mockStore(t)
		existingCreation(m)
		m.ExpectBegin()
		m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_batch_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(version, "changed"))
		m.ExpectRollback()
		if MigrateBatches(context.Background(), s.DB) != ErrUnavailable {
			t.Fatal("history accepted")
		}
	}
}
func TestBatchesMigrationRepeatedDoesNotExecuteDDLAgain(t *testing.T) {
	s, m := mockStore(t)
	existingCreation(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_batch_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(10, batchChecksum()))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 10, batchChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateBatches(context.Background(), s.DB) != nil {
		t.Fatal("repeated migration failed")
	}
}

func TestBatchesMigrationFreshIsTransactionalAndVerified(t *testing.T) {
	s, m := mockStore(t)
	existingCreation(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_batch_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}))
	m.ExpectExec(`CREATE TABLE online_catalog_batches`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO online_catalog_batch_migrations`).WithArgs(batchChecksum()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 10, batchChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateBatches(context.Background(), s.DB) != nil {
		t.Fatal("fresh migration failed")
	}
}
