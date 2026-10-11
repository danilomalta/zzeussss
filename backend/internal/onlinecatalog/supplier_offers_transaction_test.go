package onlinecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"testing"
	"time"
)

func offerFixture48() (SupplierOfferInput, SupplierOfferReceipt) {
	at := time.Date(2026, 10, 11, 14, 0, 0, 0, time.UTC)
	v := SupplierOfferInput{OperationID: testOp, SupplierID: testOp, ProductID: 1, SupplierSKU: "SUP-1", UnitPriceCents: 250, MinimumUnits: 2, LeadTimeDays: 3, Active: true, Reason: "Condições declaradas"}
	p := SupplierOffer{ID: testOp, SupplierID: v.SupplierID, ProductID: v.ProductID, SupplierSKU: v.SupplierSKU, UnitPriceCents: v.UnitPriceCents, MinimumUnits: v.MinimumUnits, LeadTimeDays: v.LeadTimeDays, Active: true, Version: 1, CreatorID: fixtureActor.User, LastActorID: fixtureActor.User, Reason: v.Reason, CreatedAt: at, UpdatedAt: at}
	return v, SupplierOfferReceipt{OperationID: testOp, OfferID: testOp, ActorID: fixtureActor.User, Kind: "create", After: p}
}
func offerRows48(p SupplierOffer) *sqlmock.Rows {
	b, _ := json.Marshal(p)
	return sqlmock.NewRows([]string{"id", "supplier", "product", "version", "active", "payload"}).AddRow(p.ID, p.SupplierID, p.ProductID, p.Version, p.Active, b)
}
func offerReceiptRows48(r SupplierOfferReceipt, h string, empty bool) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"op", "id", "actor", "kind", "hash", "receipt"})
	if !empty {
		b, _ := json.Marshal(r)
		rows.AddRow(r.OperationID, r.OfferID, r.ActorID, r.Kind, h, b)
	}
	return rows
}
func offerParents48(m sqlmock.Sqlmock) {
	_, r := supplierFixture47()
	m.ExpectQuery("SELECT supplier_id::text.*FOR SHARE").WithArgs(fixtureActor.Tenant, testOp).WillReturnRows(supplierRows47(r.After))
	m.ExpectQuery("SELECT id,nome.*FOR SHARE").WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "descricao", "sku", "preco", "ativo", "version"}).AddRow(1, "Item", "", "ABC", "2.50", true, 1))
}
func TestSupplierOfferSaveReadbackAtomicOutbox(t *testing.T) {
	for _, outcome := range []string{"success", "outbox-failure", "ignored-insert", "forged-readback", "ignored-audit", "forged-outbox"} {
		t.Run(outcome, func(t *testing.T) {
			s, m := qualityFixture(t)
			v, r := offerFixture48()
			h := proposalHash(v.OperationID, "create", v)
			qualityLive(m)
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT operation_id::text").WillReturnRows(offerReceiptRows48(r, h, true))
			offerParents48(m)
			m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.After.CreatedAt))
			count := int64(1)
			if outcome == "ignored-insert" {
				count = 0
			}
			m.ExpectExec("INSERT INTO online_supplier_offers").WillReturnResult(sqlmock.NewResult(0, count))
			if outcome != "ignored-insert" {
				actual := r.After
				if outcome == "forged-readback" {
					actual.SupplierSKU += "changed"
				}
				m.ExpectQuery("SELECT offer_id::text").WillReturnRows(offerRows48(actual))
				if outcome != "forged-readback" {
					count = 1
					if outcome == "ignored-audit" {
						count = 0
					}
					m.ExpectExec("INSERT INTO online_supplier_offer_operations").WillReturnResult(sqlmock.NewResult(0, count))
					if outcome != "ignored-audit" {
						m.ExpectQuery("SELECT operation_id::text").WillReturnRows(offerReceiptRows48(r, h, false))
						if outcome == "outbox-failure" {
							m.ExpectExec("INSERT INTO online_supplier_offer_outbox").WillReturnError(errors.New("private details"))
						} else {
							m.ExpectExec("INSERT INTO online_supplier_offer_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
							out := r
							if outcome == "forged-outbox" {
								out.After.SupplierSKU += "changed"
							}
							b, _ := json.Marshal(out)
							m.ExpectQuery("SELECT event_type,payload").WillReturnRows(sqlmock.NewRows([]string{"event", "payload"}).AddRow("supplier.offer.create", b))
						}
					}
				}
			}
			if outcome == "success" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.CreateSupplierOffer(context.Background(), fixtureActor, v)
			if outcome == "success" && (e != nil || !reflect.DeepEqual(got, r)) || outcome != "success" && e != ErrUnavailable {
				t.Fatal(got, e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestSupplierOfferReplayAndScopedConflict(t *testing.T) {
	for _, outcome := range []string{"replay", "changed-input", "other-actor"} {
		s, m := qualityFixture(t)
		v, r := offerFixture48()
		h := proposalHash(v.OperationID, "create", v)
		if outcome == "changed-input" {
			v.SupplierSKU += "changed"
		}
		if outcome == "other-actor" {
			r.ActorID = "11111111-1111-4111-8111-111111111115"
			r.After.CreatorID = r.ActorID
			r.After.LastActorID = r.ActorID
		}
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(offerReceiptRows48(r, h, false))
		if outcome == "replay" {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		_, e := s.CreateSupplierOffer(context.Background(), fixtureActor, v)
		if outcome == "replay" && e != nil || outcome != "replay" && e != ErrConflict {
			t.Fatal(outcome, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestSupplierOfferUpdateRejectsStaleRevisionBeforeWrites(t *testing.T) {
	s, m := qualityFixture(t)
	v, r := offerFixture48()
	v.OperationID = "11111111-1111-4111-8111-111111111115"
	v.ExpectedVersion = 2
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(offerReceiptRows48(r, "", true))
	m.ExpectQuery("SELECT offer_id::text.*FOR UPDATE").WillReturnRows(offerRows48(r.After))
	m.ExpectRollback()
	if _, e := s.UpdateSupplierOffer(context.Background(), fixtureActor, r.OfferID, v); e != ErrConflict {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestSupplierOfferDeniedSessionAndRoles(t *testing.T) {
	s, m := qualityFixture(t)
	v, _ := offerFixture48()
	m.ExpectBegin()
	m.ExpectQuery("SELECT s.id").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectRollback()
	if _, e := s.CreateSupplierOffer(context.Background(), fixtureActor, v); e != ErrDenied {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"stock", "cashier", "employee", "accountant"} {
		a := fixtureActor
		a.Role = role
		if _, e := s.CreateSupplierOffer(context.Background(), a, v); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestSupplierOfferUpdateAuditsBeforeAfterAndOutbox(t *testing.T) {
	s, m := qualityFixture(t)
	v, r := offerFixture48()
	before := r.After
	v.OperationID = "11111111-1111-4111-8111-111111111115"
	v.ExpectedVersion = 1
	v.Active = false
	v.Reason = "Inativado"
	r.OperationID = v.OperationID
	r.Kind = "update"
	r.Before = &before
	r.After.Version = 2
	r.After.Active = false
	r.After.Reason = v.Reason
	r.After.UpdatedAt = before.CreatedAt.Add(time.Second)
	h := proposalHash(r.OfferID, "update", v)
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(offerReceiptRows48(r, h, true))
	m.ExpectQuery("SELECT offer_id::text.*FOR UPDATE").WillReturnRows(offerRows48(before))
	m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.After.UpdatedAt))
	m.ExpectExec("UPDATE online_supplier_offers").WithArgs(fixtureActor.Tenant, r.OfferID, int64(2), false, sqlmock.AnyArg(), int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT offer_id::text").WillReturnRows(offerRows48(r.After))
	m.ExpectExec("INSERT INTO online_supplier_offer_operations").WithArgs(fixtureActor.Tenant, v.OperationID, r.OfferID, fixtureActor.User, "update", h, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(offerReceiptRows48(r, h, false))
	m.ExpectExec("INSERT INTO online_supplier_offer_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
	b, _ := json.Marshal(r)
	m.ExpectQuery("SELECT event_type,payload").WillReturnRows(sqlmock.NewRows([]string{"event", "payload"}).AddRow("supplier.offer.update", b))
	m.ExpectCommit()
	got, e := s.UpdateSupplierOffer(context.Background(), fixtureActor, r.OfferID, v)
	if e != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestSupplierOfferPageSnapshotAndCorruptData(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		s, m := qualityFixture(t)
		_, r := offerFixture48()
		m.ExpectBegin()
		m.ExpectQuery("SELECT s.id").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fixtureActor.Session))
		m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		p := r.After
		if corrupt {
			p.Version = 0
		}
		m.ExpectQuery("SELECT offer_id::text").WillReturnRows(offerRows48(p))
		if corrupt {
			m.ExpectRollback()
		} else {
			m.ExpectCommit()
		}
		got, e := s.SupplierOffers(context.Background(), fixtureActor, SupplierOfferQuery{Active: "all", Limit: 1})
		if corrupt && e != ErrUnavailable || !corrupt && (e != nil || got.Total != 1 || len(got.Items) != 1 || got.HasMore) {
			t.Fatal(got, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestSupplierOfferRejectsInactiveSupplierAndMissingProductBeforeWrites(t *testing.T) {
	for _, missing := range []bool{false, true} {
		s, m := qualityFixture(t)
		v, r := offerFixture48()
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(offerReceiptRows48(r, "", true))
		_, sr := supplierFixture47()
		sr.After.Active = missing
		m.ExpectQuery("SELECT supplier_id::text.*FOR SHARE").WillReturnRows(supplierRows47(sr.After))
		want := ErrConflict
		if missing {
			m.ExpectQuery("SELECT id,nome.*FOR SHARE").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}))
			want = ErrMissing
		}
		m.ExpectRollback()
		if _, e := s.CreateSupplierOffer(context.Background(), fixtureActor, v); e != want {
			t.Fatal(e, want)
		}
		if e := m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
