package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingOffers49(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"n", "v", "h"}).AddRow(1, 21, supplierOffersChecksum()))
	m.ExpectQuery("SELECT to_regclass").WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestPurchaseDraftsMigrationStrictHistoryAndAtomicDDL(t *testing.T) {
	for _, state := range []string{"fresh", "same", "future", "changed"} {
		s, m := qualityFixture(t)
		existingOffers49(m)
		m.ExpectBegin()
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec("CREATE TABLE IF NOT EXISTS online_purchase_migrations").WillReturnResult(sqlmock.NewResult(0, 1))
		rows := sqlmock.NewRows([]string{"version", "checksum"})
		h := purchaseDraftsChecksum()
		if state == "same" {
			rows.AddRow(22, h)
		}
		if state == "future" {
			rows.AddRow(23, h)
		}
		if state == "changed" {
			rows.AddRow(22, "changed")
		}
		m.ExpectQuery("SELECT version,checksum").WillReturnRows(rows)
		if state == "future" || state == "changed" {
			m.ExpectRollback()
		} else {
			if state == "fresh" {
				m.ExpectExec("CREATE TABLE online_purchase_drafts").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("INSERT INTO online_purchase_migrations").WithArgs(h).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"n", "v", "h"}).AddRow(1, 22, h))
			m.ExpectQuery("SELECT to_regclass").WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
			m.ExpectCommit()
		}
		e := MigratePurchaseDrafts(context.Background(), s.DB)
		if (state == "fresh" || state == "same") && e != nil || (state == "future" || state == "changed") && e != ErrUnavailable {
			t.Fatal(state, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
