package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingDiscountReviews39(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"n", "v", "h"}).AddRow(1, 16, discountReviewChecksum()))
	m.ExpectQuery("SELECT to_regclass").WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestCostMigrationStrictHistoryAndAtomicDDL(t *testing.T) {
	for _, state := range []string{"fresh", "same", "future", "changed"} {
		s, m := qualityFixture(t)
		existingDiscountReviews39(m)
		m.ExpectBegin()
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec("CREATE TABLE IF NOT EXISTS online_cost_migrations").WillReturnResult(sqlmock.NewResult(0, 1))
		rows := sqlmock.NewRows([]string{"version", "checksum"})
		h := costChecksum()
		if state == "same" {
			rows.AddRow(17, h)
		}
		if state == "future" {
			rows.AddRow(18, h)
		}
		if state == "changed" {
			rows.AddRow(17, "changed")
		}
		m.ExpectQuery("SELECT version,checksum").WillReturnRows(rows)
		if state == "future" || state == "changed" {
			m.ExpectRollback()
		} else {
			if state == "fresh" {
				m.ExpectExec("CREATE TABLE online_product_costs").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("INSERT INTO online_cost_migrations").WithArgs(h).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"n", "v", "h"}).AddRow(1, 17, h))
			m.ExpectQuery("SELECT to_regclass").WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
			m.ExpectCommit()
		}
		e := MigrateCosts(context.Background(), s.DB)
		if (state == "fresh" || state == "same") && e != nil || (state == "future" || state == "changed") && e != ErrUnavailable {
			t.Fatal(state, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
