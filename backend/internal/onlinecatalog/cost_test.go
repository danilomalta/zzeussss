package onlinecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"strconv"
	"strings"
	"testing"
	"time"
)

func costFixture39() (Product, CostState, CostInput, CostReceipt, string) {
	p := Product{ID: 1, Name: "Item", SKU: "item", PriceCents: 1000, Active: true, Version: 1}
	cost := int64(700)
	actor := fixtureActor.User
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	after := CostState{true, 1, &cost, &actor, &now}
	v := CostInput{testOp, 1, 0, 700, "Nota conferida"}
	r := CostReceipt{testOp, 1, actor, v.Reason, CostState{}, after, p, now}
	return p, after, v, r, costHash(1, v)
}
func costRows39(v CostState) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"cost", "revision", "actor", "updated"})
	if v.Known {
		rows.AddRow(*v.CostCents, v.Revision, *v.ActorID, *v.UpdatedAt)
	}
	return rows
}
func costReceiptRows39(r CostReceipt, h string) *sqlmock.Rows {
	b, _ := json.Marshal(r.Before)
	a, _ := json.Marshal(r.After)
	p, _ := json.Marshal(r.Product)
	return sqlmock.NewRows([]string{"op", "product", "actor", "reason", "before", "after", "product_snapshot", "created", "hash"}).AddRow(r.OperationID, r.ProductID, r.ActorID, r.Reason, b, a, p, r.CreatedAt, h)
}
func emptyCostReceipt39() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"op", "product", "actor", "reason", "before", "after", "product_snapshot", "created", "hash"})
}
func costProductRows39(p Product) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}).AddRow(p.ID, p.Name, p.Description, p.SKU, "10.00", p.Active, p.Version)
}
func TestCostStrictInputAndDeniedRoles(t *testing.T) {
	_, _, v, _, _ := costFixture39()
	b, _ := json.Marshal(v)
	if parsed, e := DecodeCost("application/json; charset=utf-8", b); e != nil || parsed != v {
		t.Fatal(parsed, e)
	}
	bad := []string{"{}", string(b) + "{}", strings.Replace(string(b), `"cost_cents":700`, `"cost_cents":null`, 1), strings.Replace(string(b), `"cost_cents":700`, `"cost_cents":0.7`, 1), strings.Replace(string(b), `"cost_cents":700`, `"cost_cents":700,"cost_cents":700`, 1), strings.Replace(string(b), `"cost_cents":700`, `"cost_cents":-1`, 1), strings.Replace(string(b), `"expected_cost_revision":0`, `"expected_cost_revision":9007199254740991`, 1), strings.Replace(string(b), `"reason":"Nota conferida"`, `"reason":" "`, 1), strings.Replace(string(b), `"reason":"Nota conferida"`, `"reason":"Nota conferida","tenant_id":"x"`, 1)}
	for _, raw := range bad {
		if _, e := DecodeCost("application/json", []byte(raw)); e != ErrInput {
			t.Fatal("invalid accepted", e)
		}
	}
	for _, ctype := range []string{"text/plain", ""} {
		if _, e := DecodeCost(ctype, b); e != ErrInput {
			t.Fatal(e)
		}
	}
	if _, e := DecodeCost("application/json", []byte(strings.Repeat(" ", 8193))); e != ErrInput {
		t.Fatal(e)
	}
	for _, role := range []string{"stock", "cashier", "accountant", "employee"} {
		a := fixtureActor
		a.Role = role
		s := Store{}
		if _, e := s.SetCost(context.Background(), a, 1, v); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.ProductCost(context.Background(), a, 1); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.ProductMargin(context.Background(), a, 1); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.CostHistory(context.Background(), a, 1, 50, 0); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := s.CostOperation(context.Background(), a, testOp); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestCostMarginExactUnknownZeroNegativeAndBoundary(t *testing.T) {
	p, c, _, _, _ := costFixture39()
	unknown, e := calculateMargin(ProductCost{p, CostState{}})
	if e != nil || unknown.State != "unknown_cost" || unknown.MarginCents != nil || unknown.MarginBPS != nil {
		t.Fatal(unknown, e)
	}
	for _, tc := range []struct {
		price, cost, margin, bps int64
		state                    string
		percent                  bool
	}{{1000, 700, 300, 3000, "nonnegative_margin", true}, {1000, 1200, -200, -2000, "negative_margin", true}, {0, 700, -700, 0, "zero_price", false}, {0, 0, 0, 0, "zero_price", false}, {3, 1, 2, 6667, "nonnegative_margin", true}, {3, 5, -2, -6667, "negative_margin", true}, {32, 1, 31, 9688, "nonnegative_margin", true}, {32, 33, -1, -313, "negative_margin", true}, {MaxPrice, 0, MaxPrice, 10000, "nonnegative_margin", true}, {1, MaxPrice, 1 - MaxPrice, (1 - MaxPrice) * 10000, "negative_margin", true}, {1000, 0, 1000, 10000, "nonnegative_margin", true}} {
		p.PriceCents = tc.price
		cost := tc.cost
		c.CostCents = &cost
		r, e := calculateMargin(ProductCost{p, c})
		if e != nil || r.State != tc.state || r.MarginCents == nil || *r.MarginCents != tc.margin || tc.percent && (r.MarginBPS == nil || *r.MarginBPS != strconv.FormatInt(tc.bps, 10)) || !tc.percent && r.MarginBPS != nil {
			t.Fatal(tc, r, e)
		}
	}
}
func TestCostAtomicSuccessAndLateRollback(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "ignored"} {
		fail := outcome != "success"
		s, m := qualityFixture(t)
		p, after, v, r, h := costFixture39()
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(fixtureActor.Tenant + ":cost:" + testOp).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WithArgs(fixtureActor.Tenant, testOp).WillReturnRows(emptyCostReceipt39())
		m.ExpectQuery("SELECT id,nome.*FOR UPDATE").WillReturnRows(costProductRows39(p))
		m.ExpectQuery("SELECT cost_cents.*FOR UPDATE").WillReturnRows(costRows39(CostState{}))
		m.ExpectExec("INSERT INTO online_product_costs").WithArgs(fixtureActor.Tenant, int64(1), int64(700), int64(1), fixtureActor.User).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT cost_cents").WillReturnRows(costRows39(after))
		m.ExpectExec("INSERT INTO online_cost_operations").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(costReceiptRows39(r, h))
		if fail {
			if outcome == "ignored" {
				m.ExpectExec("INSERT INTO online_cost_outbox").WillReturnResult(sqlmock.NewResult(0, 0))
			} else {
				m.ExpectExec("INSERT INTO online_cost_outbox").WillReturnError(errors.New("private failure"))
			}
			m.ExpectRollback()
		} else {
			m.ExpectExec("INSERT INTO online_cost_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}
		got, e := s.SetCost(context.Background(), fixtureActor, 1, v)
		if fail && e != ErrUnavailable || !fail && (e != nil || got.Product != p || !equalCost(got.After, after)) {
			t.Fatal(got, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestCostReplayAndConflictsPreserveReceipt(t *testing.T) {
	for _, kind := range []string{"replay", "actor", "payload"} {
		s, m := qualityFixture(t)
		_, _, v, r, h := costFixture39()
		if kind == "actor" {
			r.ActorID = "11111111-1111-4111-8111-111111111119"
			r.After.ActorID = &r.ActorID
		}
		if kind == "payload" {
			v.CostCents++
		}
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(costReceiptRows39(r, h))
		if kind == "replay" {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		_, e := s.SetCost(context.Background(), fixtureActor, 1, v)
		if kind == "replay" && e != nil || kind != "replay" && e != ErrConflict {
			t.Fatal(kind, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
	for _, kind := range []string{"product_version", "cost_revision"} {
		s, m := qualityFixture(t)
		p, after, v, _, _ := costFixture39()
		qualityLive(m)
		if kind == "product_version" {
			p.Version++
		} else {
			v.ExpectedCostRevision = 2
		}
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(emptyCostReceipt39())
		m.ExpectQuery("SELECT id,nome").WillReturnRows(costProductRows39(p))
		m.ExpectQuery("SELECT cost_cents").WillReturnRows(costRows39(after))
		m.ExpectRollback()
		if _, e := s.SetCost(context.Background(), fixtureActor, 1, v); e != ErrConflict {
			t.Fatal(kind, e)
		}
		if e := m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestCostReadsUnknownRevocationAndForgedReceipt(t *testing.T) {
	s, m := qualityFixture(t)
	p, _, _, r, h := costFixture39()
	qualityLive(m)
	m.ExpectQuery("SELECT id,nome").WillReturnRows(costProductRows39(p))
	m.ExpectQuery("SELECT cost_cents").WillReturnRows(costRows39(CostState{}))
	m.ExpectCommit()
	v, e := s.ProductCost(context.Background(), fixtureActor, 1)
	if e != nil || v.Cost.Known || v.Cost.CostCents != nil {
		t.Fatal(v, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*online_sessions").WillReturnRows(sqlmock.NewRows([]string{"user"}))
	m.ExpectRollback()
	if _, e = s.CostOperation(context.Background(), fixtureActor, testOp); e != ErrDenied {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	r.After.Revision = 9
	s, m = qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT operation_id::text").WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnRows(costReceiptRows39(r, h))
	m.ExpectRollback()
	if _, e = s.CostOperation(context.Background(), fixtureActor, testOp); e != ErrUnavailable {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
