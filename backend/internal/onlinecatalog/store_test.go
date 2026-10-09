package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
	"time"
)

var fixtureActor = Actor{Tenant: "22222222-2222-4222-8222-222222222222", User: "33333333-3333-4333-8333-333333333333", Session: "44444444-4444-4444-8444-444444444444", Role: "owner"}

func mockStore(t *testing.T) (Store, sqlmock.Sqlmock) {
	t.Helper()
	db, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		db.Close()
		if e := m.ExpectationsWereMet(); e != nil {
			t.Error(e)
		}
	})
	return Store{DB: db}, m
}
func lockActor(m sqlmock.Sqlmock) {
	m.ExpectBegin()
	m.ExpectQuery(`SELECT s.id::text`).WithArgs(fixtureActor.Session, fixtureActor.Tenant, fixtureActor.User, fixtureActor.Role).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fixtureActor.Session))
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(fixtureActor.Tenant + ":" + testOp).WillReturnResult(sqlmock.NewResult(0, 1))
}
func missingReceipt(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT operation_id::text`).WithArgs(fixtureActor.Tenant, testOp).WillReturnError(sql.ErrNoRows)
}
func productRow(m sqlmock.Sqlmock, version int64) {
	m.ExpectQuery(`SELECT id,nome`).WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "descricao", "sku", "preco", "ativo", "catalog_version"}).AddRow(1, "Item", "", "ABC", "2.50", true, version))
}
func verifiedRow(m sqlmock.Sqlmock, p Product) {
	m.ExpectQuery(`SELECT id,nome`).WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "descricao", "sku", "preco", "ativo", "catalog_version"}).AddRow(p.ID, p.Name, p.Description, p.SKU, fmt.Sprintf("%d.%02d", p.PriceCents/100, p.PriceCents%100), p.Active, p.Version))
}
func TestAllMutationsPublishSnapshotAndEventOnlyAfterCommit(t *testing.T) {
	for _, action := range []string{"details", "price", "active"} {
		t.Run(action, func(t *testing.T) {
			s, m := mockStore(t)
			c := Change{OperationID: testOp, ExpectedVersion: 1, Name: "New", Description: "Description", SKU: "NEW", PriceCents: MaxPrice, Active: false}
			lockActor(m)
			missingReceipt(m)
			productRow(m, 1)
			m.ExpectExec(`UPDATE products SET`).WillReturnResult(sqlmock.NewResult(0, 1))
			after := Product{ID: 1, Name: "Item", SKU: "ABC", PriceCents: 250, Active: true, Version: 2}
			switch action {
			case "details":
				after.Name = c.Name
				after.Description = c.Description
				after.SKU = c.SKU
			case "price":
				after.PriceCents = c.PriceCents
			case "active":
				after.Active = c.Active
			}
			verifiedRow(m, after)
			m.ExpectQuery(`INSERT INTO online_catalog_operations`).WithArgs(fixtureActor.Tenant, testOp, int64(1), fixtureActor.User, action, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))
			m.ExpectExec(`INSERT INTO online_catalog_outbox`).WithArgs(fixtureActor.Tenant, testOp, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
			r, e := s.Mutate(context.Background(), fixtureActor, 1, action, c)
			if e != nil || r.Before.Version != 1 || r.After.Version != 2 || r.Before.PriceCents != 250 {
				t.Fatal(r, e)
			}
			if action == "price" && r.After.PriceCents != MaxPrice {
				t.Fatal("inexact price")
			}
			if action == "active" && r.After.Active {
				t.Fatal("inactive not preserved")
			}
		})
	}
}
func TestMutationFailuresRollbackAndNeverReturnReceipt(t *testing.T) {
	for _, failure := range []string{"stale", "duplicate-sku", "audit", "event", "event-empty", "commit"} {
		t.Run(failure, func(t *testing.T) {
			s, m := mockStore(t)
			lockActor(m)
			missingReceipt(m)
			productRow(m, 1)
			c := Change{OperationID: testOp, ExpectedVersion: 1, PriceCents: 300}
			want := ErrUnavailable
			if failure == "stale" {
				c.ExpectedVersion = 2
				want = ErrConflict
				m.ExpectRollback()
			} else {
				update := m.ExpectExec(`UPDATE products SET preco`)
				if failure == "duplicate-sku" {
					update.WillReturnError(&pgconn.PgError{Code: "23505"})
					want = ErrConflict
					m.ExpectRollback()
				} else {
					update.WithArgs(fixtureActor.Tenant, int64(1), "3.00", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
					verifiedRow(m, Product{ID: 1, Name: "Item", SKU: "ABC", PriceCents: 300, Active: true, Version: 2})
					audit := m.ExpectQuery(`INSERT INTO online_catalog_operations`)
					if failure == "audit" {
						audit.WillReturnError(errors.New("private detail"))
						m.ExpectRollback()
					} else {
						audit.WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))
						event := m.ExpectExec(`INSERT INTO online_catalog_outbox`)
						if failure == "event" {
							event.WillReturnError(errors.New("private event"))
							m.ExpectRollback()
						} else if failure == "event-empty" {
							event.WillReturnResult(sqlmock.NewResult(0, 0))
							m.ExpectRollback()
						} else {
							event.WillReturnResult(sqlmock.NewResult(0, 1))
							m.ExpectCommit().WillReturnError(errors.New("commit connection lost"))
						}
					}
				}
			}
			r, e := s.Mutate(context.Background(), fixtureActor, 1, "price", c)
			if !errors.Is(e, want) || r.OperationID != "" {
				t.Fatal("uncertain write returned success", r, e)
			}
		})
	}
}
func TestOperationReplayBypassesCurrentVersionButRejectsChangedPayload(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "same", true: "changed"}[changed], func(t *testing.T) {
			s, m := mockStore(t)
			lockActor(m)
			c := Change{OperationID: testOp, ExpectedVersion: 1, PriceCents: 300}
			before := Product{ID: 1, Version: 1, PriceCents: 250}
			after := before
			after.Version = 2
			after.PriceCents = 300
			b, _ := json.Marshal(before)
			a, _ := json.Marshal(after)
			m.ExpectQuery(`SELECT operation_id::text`).WillReturnRows(sqlmock.NewRows([]string{"op", "id", "actor", "action", "before", "after", "created", "hash"}).AddRow(testOp, 1, fixtureActor.User, "price", b, a, time.Now(), c.hash("price", 1)))
			if changed {
				c.PriceCents = 301
				m.ExpectRollback()
			} else {
				m.ExpectCommit()
			}
			r, e := s.Mutate(context.Background(), fixtureActor, 1, "price", c)
			if changed {
				if !errors.Is(e, ErrConflict) {
					t.Fatal(e)
				}
			} else if e != nil || r.After.PriceCents != 300 {
				t.Fatal(r, e)
			}
		})
	}
}
func TestSessionRevocationAndStockRolePreventWrites(t *testing.T) {
	s, m := mockStore(t)
	m.ExpectBegin()
	m.ExpectQuery(`SELECT s.id::text`).WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	if _, e := s.Mutate(context.Background(), fixtureActor, 1, "price", Change{OperationID: testOp, ExpectedVersion: 1}); !errors.Is(e, ErrDenied) {
		t.Fatal(e)
	}
	stock := fixtureActor
	stock.Role = "stock"
	if _, e := s.Mutate(context.Background(), stock, 1, "price", Change{}); !errors.Is(e, ErrDenied) {
		t.Fatal(e)
	}
}
func TestReceiptLookupRequiresSameTenantAndActor(t *testing.T) {
	s, m := mockStore(t)
	m.ExpectQuery(`SELECT operation_id::text.*AND actor_id=\$3`).WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnError(sql.ErrNoRows)
	if _, e := s.Receipt(context.Background(), fixtureActor, testOp); !errors.Is(e, ErrMissing) {
		t.Fatal(e)
	}
}
