package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingAdjustments(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 13, adjustmentChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestBarcodeMigrationRejectsFutureOrChangedHistory(t *testing.T) {
	for _, version := range []int{14, 15} {
		s, m := mockStore(t)
		existingAdjustments(m)
		m.ExpectBegin()
		m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_barcode_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(version, "changed"))
		m.ExpectRollback()
		if MigrateBarcodes(context.Background(), s.DB) != ErrUnavailable {
			t.Fatal("history accepted")
		}
	}
}
func TestBarcodeMigrationRepeatedDoesNotExecuteDDLAgain(t *testing.T) {
	s, m := mockStore(t)
	existingAdjustments(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_barcode_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(14, barcodeChecksum()))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 14, barcodeChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateBarcodes(context.Background(), s.DB) != nil {
		t.Fatal("repeated migration failed")
	}
}

func TestBarcodeMigrationFreshIsTransactionalAndVerified(t *testing.T) {
	s, m := mockStore(t)
	existingAdjustments(m)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_barcode_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}))
	m.ExpectExec(`CREATE TABLE online_catalog_barcodes`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO online_catalog_barcode_migrations`).WithArgs(barcodeChecksum()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "version", "checksum"}).AddRow(1, 14, barcodeChecksum()))
	m.ExpectQuery(`SELECT to_regclass`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if MigrateBarcodes(context.Background(), s.DB) != nil {
		t.Fatal("fresh migration failed")
	}
}
