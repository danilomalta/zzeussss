package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
	"time"
)

func createStart(m sqlmock.Sqlmock) {
	m.ExpectBegin()
	m.ExpectQuery(`SELECT s.id::text`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fixtureActor.Session))
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(fixtureActor.Tenant + ":create:" + testOp).WillReturnResult(sqlmock.NewResult(0, 1))
}
func createProductRow(m sqlmock.Sqlmock) {
	m.ExpectQuery(`INSERT INTO products`).WithArgs(fixtureActor.Tenant, "Item", "", "ABC", "9999999999.99", int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version", "stock", "created", "updated"}).AddRow(7, "Item", "", "ABC", "9999999999.99", true, 1, 1, time.Now(), time.Now()))
}
func TestCreationCommitsExactSnapshotAuditAndEvent(t *testing.T) {
	s, m := mockStore(t)
	createStart(m)
	m.ExpectQuery(`SELECT operation_id::text`).WillReturnError(sql.ErrNoRows)
	createProductRow(m)
	m.ExpectQuery(`INSERT INTO online_catalog_creations`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
	m.ExpectExec(`INSERT INTO online_catalog_creation_outbox`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	r, e := s.Create(context.Background(), fixtureActor, testOp, NewProduct{Name: "Item", SKU: "ABC", PriceCents: MaxPrice, Stock: 1})
	if e != nil || r.Snapshot.ID != 7 || r.Snapshot.PriceCents != MaxPrice || r.Snapshot.Stock != 1 {
		t.Fatal(r, e)
	}
}
func TestCreationFailureNeverReturnsSuccess(t *testing.T) {
	for _, stage := range []string{"insert", "audit", "outbox", "ignored-event", "commit"} {
		t.Run(stage, func(t *testing.T) {
			s, m := mockStore(t)
			createStart(m)
			m.ExpectQuery(`SELECT operation_id::text`).WillReturnError(sql.ErrNoRows)
			if stage == "insert" {
				m.ExpectQuery(`INSERT INTO products`).WillReturnError(errors.New("private"))
				m.ExpectRollback()
			} else {
				createProductRow(m)
				audit := m.ExpectQuery(`INSERT INTO online_catalog_creations`)
				if stage == "audit" {
					audit.WillReturnError(errors.New("private"))
					m.ExpectRollback()
				} else {
					audit.WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
					outbox := m.ExpectExec(`INSERT INTO online_catalog_creation_outbox`)
					switch stage {
					case "outbox":
						outbox.WillReturnError(errors.New("private"))
						m.ExpectRollback()
					case "ignored-event":
						outbox.WillReturnResult(sqlmock.NewResult(0, 0))
						m.ExpectRollback()
					case "commit":
						outbox.WillReturnResult(sqlmock.NewResult(0, 1))
						m.ExpectCommit().WillReturnError(errors.New("lost reply"))
					}
				}
			}
			r, e := s.Create(context.Background(), fixtureActor, testOp, NewProduct{Name: "Item", SKU: "ABC", PriceCents: MaxPrice, Stock: 1})
			if e != ErrUnavailable || r.OperationID != "" {
				t.Fatal(r, e)
			}
		})
	}
}
func TestCreationReplayIsStableAndContentConflictDoesNotInsert(t *testing.T) {
	input := NewProduct{Name: "Item", SKU: "ABC", PriceCents: 250}
	raw, _ := json.Marshal(input)
	hash := creationHash(raw)
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "same", true: "changed"}[changed], func(t *testing.T) {
			s, m := mockStore(t)
			createStart(m)
			snapshot, _ := json.Marshal(CreatedProduct{Product: Product{ID: 7, Name: "Item", SKU: "ABC", PriceCents: 250, Active: true, Version: 1}})
			m.ExpectQuery(`SELECT operation_id::text`).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "snapshot", "created", "hash"}).AddRow(testOp, fixtureActor.User, snapshot, time.Now(), hash))
			want := input
			if changed {
				want.PriceCents++
				m.ExpectRollback()
			} else {
				m.ExpectCommit()
			}
			r, e := s.Create(context.Background(), fixtureActor, testOp, want)
			if changed {
				if e != ErrConflict {
					t.Fatal(e)
				}
			} else if e != nil || r.Snapshot.ID != 7 {
				t.Fatal(r, e)
			}
		})
	}
}
func TestCreationInvalidInputAndRoleNeverOpenTransaction(t *testing.T) {
	s, _ := mockStore(t)
	a := fixtureActor
	a.Role = "cashier"
	if _, e := s.Create(context.Background(), a, testOp, NewProduct{Name: "Item", SKU: "ABC"}); e != ErrDenied {
		t.Fatal(e)
	}
	if _, e := s.Create(context.Background(), fixtureActor, testOp, NewProduct{Name: "Item", SKU: "ABC", Stock: -1}); e != ErrInput {
		t.Fatal(e)
	}
}
func TestCreationLookupIsScopedToActor(t *testing.T) {
	s, m := mockStore(t)
	m.ExpectQuery(`SELECT operation_id::text.*AND actor_id=\$3`).WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnError(sql.ErrNoRows)
	if _, e := s.Creation(context.Background(), fixtureActor, testOp); e != ErrMissing {
		t.Fatal(e)
	}
}
