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

func supplierFixture47() (SupplierInput, SupplierReceipt) {
	at := time.Date(2026, 10, 11, 14, 0, 0, 0, time.UTC)
	v := SupplierInput{OperationID: testOp, Name: "Fornecedor", Email: "", Phone: "", Active: true, Reason: "Cadastro"}
	p := Supplier{ID: testOp, Name: v.Name, Active: true, Version: 1, CreatorID: fixtureActor.User, LastActorID: fixtureActor.User, Reason: v.Reason, CreatedAt: at, UpdatedAt: at}
	return v, SupplierReceipt{OperationID: testOp, SupplierID: testOp, ActorID: fixtureActor.User, Kind: "create", After: p}
}
func supplierRows47(p Supplier) *sqlmock.Rows {
	b, _ := json.Marshal(p)
	return sqlmock.NewRows([]string{"id", "version", "active", "payload"}).AddRow(p.ID, p.Version, p.Active, b)
}
func supplierReceiptRows47(r SupplierReceipt, h string, empty bool) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"op", "id", "actor", "kind", "hash", "receipt"})
	if !empty {
		b, _ := json.Marshal(r)
		rows.AddRow(r.OperationID, r.SupplierID, r.ActorID, r.Kind, h, b)
	}
	return rows
}
func TestSupplierSaveReadbackAtomicOutbox(t *testing.T) {
	for _, outcome := range []string{"success", "outbox-failure", "ignored-insert", "forged-readback", "ignored-audit", "forged-outbox"} {
		t.Run(outcome, func(t *testing.T) {
			s, m := qualityFixture(t)
			v, r := supplierFixture47()
			h := proposalHash(v.OperationID, "create", v)
			qualityLive(m)
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT operation_id::text").WillReturnRows(supplierReceiptRows47(r, h, true))
			m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.After.CreatedAt))
			count := int64(1)
			if outcome == "ignored-insert" {
				count = 0
			}
			m.ExpectExec("INSERT INTO online_suppliers").WillReturnResult(sqlmock.NewResult(0, count))
			if outcome != "ignored-insert" {
				actual := r.After
				if outcome == "forged-readback" {
					actual.Name += "changed"
				}
				m.ExpectQuery("SELECT supplier_id::text").WillReturnRows(supplierRows47(actual))
				if outcome != "forged-readback" {
					count = 1
					if outcome == "ignored-audit" {
						count = 0
					}
					m.ExpectExec("INSERT INTO online_supplier_operations").WillReturnResult(sqlmock.NewResult(0, count))
					if outcome != "ignored-audit" {
						m.ExpectQuery("SELECT operation_id::text").WillReturnRows(supplierReceiptRows47(r, h, false))
						if outcome == "outbox-failure" {
							m.ExpectExec("INSERT INTO online_supplier_outbox").WillReturnError(errors.New("private details"))
						} else {
							m.ExpectExec("INSERT INTO online_supplier_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
							out := r
							if outcome == "forged-outbox" {
								out.After.Name += "changed"
							}
							b, _ := json.Marshal(out)
							m.ExpectQuery("SELECT event_type,payload").WillReturnRows(sqlmock.NewRows([]string{"event", "payload"}).AddRow("supplier.create", b))
						}
					}
				}
			}
			if outcome == "success" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.CreateSupplier(context.Background(), fixtureActor, v)
			if outcome == "success" && (e != nil || !reflect.DeepEqual(got, r)) || outcome != "success" && e != ErrUnavailable {
				t.Fatal(got, e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestSupplierReplayAndScopedConflict(t *testing.T) {
	for _, outcome := range []string{"replay", "changed-input", "other-actor"} {
		s, m := qualityFixture(t)
		v, r := supplierFixture47()
		h := proposalHash(v.OperationID, "create", v)
		if outcome == "changed-input" {
			v.Name += "changed"
		}
		if outcome == "other-actor" {
			r.ActorID = "11111111-1111-4111-8111-111111111115"
			r.After.CreatorID = r.ActorID
			r.After.LastActorID = r.ActorID
		}
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(supplierReceiptRows47(r, h, false))
		if outcome == "replay" {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		_, e := s.CreateSupplier(context.Background(), fixtureActor, v)
		if outcome == "replay" && e != nil || outcome != "replay" && e != ErrConflict {
			t.Fatal(outcome, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestSupplierUpdateRejectsStaleRevisionBeforeWrites(t *testing.T) {
	s, m := qualityFixture(t)
	v, r := supplierFixture47()
	v.OperationID = "11111111-1111-4111-8111-111111111115"
	v.ExpectedVersion = 2
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(supplierReceiptRows47(r, "", true))
	m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.After.CreatedAt))
	m.ExpectQuery("SELECT supplier_id::text.*FOR UPDATE").WillReturnRows(supplierRows47(r.After))
	m.ExpectRollback()
	if _, e := s.UpdateSupplier(context.Background(), fixtureActor, r.SupplierID, v); e != ErrConflict {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestSupplierDeniedSessionAndRoles(t *testing.T) {
	s, m := qualityFixture(t)
	v, _ := supplierFixture47()
	m.ExpectBegin()
	m.ExpectQuery("SELECT s.id").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectRollback()
	if _, e := s.CreateSupplier(context.Background(), fixtureActor, v); e != ErrDenied {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"stock", "cashier", "employee", "accountant"} {
		a := fixtureActor
		a.Role = role
		if _, e := s.CreateSupplier(context.Background(), a, v); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestSupplierUpdateAuditsBeforeAfterAndOutbox(t *testing.T) {
	s, m := qualityFixture(t)
	v, r := supplierFixture47()
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
	h := proposalHash(r.SupplierID, "update", v)
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(supplierReceiptRows47(r, h, true))
	m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.After.UpdatedAt))
	m.ExpectQuery("SELECT supplier_id::text.*FOR UPDATE").WillReturnRows(supplierRows47(before))
	m.ExpectExec("UPDATE online_suppliers").WithArgs(fixtureActor.Tenant, r.SupplierID, int64(2), false, sqlmock.AnyArg(), int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT supplier_id::text").WillReturnRows(supplierRows47(r.After))
	m.ExpectExec("INSERT INTO online_supplier_operations").WithArgs(fixtureActor.Tenant, v.OperationID, r.SupplierID, fixtureActor.User, "update", h, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(supplierReceiptRows47(r, h, false))
	m.ExpectExec("INSERT INTO online_supplier_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
	b, _ := json.Marshal(r)
	m.ExpectQuery("SELECT event_type,payload").WillReturnRows(sqlmock.NewRows([]string{"event", "payload"}).AddRow("supplier.update", b))
	m.ExpectCommit()
	got, e := s.UpdateSupplier(context.Background(), fixtureActor, r.SupplierID, v)
	if e != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestSupplierPageSnapshotAndCorruptData(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		s, m := qualityFixture(t)
		_, r := supplierFixture47()
		m.ExpectBegin()
		m.ExpectQuery("SELECT s.id").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fixtureActor.Session))
		m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		p := r.After
		if corrupt {
			p.Version = 0
		}
		m.ExpectQuery("SELECT supplier_id::text").WillReturnRows(supplierRows47(p))
		if corrupt {
			m.ExpectRollback()
		} else {
			m.ExpectCommit()
		}
		got, e := s.Suppliers(context.Background(), fixtureActor, SupplierQuery{Active: "all", Limit: 1})
		if corrupt && e != ErrUnavailable || !corrupt && (e != nil || got.Total != 1 || len(got.Items) != 1 || got.HasMore) {
			t.Fatal(got, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
