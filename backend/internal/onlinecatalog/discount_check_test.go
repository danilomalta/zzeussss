package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func checkQ38() DiscountCheckQuery { return DiscountCheckQuery{Issue: "all", Limit: 1} }
func checkRows38(kind string) *sqlmock.Rows {
	price, version, active := int64(1000), int64(1), true
	changed, vchanged, missing := false, false, false
	if kind == "price" {
		price = 1100
		version = 2
		changed = true
		vchanged = true
	}
	if kind == "inactive" {
		active = false
		version = 2
		vchanged = true
	}
	if kind == "version" {
		version = 2
		vchanged = true
	}
	if kind == "legacy" {
		missing = true
	}
	actor, action, op := fixtureActor.User, "approve", "11111111-1111-4111-8111-111111111112"
	snap := DiscountSuggestion{ID: 1, ProductID: 2, Status: "APPROVED", ReviewedBy: fixtureActor.User, DiscountBPS: 1500, Product: Product{ID: 2, Active: true, PriceCents: 1000, Version: 1}}
	if kind == "corrupt" {
		snap.Product.ID = 3
	}
	raw, _ := json.Marshal(snap)
	if missing {
		actor = ""
		action = ""
		op = ""
		raw = []byte(`{}`)
	}
	money := "10.00"
	if price == 1100 {
		money = "11.00"
	}
	return sqlmock.NewRows([]string{"id", "pid", "name", "sku", "active", "price", "version", "percent", "reviewer", "op", "snapshot", "missing", "inactive", "priceChanged", "versionChanged", "actor", "action"}).AddRow(1, 2, " =2+2", "@SKU", active, money, version, "15.00", fixtureActor.User, op, raw, missing, !active, changed, vchanged, actor, action)
}
func TestDiscountCheckSnapshotsAndLegacy(t *testing.T) {
	for _, kind := range []string{"current", "price", "inactive", "version", "legacy", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			s, m := qualityFixture(t)
			qualityLive(m)
			m.ExpectQuery("WITH checks AS").WithArgs(fixtureActor.Tenant, "", int64(1)).WillReturnRows(checkRows38(kind))
			if kind == "corrupt" {
				m.ExpectRollback()
			} else {
				m.ExpectCommit()
			}
			r, e := s.DiscountCheck(context.Background(), fixtureActor, 1)
			if kind == "corrupt" {
				if e != ErrUnavailable {
					t.Fatal(e)
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				issue := map[string]string{"price": "price_changed", "inactive": "inactive", "version": "version_changed", "legacy": "no_receipt"}[kind]
				if issue != "" && !qualityHas(r.Issues, issue) {
					t.Fatal(r)
				}
				if kind == "current" && len(r.Issues) != 0 {
					t.Fatal(r)
				}
				if kind == "legacy" && (r.ApprovedPrice != nil || r.HasReceipt) {
					t.Fatal("fabricated snapshot")
				}
				if kind != "legacy" && (r.ApprovedPrice == nil || *r.ApprovedPrice != 1000) {
					t.Fatal(r)
				}
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestDiscountCheckPageCSVAndInput(t *testing.T) {
	q := checkQ38()
	q.Issue = "price_changed"
	q.Query = "a%_"
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH checks AS").WithArgs(fixtureActor.Tenant, "%a\\%\\_%", "price_changed").WillReturnRows(sqlmock.NewRows([]string{"count", "ts"}).AddRow(2, time.Now()))
	m.ExpectQuery("WITH checks AS").WithArgs(fixtureActor.Tenant, "%a\\%\\_%", "price_changed", 1, 0).WillReturnRows(checkRows38("price"))
	m.ExpectCommit()
	r, e := s.DiscountChecks(context.Background(), fixtureActor, q)
	if e != nil || r.Total != 2 || !r.HasMore || *r.NextOffset != 1 || len(r.SourceHash) != 64 {
		t.Fatal(r, e)
	}
	out, e := DiscountChecksCSV(r)
	csvr := csv.NewReader(strings.NewReader(out))
	csvr.Comma = ';'
	rows, ce := csvr.ReadAll()
	if e != nil || ce != nil || rows[1][2] != "'@SKU" || rows[1][3] != "' =2+2" || rows[1][5] != "1100" || rows[1][8] != "1000" {
		t.Fatal(rows, e, ce)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []DiscountCheckQuery{{Issue: "bad", Limit: 1}, {Issue: "all", Limit: 101}, {Issue: "all", Limit: 1, Offset: -1}, {Issue: "all", Limit: 1, Query: strings.Repeat("a", 121)}} {
		if ValidateDiscountCheck(bad) != ErrInput {
			t.Fatal(bad)
		}
	}
	for _, role := range []string{"stock", "cashier", "employee", "accountant"} {
		a := fixtureActor
		a.Role = role
		if _, e := (Store{}).DiscountChecks(context.Background(), a, checkQ38()); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestDiscountCheckSummaryMissingAndRevoked(t *testing.T) {
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH checks AS").WithArgs(fixtureActor.Tenant, "").WillReturnRows(sqlmock.NewRows([]string{"total", "attention", "price", "version", "inactive", "missing", "ts"}).AddRow(4, 3, 1, 2, 1, 1, time.Now()))
	m.ExpectCommit()
	r, e := s.DiscountCheckSummary(context.Background(), fixtureActor, checkQ38())
	if e != nil || r.Total != 4 || r.Attention != 3 {
		t.Fatal(r, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH checks AS").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectRollback()
	if _, e = s.DiscountCheck(context.Background(), fixtureActor, 1); e != ErrMissing {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*online_sessions").WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	if _, e = s.DiscountCheckSummary(context.Background(), fixtureActor, checkQ38()); e != ErrDenied {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestDiscountCheckCountMismatchAndEmpty(t *testing.T) {
	for _, kind := range []string{"count", "wrong_issue", "empty"} {
		t.Run(kind, func(t *testing.T) {
			s, m := qualityFixture(t)
			qualityLive(m)
			q := checkQ38()
			q.Issue = "price_changed"
			total := 1
			if kind == "empty" {
				total = 0
			}
			if kind == "count" {
				total = 0
			}
			m.ExpectQuery("WITH checks AS").WillReturnRows(sqlmock.NewRows([]string{"count", "ts"}).AddRow(total, time.Now()))
			rows := checkRows38("price")
			if kind == "wrong_issue" {
				rows = checkRows38("current")
			}
			if kind == "empty" {
				rows = sqlmock.NewRows([]string{"id"})
			}
			m.ExpectQuery("WITH checks AS").WillReturnRows(rows)
			if kind == "empty" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			p, e := s.DiscountChecks(context.Background(), fixtureActor, q)
			if kind == "empty" {
				if e != nil || p.Items == nil || len(p.Items) != 0 || p.HasMore {
					t.Fatal(p, e)
				}
			} else if e != ErrUnavailable {
				t.Fatal(e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
