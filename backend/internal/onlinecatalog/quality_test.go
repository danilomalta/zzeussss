package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func qualityFixture(t *testing.T) (Store, sqlmock.Sqlmock) {
	t.Helper()
	db, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return Store{DB: db}, m
}
func qualityLive(m sqlmock.Sqlmock) {
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*online_sessions").WithArgs(fixtureActor.Session, fixtureActor.Tenant, fixtureActor.User, fixtureActor.Role).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fixtureActor.User))
}
func TestQualityValidationAndLiteralSearch(t *testing.T) {
	good := QualityQuery{Active: "active", Issue: "any", Limit: 50}
	for _, issue := range []string{"any", "no_active_gtin", "no_description", "zero_stock"} {
		q := good
		q.Issue = issue
		if ValidateQuality(q) != nil {
			t.Fatal(issue)
		}
	}
	bad := []QualityQuery{{Active: "unknown", Issue: "any", Limit: 50}, {Active: "active", Issue: "cost", Limit: 50}, {Active: "active", Issue: "any", Limit: 101}, {Active: "active", Issue: "any", Limit: 50, Offset: -1}, {Active: "active", Issue: "any", Limit: 50, Query: strings.Repeat("a", 121)}}
	for _, q := range bad {
		if ValidateQuality(q) != ErrInput {
			t.Fatal(q)
		}
	}
	if qualityPattern("a%_\\") != "%a\\%\\_\\\\%" {
		t.Fatal("wildcards not literal")
	}
	for _, role := range []string{"cashier", "employee", "accountant", ""} {
		a := fixtureActor
		a.Role = role
		if _, e := (Store{}).Quality(context.Background(), a, good); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := (Store{}).QualitySummary(context.Background(), a, good); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestQualitySummarySnapshotCountsAndRevocation(t *testing.T) {
	s, m := qualityFixture(t)
	qualityLive(m)
	now := time.Now().UTC()
	m.ExpectQuery("WITH quality AS").WithArgs(fixtureActor.Tenant, "active", "%Arroz%").WillReturnRows(sqlmock.NewRows([]string{"all", "pending", "gtin", "description", "stock", "generated"}).AddRow(4, 3, 2, 2, 1, now))
	m.ExpectCommit()
	r, e := s.QualitySummary(context.Background(), fixtureActor, QualityQuery{Query: "Arroz", Active: "active", Issue: "any", Limit: 50})
	if e != nil || r.Products != 4 || r.PendingProducts != 3 || r.NoActiveGTIN != 2 || r.GeneratedAt != now {
		t.Fatal(r, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*online_sessions").WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	if _, e = s.Quality(context.Background(), fixtureActor, QualityQuery{Active: "active", Issue: "any", Limit: 1}); e != ErrDenied {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestQualityPageConsistentCountAndCSVProtection(t *testing.T) {
	s, m := qualityFixture(t)
	qualityLive(m)
	q := QualityQuery{Active: "active", Issue: "any", Limit: 1}
	now := time.Now().UTC()
	m.ExpectQuery("WITH quality AS").WithArgs(fixtureActor.Tenant, "active", "", "any").WillReturnRows(sqlmock.NewRows([]string{"count", "generated"}).AddRow(2, now))
	m.ExpectQuery("WITH quality AS").WithArgs(fixtureActor.Tenant, "active", "", "any", 1, 0).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version", "stock", "gtin", "description_missing", "zero"}).AddRow(1, " =2+2", "", "@SKU", "2.50", true, 1, 0, true, true, true))
	m.ExpectCommit()
	r, e := s.Quality(context.Background(), fixtureActor, q)
	if e != nil || r.Total != 2 || !r.HasMore || r.NextOffset == nil || *r.NextOffset != 1 || len(r.SourceHash) != 64 || len(r.Items[0].Issues) != 3 {
		t.Fatal(r, e)
	}
	out, e := QualityCSV(r)
	if e != nil {
		t.Fatal(e)
	}
	reader := csv.NewReader(strings.NewReader(out))
	reader.Comma = ';'
	rows, e := reader.ReadAll()
	if e != nil || len(rows) != 2 || rows[1][2] != "'@SKU" || rows[1][3] != "' =2+2" || rows[1][5] != "250" {
		t.Fatal(rows, e)
	}
	if r.Items[0].Name != " =2+2" || r.Items[0].SKU != "@SKU" {
		t.Fatal("report altered data")
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestQualityRejectsCorruptRowsAndDatabaseFailures(t *testing.T) {
	for _, kind := range []string{"wrong_issue", "bad_money", "negative_stock", "count_mismatch", "query_error", "commit_error"} {
		t.Run(kind, func(t *testing.T) {
			s, m := qualityFixture(t)
			qualityLive(m)
			q := QualityQuery{Active: "active", Issue: "zero_stock", Limit: 2}
			now := time.Now().UTC()
			total := 1
			if kind == "count_mismatch" {
				total = 2
			}
			m.ExpectQuery("WITH quality AS").WillReturnRows(sqlmock.NewRows([]string{"count", "generated"}).AddRow(total, now))
			if kind == "query_error" {
				m.ExpectQuery("WITH quality AS").WillReturnError(errors.New("private SQL details"))
			} else {
				stock := int64(0)
				zero := true
				price := "1.00"
				if kind == "negative_stock" {
					stock = -1
				}
				if kind == "wrong_issue" {
					stock = 1
					zero = false
				}
				if kind == "bad_money" {
					price = "0.001"
				}
				m.ExpectQuery("WITH quality AS").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version", "stock", "gtin", "description_missing", "zero"}).AddRow(1, "Item", "", "sku", price, true, 1, stock, true, true, zero))
			}
			if kind == "commit_error" {
				m.ExpectCommit().WillReturnError(errors.New("commit failed"))
			} else {
				m.ExpectRollback()
			}
			if _, e := s.Quality(context.Background(), fixtureActor, q); e != ErrUnavailable {
				t.Fatal(e)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestQualityEmptyPageAndSummaryCannotInventCounts(t *testing.T) {
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH quality AS").WillReturnRows(sqlmock.NewRows([]string{"count", "generated"}).AddRow(0, time.Now()))
	m.ExpectQuery("WITH quality AS").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version", "stock", "gtin", "description_missing", "zero"}))
	m.ExpectCommit()
	p, e := s.Quality(context.Background(), fixtureActor, QualityQuery{Active: "active", Issue: "any", Limit: 50})
	if e != nil || p.Items == nil || len(p.Items) != 0 || p.HasMore || p.NextOffset != nil {
		t.Fatal(p, e)
	}
	s, m = qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("WITH quality AS").WillReturnRows(sqlmock.NewRows([]string{"all", "pending", "gtin", "description", "stock", "generated"}).AddRow(1, 2, 0, 0, 0, time.Now()))
	m.ExpectRollback()
	if _, e = s.QualitySummary(context.Background(), fixtureActor, QualityQuery{Active: "all", Issue: "any", Limit: 50}); e != ErrUnavailable {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
