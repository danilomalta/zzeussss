package onlinecatalog

import (
	"context"
	"encoding/csv"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func replenishQuery44() ReplenishmentQuery {
	return ReplenishmentQuery{Active: "true", State: "all", ReviewDays: 7, Limit: 50}
}
func TestReplenishmentExactRoundingAndBlockedQuantities(t *testing.T) {
	p, _, _, _ := planningFixture43()
	for _, tc := range []struct {
		stock, min, reorder, lead, review int64
		demand                            string
		active                            bool
		target, qty                       int64
		state                             string
	}{
		{4, 5, 8, 2, 7, "1.0000", true, 9, 5, "needs_replenishment"}, {7, 0, 0, 0, 7, "1.0000", true, 7, 0, "within_target"}, {0, 3, 0, 0, 7, "0.0000", true, 3, 3, "needs_replenishment"}, {1, 0, 0, 0, 7, "0.1429", true, 2, 1, "needs_replenishment"}, {0, 0, 0, 0, 7, "0.0001", true, 1, 1, "needs_replenishment"}, {0, 0, 0, 365, 365, "99999999.9999", true, 73000000000, 0, "quantity_exceeds_limit"}, {0, 10, 0, 0, 7, "0.0000", false, 10, 0, "inactive"}, {MaxStockUnits, MaxStockUnits, 0, 0, 7, "0.0000", true, MaxStockUnits, 0, "within_target"},
	} {
		p.StockUnits = tc.stock
		p.Product.Active = tc.active
		p.Settings = StockSettings{MinimumUnits: tc.min, ReorderPointUnits: tc.reorder, LeadTimeDays: tc.lead, DailyDemand: tc.demand}
		v, e := replenish(p, tc.review)
		if e != nil || v.TargetUnits != tc.target || v.SuggestedUnits != tc.qty || v.State != tc.state || !v.PlanningOnly || v.RequiresApproval != (tc.state == "needs_replenishment") {
			t.Fatal(tc, v, e)
		}
	}
	p.Product.Active = true
	p.Settings.DailyDemand = "-1"
	if _, e := replenish(p, 7); e != ErrUnavailable {
		t.Fatal(e)
	}
	p.Settings.DailyDemand = "1.0000"
	p.Settings.LeadTimeDays = 366
	if _, e := replenish(p, 7); e != ErrUnavailable {
		t.Fatal(e)
	}
}
func TestReplenishmentSimulationStrictInputAndNoWrites(t *testing.T) {
	valid := `{"expected_version":1,"review_days":7,"stock_units":0}`
	for _, raw := range []string{"{}", valid + "{}", strings.Replace(valid, `"stock_units":0`, `"stock_units":null`, 1), strings.Replace(valid, `"stock_units":0`, `"stock_units":0.1`, 1), strings.Replace(valid, `"stock_units":0`, `"stock_units":0,"stock_units":1`, 1), strings.Replace(valid, `"review_days":7`, `"review_days":366`, 1), strings.Replace(valid, `"stock_units":0`, `"stock_units":0,"tenant_id":"x"`, 1)} {
		if _, e := DecodeReplenishmentSimulation("application/json", []byte(raw)); e != ErrInput {
			t.Fatal(raw, e)
		}
	}
	if _, e := DecodeReplenishmentSimulation("text/plain", []byte(valid)); e != ErrInput {
		t.Fatal(e)
	}
	input, e := DecodeReplenishmentSimulation("application/json", []byte(valid))
	if e != nil {
		t.Fatal(e)
	}
	p, _, _, _ := planningFixture43()
	p.Settings = StockSettings{MinimumUnits: 5, ReorderPointUnits: 8, LeadTimeDays: 2, DailyDemand: "1.0000"}
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT id,nome").WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(planRows43(p))
	m.ExpectCommit()
	got, e := s.SimulateReplenishment(context.Background(), fixtureActor, 1, input)
	if e != nil || !got.Simulated || got.Current.StockUnits != 4 || got.Current.SuggestedUnits != 5 || got.Scenario.StockUnits != 0 || got.Scenario.SuggestedUnits != 9 {
		t.Fatal(got, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	qualityLive(m)
	p.Product.Version = 2
	m.ExpectQuery("SELECT id,nome").WillReturnRows(planRows43(p))
	m.ExpectCommit()
	if _, e = s.SimulateReplenishment(context.Background(), fixtureActor, 1, input); e != ErrConflict {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestReplenishmentPageStablePaginationAndCSVProtection(t *testing.T) {
	q := replenishQuery44()
	q.Limit = 1
	q.Query = "%_"
	p, _, _, _ := planningFixture43()
	p.Product.Name = "=Item;\"44\""
	p.Product.SKU = "@sku44"
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH raw AS.*SELECT .*count").WithArgs(fixtureActor.Tenant, qualityPattern(q.Query), q.Active, q.ReviewDays, q.State).WillReturnRows(sqlmock.NewRows([]string{"total", "invalid", "stamp"}).AddRow(2, 0, time.Now()))
	m.ExpectQuery("WITH raw AS.*SELECT id,nome").WithArgs(fixtureActor.Tenant, qualityPattern(q.Query), q.Active, q.ReviewDays, q.State, 1, 0).WillReturnRows(planRows43(p))
	m.ExpectCommit()
	page, e := s.Replenishments(context.Background(), fixtureActor, q)
	if e != nil || page.Total != 2 || !page.HasMore || page.NextOffset == nil || *page.NextOffset != 1 || len(page.SourceHash) != 64 {
		t.Fatal(page, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	report, e := ReplenishmentCSV(page)
	if e != nil {
		t.Fatal(e)
	}
	reader := csv.NewReader(strings.NewReader(report))
	reader.Comma = ';'
	rows, e := reader.ReadAll()
	if e != nil || len(rows) != 2 || rows[1][1] != "'@sku44" || rows[1][2] != "'=Item;\"44\"" || rows[1][19] != "true" {
		t.Fatal(rows, e)
	}
	page.Items[0].SuggestedUnits++
	if _, e := ReplenishmentCSV(page); e != ErrUnavailable {
		t.Fatal("forged quantity accepted", e)
	}
}
func TestReplenishmentFailsClosedOnInvalidDataSessionAndRole(t *testing.T) {
	q := replenishQuery44()
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH raw AS.*SELECT .*count").WillReturnRows(sqlmock.NewRows([]string{"total", "invalid", "stamp"}).AddRow(0, 1, time.Now()))
	m.ExpectRollback()
	if _, e := s.Replenishments(context.Background(), fixtureActor, q); e != ErrUnavailable {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*online_sessions").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectRollback()
	if _, e := s.ProductReplenishment(context.Background(), fixtureActor, 1, 7); e != ErrDenied {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"cashier", "accountant", "employee"} {
		a := fixtureActor
		a.Role = role
		s = Store{}
		if _, e := s.Replenishments(context.Background(), a, q); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.ReplenishmentSummary(context.Background(), a, q); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.SimulateReplenishment(context.Background(), a, 1, ReplenishmentSimulationInput{1, 7, 0}); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
	for _, edit := range []func(*ReplenishmentQuery){func(q *ReplenishmentQuery) { q.ReviewDays = 0 }, func(q *ReplenishmentQuery) { q.ReviewDays = 366 }, func(q *ReplenishmentQuery) { q.Active = "" }, func(q *ReplenishmentQuery) { q.State = "bad" }, func(q *ReplenishmentQuery) { q.Limit = 101 }, func(q *ReplenishmentQuery) { q.Offset = -1 }} {
		q := replenishQuery44()
		edit(&q)
		if ValidateReplenishmentQuery(q) != ErrInput {
			t.Fatal(q)
		}
	}
}
func TestReplenishmentSummaryCountsAllStates(t *testing.T) {
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH raw AS.*SELECT count").WillReturnRows(sqlmock.NewRows([]string{"total", "needs", "within", "inactive", "blocked", "noDemand", "invalid", "stamp"}).AddRow(6, 3, 1, 1, 1, 2, 0, time.Now()))
	m.ExpectCommit()
	v, e := s.ReplenishmentSummary(context.Background(), fixtureActor, replenishQuery44())
	if e != nil || v.Products != 6 || v.NeedsReplenishment != 3 || v.QuantityExceedsLimit != 1 {
		t.Fatal(v, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
