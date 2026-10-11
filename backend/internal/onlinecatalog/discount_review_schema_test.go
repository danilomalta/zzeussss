package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func existingBarcodeBatches36(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"n", "v", "h"}).AddRow(1, 15, barcodeBatchChecksum()))
	m.ExpectQuery("SELECT to_regclass").WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}
func TestDiscountReviewMigrationStrictHistoryAndAtomicDDL(t *testing.T) {
	for _, state := range []string{"fresh", "same", "future", "changed"} {
		s, m := qualityFixture(t)
		existingBarcodeBatches36(m)
		m.ExpectBegin()
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectExec("CREATE TABLE IF NOT EXISTS online_discount_review_migrations").WillReturnResult(sqlmock.NewResult(0, 1))
		rows := sqlmock.NewRows([]string{"version", "checksum"})
		h := discountReviewChecksum()
		if state == "same" {
			rows.AddRow(16, h)
		}
		if state == "future" {
			rows.AddRow(17, h)
		}
		if state == "changed" {
			rows.AddRow(16, "changed")
		}
		m.ExpectQuery("SELECT version,checksum").WillReturnRows(rows)
		if state == "future" || state == "changed" {
			m.ExpectRollback()
		} else {
			if state == "fresh" {
				m.ExpectExec("CREATE UNIQUE INDEX discount_suggestions").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("INSERT INTO online_discount_review_migrations").WithArgs(h).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			m.ExpectQuery(`SELECT count\(\*\),COALESCE`).WillReturnRows(sqlmock.NewRows([]string{"n", "v", "h"}).AddRow(1, 16, h))
			m.ExpectQuery("SELECT to_regclass").WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
			m.ExpectCommit()
		}
		e := MigrateDiscountReviews(context.Background(), s.DB)
		if (state == "fresh" || state == "same") && e != nil || (state == "future" || state == "changed") && e != ErrUnavailable {
			t.Fatal(state, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
