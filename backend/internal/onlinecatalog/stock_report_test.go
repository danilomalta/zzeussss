package onlinecatalog

import (
	"context"
	"encoding/csv"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func stockQuery42() StockReportQuery {
	return StockReportQuery{Active: "true", Issue: "all", HorizonDays: 7, Limit: 50}
}
func stockItem42(stock int64, demand string) StockReportItem {
	return StockReportItem{Product: Product{ID: 1, Name: "=Item;\"42\"", SKU: "@sku42", PriceCents: 1000, Active: true, Version: 1}, StockUnits: stock, MinimumUnits: 10, DailyDemand: demand}
}
func stockRows42(v StockReportItem) *sqlmock.Rows {
	var stamp interface{}
	if v.ForecastAt != nil {
		stamp = *v.ForecastAt
	}
	return sqlmock.NewRows([]string{"id", "nome", "description", "sku", "price", "active", "version", "stock", "minimum", "reorder", "demand", "forecast"}).AddRow(v.ID, v.Name, v.Description, v.SKU, "10.00", v.Active, v.Version, v.StockUnits, v.MinimumUnits, v.ReorderPointUnits, v.DailyDemand, stamp)
}
func TestStockMetricsExactUnknownAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		stock                   int64
		demand, state, coverage string
		low                     bool
	}{{0, "0.0000", "no_positive_demand", "", false}, {0, "2.0000", "below_horizon", "0.0000", true}, {14, "2", "at_or_above_horizon", "7.0000", false}, {13, "2", "below_horizon", "6.5000", true}, {1, "0.1429", "below_horizon", "6.9979", true}, {MaxStockUnits, "0.0001", "at_or_above_horizon", "21474836470000.0000", false}} {
		v, e := stockMetrics(stockItem42(tc.stock, tc.demand), 7)
		if e != nil || v.CoverageState != tc.state || v.LowCoverage != tc.low {
			t.Fatal(tc, v, e)
		}
		if tc.coverage == "" && v.CoverageDays != nil || tc.coverage != "" && (v.CoverageDays == nil || *v.CoverageDays != tc.coverage) {
			t.Fatal(tc, v)
		}
		want := int64(10) - tc.stock
		if want < 0 {
			want = 0
		}
		if v.MissingToMinimum != want {
			t.Fatal(v)
		}
	}
	for _, raw := range []string{"-1", "NaN", "Infinity", "1e2", "1.23456", "100000000.0000", "", "1."} {
		if _, e := demandUnits(raw); e != ErrUnavailable {
			t.Fatal(raw, e)
		}
	}
	for _, edit := range []func(*StockReportQuery){func(q *StockReportQuery) { q.HorizonDays = 0 }, func(q *StockReportQuery) { q.HorizonDays = 366 }, func(q *StockReportQuery) { q.Active = "yes" }, func(q *StockReportQuery) { q.Issue = "bad" }, func(q *StockReportQuery) { q.Limit = 101 }, func(q *StockReportQuery) { q.Offset = -1 }} {
		q := stockQuery42()
		edit(&q)
		if ValidateStockReport(q) != ErrInput {
			t.Fatal(q)
		}
	}
}
func TestStockReportPageAndCSV(t *testing.T) {
	s, m := qualityFixture(t)
	q := stockQuery42()
	q.Query = "%_"
	q.Limit = 1
	qualityLive(m)
	m.ExpectQuery("WITH stock_report AS.*SELECT count").WithArgs(fixtureActor.Tenant, qualityPattern(q.Query), q.Active, q.HorizonDays, q.Issue).WillReturnRows(sqlmock.NewRows([]string{"n", "stamp"}).AddRow(2, time.Now()))
	m.ExpectQuery("WITH stock_report AS.*SELECT id,nome").WithArgs(fixtureActor.Tenant, qualityPattern(q.Query), q.Active, q.HorizonDays, q.Issue, 1, 0).WillReturnRows(stockRows42(stockItem42(0, "0.0000")))
	m.ExpectCommit()
	page, e := s.StockReport(context.Background(), fixtureActor, q)
	if e != nil || page.Total != 2 || !page.HasMore || page.NextOffset == nil || *page.NextOffset != 1 || len(page.SourceHash) != 64 {
		t.Fatal(page, e)
	}
	report, e := StockReportCSV(page)
	if e != nil {
		t.Fatal(e)
	}
	reader := csv.NewReader(strings.NewReader(report))
	reader.Comma = ';'
	rows, e := reader.ReadAll()
	if e != nil || len(rows) != 2 || rows[1][1] != "'@sku42" || rows[1][2] != "'=Item;\"42\"" || rows[1][8] != "0.0000" || rows[1][9] != "" || rows[1][10] != "" || rows[1][11] != "no_positive_demand" {
		t.Fatal(rows, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestStockSummaryAndInvalidDemand(t *testing.T) {
	for _, invalid := range []int64{0, 1} {
		s, m := qualityFixture(t)
		qualityLive(m)
		m.ExpectQuery("WITH stock_report AS.*SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "zero", "below", "low", "unknown", "invalid", "stamp"}).AddRow(5, 1, 2, 2, 1, invalid, time.Now()))
		if invalid == 0 {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		r, e := s.StockReportSummary(context.Background(), fixtureActor, stockQuery42())
		if invalid == 0 && (e != nil || r.Products != 5 || r.NoPositiveDemand != 1) || invalid != 0 && e != ErrUnavailable {
			t.Fatal(r, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestStockReportRevocationPartialAndBadRows(t *testing.T) {
	for _, mode := range []string{"revoked", "partial", "negative", "wrong_issue"} {
		s, m := qualityFixture(t)
		q := stockQuery42()
		if mode == "revoked" {
			m.ExpectBegin()
			m.ExpectQuery("SELECT .*online_sessions").WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectRollback()
		} else {
			qualityLive(m)
			n := 1
			if mode == "partial" {
				n = 2
			}
			m.ExpectQuery("WITH stock_report AS.*SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "stamp"}).AddRow(n, time.Now()))
			v := stockItem42(1, "2.0000")
			if mode == "negative" {
				v.DailyDemand = "-1"
			}
			if mode == "wrong_issue" {
				q.Issue = "zero_stock"
			}
			m.ExpectQuery("WITH stock_report AS.*SELECT id,nome").WillReturnRows(stockRows42(v))
			m.ExpectRollback()
		}
		_, e := s.StockReport(context.Background(), fixtureActor, q)
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
	for _, role := range []string{"cashier", "employee", "accountant"} {
		a := fixtureActor
		a.Role = role
		if _, e := (Store{}).StockReport(context.Background(), a, stockQuery42()); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
	a := fixtureActor
	a.Role = "stock"
	if _, e := (Store{}).StockReport(context.Background(), a, stockQuery42()); e != ErrUnavailable {
		t.Fatal("stock role rejected before database", e)
	}
}
