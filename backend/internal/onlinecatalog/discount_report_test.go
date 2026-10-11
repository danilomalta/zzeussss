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

func reportQ37() DiscountReportQuery {
	return DiscountReportQuery{Active: "all", Status: "all", Limit: 1}
}
func reportRows37(status, price string) *sqlmock.Rows {
	actor, reason, op, action := "", "", "", ""
	if status != "PENDING" {
		actor = fixtureActor.User
		reason = "@reason"
		op = "11111111-1111-4111-8111-111111111112"
		action = "approve"
		if status == "REJECTED" {
			action = "reject"
		}
	}
	return sqlmock.NewRows([]string{"id", "pid", "name", "sku", "active", "price", "version", "percent", "status", "reason", "actor", "review", "op", "action", "created", "updated"}).AddRow(1, 2, " =2+2", "@SKU", true, price, 1, "15.00", status, "+formula", actor, reason, op, action, time.Now().UTC(), time.Now().UTC())
}
func TestDiscountReportValidationAndDeniedRoles(t *testing.T) {
	q := reportQ37()
	for _, status := range []string{"all", "PENDING", "APPROVED", "REJECTED"} {
		q.Status = status
		if ValidateDiscountReport(q) != nil {
			t.Fatal(status)
		}
	}
	for _, bad := range []DiscountReportQuery{{Active: "bad", Status: "all", Limit: 1}, {Active: "all", Status: "bad", Limit: 1}, {Active: "all", Status: "all", Limit: 101}, {Active: "all", Status: "all", Limit: 1, Offset: -1}, {Active: "all", Status: "all", Limit: 1, Query: strings.Repeat("a", 121)}} {
		if ValidateDiscountReport(bad) != ErrInput {
			t.Fatal(bad)
		}
	}
	for _, role := range []string{"stock", "cashier", "employee", "accountant", ""} {
		a := fixtureActor
		a.Role = role
		if _, e := (Store{}).DiscountReport(context.Background(), a, reportQ37()); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestDiscountReportSummaryAndSessionRevalidation(t *testing.T) {
	s, m := qualityFixture(t)
	qualityLive(m)
	q := reportQ37()
	q.Query = "a%_"
	m.ExpectQuery("SELECT count").WithArgs(fixtureActor.Tenant, "all", "%a\\%\\_%").WillReturnRows(sqlmock.NewRows([]string{"total", "pending", "approved", "rejected", "legacy", "generated"}).AddRow(3, 1, 1, 1, 1, time.Now()))
	m.ExpectCommit()
	r, e := s.DiscountReportSummary(context.Background(), fixtureActor, q)
	if e != nil || r.Total != 3 || r.FinalWithoutReceipt != 1 {
		t.Fatal(r, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*online_sessions").WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	if _, e = s.DiscountReport(context.Background(), fixtureActor, q); e != ErrDenied {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestDiscountReportPageAndProtectedCSV(t *testing.T) {
	for _, status := range []string{"PENDING", "APPROVED", "REJECTED"} {
		t.Run(status, func(t *testing.T) {
			s, m := qualityFixture(t)
			qualityLive(m)
			q := reportQ37()
			q.Status = status
			m.ExpectQuery("SELECT count").WithArgs(fixtureActor.Tenant, "all", "", status).WillReturnRows(sqlmock.NewRows([]string{"n", "ts"}).AddRow(2, time.Now()))
			m.ExpectQuery("SELECT d.id").WithArgs(fixtureActor.Tenant, "all", "", status, 1, 0).WillReturnRows(reportRows37(status, "10.25"))
			m.ExpectCommit()
			r, e := s.DiscountReport(context.Background(), fixtureActor, q)
			if e != nil || !r.HasMore || *r.NextOffset != 1 || r.Items[0].DiscountBPS != 1500 || r.Items[0].PriceCents != 1025 || len(r.SourceHash) != 64 || r.Items[0].HasReceipt != (status != "PENDING") {
				t.Fatal(r, e)
			}
			out, e := DiscountReportCSV(r)
			reader := csv.NewReader(strings.NewReader(out))
			reader.Comma = ';'
			csvRows, ce := reader.ReadAll()
			if e != nil || ce != nil || len(csvRows) != 2 || csvRows[1][2] != "'@SKU" || csvRows[1][3] != "' =2+2" || csvRows[1][9] != "'+formula" || csvRows[1][7] != "1500" {
				t.Fatal(out, e, ce)
			}
			if r.Items[0].Name != " =2+2" {
				t.Fatal("report changed stored text")
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestDiscountReportFailsClosed(t *testing.T) {
	for _, kind := range []string{"money", "wrong_status", "count", "query", "commit"} {
		t.Run(kind, func(t *testing.T) {
			s, m := qualityFixture(t)
			qualityLive(m)
			q := reportQ37()
			q.Status = "APPROVED"
			total := 1
			if kind == "count" {
				total = 0
			}
			m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "ts"}).AddRow(total, time.Now()))
			if kind == "query" {
				m.ExpectQuery("SELECT d.id").WillReturnError(errors.New("private"))
			} else {
				status, price := "APPROVED", "1.00"
				if kind == "money" {
					price = "0.001"
				}
				if kind == "wrong_status" {
					status = "REJECTED"
				}
				m.ExpectQuery("SELECT d.id").WillReturnRows(reportRows37(status, price))
			}
			if kind == "commit" {
				m.ExpectCommit().WillReturnError(errors.New("private"))
			} else {
				m.ExpectRollback()
			}
			if _, e := s.DiscountReport(context.Background(), fixtureActor, q); e != ErrUnavailable {
				t.Fatal(e)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "ts"}).AddRow(0, time.Now()))
	m.ExpectQuery("SELECT d.id").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectCommit()
	p, e := s.DiscountReport(context.Background(), fixtureActor, reportQ37())
	if e != nil || p.Items == nil || len(p.Items) != 0 || p.HasMore {
		t.Fatal(p, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"total", "pending", "approved", "rejected", "legacy", "generated"}).AddRow(1, 1, 1, 0, 0, time.Now()))
	m.ExpectRollback()
	if _, e = s.DiscountReportSummary(context.Background(), fixtureActor, reportQ37()); e != ErrUnavailable {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
