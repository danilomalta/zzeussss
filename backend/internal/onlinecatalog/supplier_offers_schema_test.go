package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingSuppliers48(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"n", "v", "h"}).AddRow(1, 20, suppliersChecksum()))
	m.ExpectQuery("SELECT to_regclass").WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestSupplierOffersMigrationStrictHistoryAndAtomicDDL(t *testing.T) {
	for _, state := range []string{"fresh", "same", "future", "changed"} {
		s, m := qualityFixture(t)
		existingSuppliers48(m)
		m.ExpectBegin()
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec("CREATE TABLE IF NOT EXISTS online_supplier_offer_migrations").WillReturnResult(sqlmock.NewResult(0, 1))
		rows := sqlmock.NewRows([]string{"version", "checksum"})
		h := supplierOffersChecksum()
		if state == "same" {
			rows.AddRow(21, h)
		}
		if state == "future" {
			rows.AddRow(22, h)
		}
		if state == "changed" {
			rows.AddRow(21, "changed")
		}
		m.ExpectQuery("SELECT version,checksum").WillReturnRows(rows)
		if state == "future" || state == "changed" {
			m.ExpectRollback()
		} else {
			if state == "fresh" {
				m.ExpectExec("CREATE TABLE online_supplier_offers").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("INSERT INTO online_supplier_offer_migrations").WithArgs(h).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"n", "v", "h"}).AddRow(1, 21, h))
			m.ExpectQuery("SELECT to_regclass").WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
			m.ExpectCommit()
		}
		e := MigrateSupplierOffers(context.Background(), s.DB)
		if (state == "fresh" || state == "same") && e != nil || (state == "future" || state == "changed") && e != ErrUnavailable {
			t.Fatal(state, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
