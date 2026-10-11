package onlinecatalog

import (
	"context"
	"encoding/csv"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func reportQuery41() CostReportQuery { return CostReportQuery{Change: "all", Limit: 50} }
func TestCostReportFiltersAndClassification(t *testing.T) {
	q := reportQuery41()
	if ValidateCostReport(q) != nil {
		t.Fatal("default rejected")
	}
	for _, kind := range []string{"initial", "increased", "decreased", "unchanged"} {
		v := q
		v.Change = kind
		if ValidateCostReport(v) != nil {
			t.Fatal(kind)
		}
	}
	for _, edit := range []func(*CostReportQuery){
		func(q *CostReportQuery) { q.Change = "unknown" }, func(q *CostReportQuery) { q.ActorID = "invalid" },
		func(q *CostReportQuery) { q.ProductID = -1 }, func(q *CostReportQuery) { q.Limit = 101 },
		func(q *CostReportQuery) { q.Offset = -1 }, func(q *CostReportQuery) { q.Query = "bad\n" },
		func(q *CostReportQuery) { q.From = "yesterday" }, func(q *CostReportQuery) { q.Until = "2026-10-11" },
		func(q *CostReportQuery) { q.From = "2026-10-12T00:00:00Z"; q.Until = "2026-10-11T00:00:00Z" },
	} {
		v := q
		edit(&v)
		if ValidateCostReport(v) != ErrInput {
			t.Fatal(v)
		}
	}
	_, _, _, r, _ := costFixture39()
	entry, e := costReportEntry(r)
	if e != nil || entry.Change != "initial" || entry.DeltaCents != nil {
		t.Fatal(entry, e)
	}
	for _, tc := range []struct {
		before int64
		kind   string
		delta  int64
	}{{600, "increased", 100}, {800, "decreased", -100}, {700, "unchanged", 0}} {
		v := r
		v.Before = r.After
		v.Before.CostCents = &tc.before
		v.After.Revision = 2
		entry, e = costReportEntry(v)
		if e != nil || entry.Change != tc.kind || *entry.DeltaCents != tc.delta {
			t.Fatal(entry, e)
		}
	}
}
func TestCostReportPageAndCSVProtectSnapshots(t *testing.T) {
	s, m := qualityFixture(t)
	q := reportQuery41()
	q.Query = "Item%_"
	q.Limit = 1
	_, _, _, r, h := costFixture39()
	r.Product.Name = "=Item;\"original\""
	r.Product.SKU = "@item"
	r.Reason = "+Invoice"
	qualityLive(m)
	args := costReportArgs(fixtureActor, q)
	m.ExpectQuery("WITH history AS.*SELECT count").WithArgs(args[0], args[1], args[2], args[3], args[4], args[5], args[6]).WillReturnRows(sqlmock.NewRows([]string{"count", "stamp"}).AddRow(2, time.Now()))
	m.ExpectQuery("WITH history AS.*SELECT operation_id").WithArgs(args[0], args[1], args[2], args[3], args[4], args[5], args[6], 1, 0).WillReturnRows(costReceiptRows39(r, h))
	m.ExpectCommit()
	page, e := s.CostReport(context.Background(), fixtureActor, q)
	if e != nil || page.Total != 2 || len(page.Items) != 1 || !page.HasMore || page.NextOffset == nil || *page.NextOffset != 1 || len(page.SourceHash) != 64 {
		t.Fatal(page, e)
	}
	text, e := CostReportCSV(page)
	if e != nil {
		t.Fatal(e)
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = ';'
	cells, e := reader.ReadAll()
	if e != nil || len(cells) != 2 || cells[1][2] != "'@item" || cells[1][3] != "'=Item;\"original\"" || cells[1][7] != "" || cells[1][8] != "700" || cells[1][9] != "" || cells[1][11] != "'+Invoice" {
		t.Fatal(cells, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestCostReportSummaryPartitionsAndFailureClosed(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		s, m := qualityFixture(t)
		qualityLive(m)
		initial := 2
		if invalid {
			initial = 3
		}
		m.ExpectQuery("WITH history AS.*SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "i", "up", "down", "same", "products", "actors", "stamp"}).AddRow(5, initial, 1, 1, 1, 2, 1, time.Now()))
		if invalid {
			m.ExpectRollback()
		} else {
			m.ExpectCommit()
		}
		r, e := s.CostReportSummary(context.Background(), fixtureActor, reportQuery41())
		if invalid && e != ErrUnavailable || !invalid && (e != nil || r.Total != 5 || r.Products != 2) {
			t.Fatal(r, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestCostReportDetailAuditAndMissing(t *testing.T) {
	for _, missing := range []bool{false, true} {
		s, m := qualityFixture(t)
		qualityLive(m)
		_, _, _, r, h := costFixture39()
		rows := costReceiptRows39(r, h)
		if missing {
			rows = emptyCostReceipt39()
		}
		m.ExpectQuery("SELECT operation_id::text.*WHERE tenant_id=\\$1 AND operation_id=\\$2$").WithArgs(fixtureActor.Tenant, testOp).WillReturnRows(rows)
		if missing {
			m.ExpectRollback()
		} else {
			m.ExpectCommit()
		}
		entry, e := s.CostReportDetail(context.Background(), fixtureActor, testOp)
		if missing && e != ErrMissing || !missing && (e != nil || entry.Change != "initial") {
			t.Fatal(entry, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestCostReportRevokedPartialAndUnavailable(t *testing.T) {
	for _, mode := range []string{"revoked", "partial", "query"} {
		s, m := qualityFixture(t)
		if mode == "revoked" {
			m.ExpectBegin()
			m.ExpectQuery("SELECT .*online_sessions").WillReturnRows(sqlmock.NewRows([]string{"user"}))
			m.ExpectRollback()
		} else {
			qualityLive(m)
			if mode == "query" {
				m.ExpectQuery("WITH history AS.*SELECT count").WillReturnError(errors.New("private db details"))
			} else {
				m.ExpectQuery("WITH history AS.*SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "stamp"}).AddRow(2, time.Now()))
				_, _, _, r, h := costFixture39()
				m.ExpectQuery("WITH history AS.*SELECT operation_id").WillReturnRows(costReceiptRows39(r, h))
			}
			m.ExpectRollback()
		}
		_, e := s.CostReport(context.Background(), fixtureActor, reportQuery41())
		want := ErrUnavailable
		if mode == "revoked" {
			want = ErrDenied
		}
		if e != want {
			t.Fatal(mode, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
	for _, role := range []string{"stock", "cashier", "employee", "accountant"} {
		a := fixtureActor
		a.Role = role
		if _, e := (Store{}).CostReport(context.Background(), a, reportQuery41()); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := (Store{}).CostReportDetail(context.Background(), a, testOp); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
