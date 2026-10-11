package onlinecatalog

import (
	"context"
	"encoding/csv"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func marginQuery40() MarginQuery {
	return MarginQuery{Query: "Item", Active: "all", Issue: "all", TargetBPS: 3000, Limit: 50}
}
func marginRows40(known bool) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version", "cost", "revision", "actor", "updated"})
	if known {
		return rows.AddRow(1, "=Item;\"um\"", "", "@SKU", "10.00", true, 1, 700, 1, fixtureActor.User, time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC))
	}
	return rows.AddRow(1, "=Item;\"um\"", "", "@SKU", "10.00", true, 1, nil, nil, nil, nil)
}
func TestMarginQueriesAndDeniedRoles(t *testing.T) {
	q := marginQuery40()
	if ValidateMarginQuery(q) != nil {
		t.Fatal("query válida rejeitada")
	}
	for _, kind := range []string{"target_low", "target_high", "issue", "active", "limit", "offset", "text"} {
		v := q
		switch kind {
		case "target_low":
			v.TargetBPS = -1
		case "target_high":
			v.TargetBPS = 10001
		case "issue":
			v.Issue = "any"
		case "active":
			v.Active = "yes"
		case "limit":
			v.Limit = 101
		case "offset":
			v.Offset = -1
		case "text":
			v.Query = "bad\n"
		}
		if ValidateMarginQuery(v) != ErrInput {
			t.Fatal(kind)
		}
	}
	for _, role := range []string{"stock", "cashier", "employee", "accountant"} {
		a := fixtureActor
		a.Role = role
		if _, e := (Store{}).Margins(context.Background(), a, q); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := (Store{}).MarginSummary(context.Background(), a, q); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestMarginsSnapshotUnknownKnownPaginationAndCSV(t *testing.T) {
	for _, known := range []bool{false, true} {
		s, m := qualityFixture(t)
		q := marginQuery40()
		qualityLive(m)
		m.ExpectQuery("WITH margins AS.*SELECT count").WithArgs(fixtureActor.Tenant, qualityPattern(q.Query), q.Active, q.TargetBPS, q.Issue).WillReturnRows(sqlmock.NewRows([]string{"count", "stamp"}).AddRow(1, time.Now()))
		m.ExpectQuery("WITH margins AS.*SELECT id,nome").WithArgs(fixtureActor.Tenant, qualityPattern(q.Query), q.Active, q.TargetBPS, q.Issue, q.Limit, q.Offset).WillReturnRows(marginRows40(known))
		m.ExpectCommit()
		r, e := s.Margins(context.Background(), fixtureActor, q)
		if e != nil || len(r.Items) != 1 || r.HasMore || len(r.SourceHash) != 64 || r.Items[0].Cost.Known != known {
			t.Fatal(r, e)
		}
		report, e := MarginsCSV(r)
		if e != nil {
			t.Fatal(e)
		}
		reader := csv.NewReader(strings.NewReader(report))
		reader.Comma = ';'
		cells, e := reader.ReadAll()
		if e != nil || len(cells) != 2 || cells[1][1] != "'@SKU" || cells[1][2] != "'=Item;\"um\"" {
			t.Fatal(cells, e)
		}
		if known && (cells[1][8] != "700" || cells[1][9] != "300" || cells[1][10] != "3000") {
			t.Fatal(cells)
		}
		if !known && (cells[1][8] != "" || cells[1][9] != "" || cells[1][10] != "" || cells[1][11] != "unknown_cost") {
			t.Fatal("zero inventado")
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
	s, m := qualityFixture(t)
	q := marginQuery40()
	q.Limit = 1
	qualityLive(m)
	m.ExpectQuery("WITH margins AS.*SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "stamp"}).AddRow(2, time.Now()))
	m.ExpectQuery("WITH margins AS.*SELECT id,nome").WillReturnRows(marginRows40(true))
	m.ExpectCommit()
	r, e := s.Margins(context.Background(), fixtureActor, q)
	if e != nil || !r.HasMore || r.NextOffset == nil || *r.NextOffset != 1 {
		t.Fatal(r, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestMarginReportRevocationAndPartialResultRefused(t *testing.T) {
	s, m := qualityFixture(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*online_sessions").WillReturnRows(sqlmock.NewRows([]string{"user"}))
	m.ExpectRollback()
	if _, e := s.Margins(context.Background(), fixtureActor, marginQuery40()); e != ErrDenied {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH margins AS.*SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count", "stamp"}).AddRow(2, time.Now()))
	m.ExpectQuery("WITH margins AS.*SELECT id,nome").WillReturnRows(marginRows40(true))
	m.ExpectRollback()
	if _, e := s.Margins(context.Background(), fixtureActor, marginQuery40()); e != ErrUnavailable {
		t.Fatal("partial page accepted", e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestMarginTargetUsesUnroundedRatio(t *testing.T) {
	p, c, _, _, _ := costFixture39()
	p.PriceCents = 30000
	cost := int64(20001)
	c.CostCents = &cost
	v, e := calculateMargin(ProductCost{p, c})
	if e != nil || *v.MarginBPS != "3333" {
		t.Fatal(v, e)
	}
	q := marginQuery40()
	q.Issue = "below_target"
	q.TargetBPS = 3333
	if marginMatches(v, q) {
		t.Fatal("rounded ratio used for threshold")
	}
	cost = 20002
	c.CostCents = &cost
	v, e = calculateMargin(ProductCost{p, c})
	if e != nil || !marginMatches(v, q) {
		t.Fatal("exact below-target omitted")
	}
}
func TestMarginSummaryCountsAndInconsistentState(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		s, m := qualityFixture(t)
		q := marginQuery40()
		qualityLive(m)
		known := int64(2)
		if invalid {
			known = 3
		}
		m.ExpectQuery("WITH margins AS.*SELECT count").WithArgs(fixtureActor.Tenant, qualityPattern(q.Query), q.Active, q.TargetBPS).WillReturnRows(sqlmock.NewRows([]string{"total", "known", "missing", "negative", "below", "zero", "stamp"}).AddRow(3, known, 1, 1, 1, 0, time.Now()))
		if invalid {
			m.ExpectRollback()
		} else {
			m.ExpectCommit()
		}
		r, e := s.MarginSummary(context.Background(), fixtureActor, q)
		if invalid && e != ErrUnavailable || !invalid && (e != nil || r.KnownCost != 2 || r.MissingCost != 1) {
			t.Fatal(r, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
