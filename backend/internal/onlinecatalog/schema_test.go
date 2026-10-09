package onlinecatalog

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
	"titansystem-backend/db/migrations"
)

func TestMigrationRejectsChangedOrFutureHistoryBeforeDDL(t *testing.T) {
	for _, version := range []int{8, 9} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			s, m := mockStore(t)
			m.ExpectBegin()
			m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(version, "changed"))
			m.ExpectRollback()
			if Migrate(context.Background(), s.DB) != ErrUnavailable {
				t.Fatal("unsupported history accepted")
			}
		})
	}
}
func TestMigrationDDLFailureRollsBackMarkerAndSchema(t *testing.T) {
	s, m := mockStore(t)
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}))
	m.ExpectExec(`ALTER TABLE products ADD COLUMN`).WillReturnError(errors.New("interrupted"))
	m.ExpectRollback()
	if Migrate(context.Background(), s.DB) != ErrUnavailable {
		t.Fatal("failed migration accepted")
	}
}
func TestRepeatedMigrationChecksSchemaWithoutReapplying(t *testing.T) {
	s, m := mockStore(t)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(migrations.CatalogManagementSQL)))
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`CREATE TABLE IF NOT EXISTS online_catalog_migrations`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT version,checksum`).WillReturnRows(sqlmock.NewRows([]string{"version", "checksum"}).AddRow(8, hash))
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"count", "checksum"}).AddRow(1, hash))
	m.ExpectQuery(`SELECT count\(\*\) FROM online_catalog_migrations`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	m.ExpectCommit()
	if e := Migrate(context.Background(), s.DB); e != nil {
		t.Fatal(e)
	}
}
