package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func discountFixture36() (DiscountSuggestion, DiscountSuggestion, DiscountReviewInput, DiscountReviewReceipt, string) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	before := DiscountSuggestion{ID: 1, ProductID: 1, DiscountBPS: 1500, SuggestedRange: "10% - 20%", Reason: "Excesso", Criteria: "PRODUTO_PARADO", Status: "PENDING", CreatedAt: now, UpdatedAt: now, Product: Product{ID: 1, Name: "Item", Description: "", SKU: "item", PriceCents: 100, Active: true, Version: 1}}
	after := before
	after.Status = "APPROVED"
	after.ReviewedBy = fixtureActor.User
	after.UpdatedAt = now.Add(time.Second)
	v := DiscountReviewInput{OperationID: testOp, ExpectedProductVersion: 1, ExpectedDiscountBPS: 1500, Reason: "Conferido"}
	raw, _ := json.Marshal(struct {
		ID     int64
		Action string
		Input  DiscountReviewInput
	}{1, "approve", v})
	h := creationHash(raw)
	r := DiscountReviewReceipt{OperationID: testOp, SuggestionID: 1, ActorID: fixtureActor.User, Action: "approve", Reason: v.Reason, Before: before, After: after, CreatedAt: now.Add(2 * time.Second)}
	return before, after, v, r, h
}
func discountRows36(s DiscountSuggestion) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "product_id", "percent", "range", "reason", "criteria", "status", "reviewer", "created", "updated"}).AddRow(s.ID, s.ProductID, "15.00", s.SuggestedRange, s.Reason, s.Criteria, s.Status, s.ReviewedBy, s.CreatedAt, s.UpdatedAt)
}
func reviewRows36(r DiscountReviewReceipt, h string) *sqlmock.Rows {
	b, _ := json.Marshal(r.Before)
	a, _ := json.Marshal(r.After)
	return sqlmock.NewRows([]string{"op", "suggestion", "actor", "action", "reason", "before", "after", "created", "hash"}).AddRow(r.OperationID, r.SuggestionID, r.ActorID, r.Action, r.Reason, b, a, r.CreatedAt, h)
}
func noReviewRows36() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"op", "suggestion", "actor", "action", "reason", "before", "after", "created", "hash"})
}
func TestDiscountReviewStrictInputAndDeniedRoles(t *testing.T) {
	_, _, v, _, _ := discountFixture36()
	b, _ := json.Marshal(v)
	if _, e := DecodeDiscountReview("application/json", b); e != nil {
		t.Fatal(e)
	}
	for _, body := range []string{"{}", string(b) + "{}", strings.Replace(string(b), `"expected_product_version":1`, `"expected_product_version":1.5`, 1), strings.Replace(string(b), `"expected_discount_bps":1500`, `"expected_discount_bps":1500,"expected_discount_bps":1500`, 1), strings.Replace(string(b), `"reason":"Conferido"`, `"reason":null`, 1), strings.Replace(string(b), `"reason":"Conferido"`, `"reason":"   "`, 1), strings.Replace(string(b), "1500", "10001", 1)} {
		if _, e := DecodeDiscountReview("application/json", []byte(body)); e != ErrInput {
			t.Fatal("invalid input accepted", e)
		}
	}
	for _, role := range []string{"cashier", "stock", "employee", "accountant"} {
		a := fixtureActor
		a.Role = role
		if _, e := (Store{}).ReviewDiscount(context.Background(), a, 1, "approve", v); e != ErrDenied {
			t.Fatal(role, e)
		}
		if _, e := (Store{}).DiscountSuggestion(context.Background(), a, 1); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestDiscountReviewAtomicSuccessAndOutboxFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "late_failure"}[fail], func(t *testing.T) {
			s, m := qualityFixture(t)
			before, after, v, r, h := discountFixture36()
			qualityLive(m)
			m.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(fixtureActor.Tenant + ":discount-review:" + testOp).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT operation_id::text").WithArgs(fixtureActor.Tenant, testOp).WillReturnRows(noReviewRows36())
			m.ExpectQuery("SELECT id,product_id.*FOR UPDATE").WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(discountRows36(before))
			m.ExpectQuery("SELECT id,nome.*FOR SHARE").WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}).AddRow(1, "Item", "", "item", "1.00", true, 1))
			m.ExpectExec("UPDATE discount_suggestions").WithArgs(fixtureActor.Tenant, int64(1), "APPROVED", fixtureActor.User).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT id,product_id").WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(discountRows36(after))
			m.ExpectExec("INSERT INTO online_discount_reviews").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT operation_id::text").WithArgs(fixtureActor.Tenant, testOp).WillReturnRows(reviewRows36(r, h))
			if fail {
				m.ExpectExec("INSERT INTO online_discount_review_outbox").WillReturnError(errors.New("private trigger"))
				m.ExpectRollback()
			} else {
				m.ExpectExec("INSERT INTO online_discount_review_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			result, e := s.ReviewDiscount(context.Background(), fixtureActor, 1, "approve", v)
			if fail && e != ErrUnavailable {
				t.Fatal(e)
			}
			if !fail && (e != nil || result.Action != "approve") {
				t.Fatal(result, e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestDiscountReviewReplayDoesNotUpdateAndConflictingActorCannotTakeReceipt(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		s, m := qualityFixture(t)
		_, _, v, r, h := discountFixture36()
		if foreign {
			r.ActorID = "11111111-1111-4111-8111-111111111119"
			r.After.ReviewedBy = r.ActorID
		}
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(reviewRows36(r, h))
		if foreign {
			m.ExpectRollback()
		} else {
			m.ExpectCommit()
		}
		_, e := s.ReviewDiscount(context.Background(), fixtureActor, 1, "approve", v)
		if foreign && e != ErrConflict || !foreign && e != nil {
			t.Fatal(e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestDiscountReviewRejectsChangedVersionPercentInactiveAndReviewed(t *testing.T) {
	for _, kind := range []string{"version", "percent", "inactive", "reviewed", "missing"} {
		s, m := qualityFixture(t)
		before, _, v, _, _ := discountFixture36()
		if kind == "version" {
			v.ExpectedProductVersion = 2
		}
		if kind == "percent" {
			v.ExpectedDiscountBPS = 1000
		}
		if kind == "reviewed" {
			before.Status = "REJECTED"
			before.ReviewedBy = fixtureActor.User
		}
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(noReviewRows36())
		if kind == "missing" {
			m.ExpectQuery("SELECT id,product_id").WillReturnError(sql.ErrNoRows)
		} else {
			m.ExpectQuery("SELECT id,product_id").WillReturnRows(discountRows36(before))
			m.ExpectQuery("SELECT id,nome").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}).AddRow(1, "Item", "", "item", "1.00", kind != "inactive", 1))
		}
		m.ExpectRollback()
		_, e := s.ReviewDiscount(context.Background(), fixtureActor, 1, "approve", v)
		if kind == "missing" && e != ErrMissing || kind != "missing" && e != ErrConflict {
			t.Fatal(kind, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestDiscountReviewReadRevocationAndForgedSnapshot(t *testing.T) {
	s, m := qualityFixture(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*online_sessions").WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	if _, e := s.DiscountReview(context.Background(), fixtureActor, testOp); e != ErrDenied {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	_, _, _, r, h := discountFixture36()
	r.After.Product.Version = 2
	rows := reviewRows36(r, h)
	s, m = qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT operation_id::text").WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnRows(rows)
	m.ExpectRollback()
	if _, e := s.DiscountReview(context.Background(), fixtureActor, testOp); e != ErrUnavailable {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
