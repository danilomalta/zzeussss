package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func batchFixture() BatchInput {
	price := int64(375)
	active := false
	return BatchInput{OperationID: testOp, Items: []BatchItem{{ProductID: 1, Action: "price", ExpectedVersion: 1, PriceCents: &price}, {ProductID: 2, Action: "active", ExpectedVersion: 1, Active: &active}}}
}
func batchProduct(id int64) Product {
	return Product{ID: id, Name: "Item", SKU: fmt.Sprint(id), PriceCents: 250, Active: true, Version: 1}
}
func batchRow(m sqlmock.Sqlmock, p Product) {
	m.ExpectQuery(`SELECT id,nome`).WithArgs(fixtureActor.Tenant, p.ID).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}).AddRow(p.ID, p.Name, p.Description, p.SKU, fmt.Sprintf("%d.%02d", p.PriceCents/100, p.PriceCents%100), p.Active, p.Version))
}
func batchLock(m sqlmock.Sqlmock, apply bool) {
	m.ExpectBegin()
	m.ExpectQuery(`SELECT s.id::text`).WithArgs(fixtureActor.Session, fixtureActor.Tenant, fixtureActor.User, fixtureActor.Role).WillReturnRows(sqlmock.NewRows([]string{"live"}).AddRow(fixtureActor.Session))
	if apply {
		m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(fixtureActor.Tenant + ":batch:" + testOp).WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery(`SELECT operation_id::text.*online_catalog_batches`).WithArgs(fixtureActor.Tenant, testOp).WillReturnError(sql.ErrNoRows)
		for _, id := range []int64{1, 2} {
			m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(fixtureActor.Tenant + ":" + batchChild(testOp, id)).WillReturnResult(sqlmock.NewResult(0, 1))
		}
	}
}
func TestBatchPreviewHasNoCommercialWritesAndExactSnapshots(t *testing.T) {
	s, m := mockStore(t)
	batchLock(m, false)
	batchRow(m, batchProduct(1))
	batchRow(m, batchProduct(2))
	m.ExpectCommit()
	p, e := s.PreviewBatch(context.Background(), fixtureActor, batchFixture())
	if e != nil || len(p.Items) != 2 || len(p.PreviewHash) != 64 || p.Items[0].Before.PriceCents != 250 || p.Items[0].After.PriceCents != 375 || p.Items[1].After.Active {
		t.Fatal(p, e)
	}
}
func batchPreviewFixture(b BatchInput) BatchPreview {
	p := BatchPreview{OperationID: b.OperationID, Items: []BatchPreviewItem{{Action: "price", Before: batchProduct(1), After: batchProduct(1)}, {Action: "active", Before: batchProduct(2), After: batchProduct(2)}}}
	p.Items[0].After.PriceCents = *b.Items[0].PriceCents
	p.Items[0].After.Version = 2
	p.Items[1].After.Active = false
	p.Items[1].After.Version = 2
	raw, _ := json.Marshal(struct {
		Tenant, Actor, Operation string
		Items                    []BatchPreviewItem
	}{fixtureActor.Tenant, fixtureActor.User, b.OperationID, p.Items})
	p.PreviewHash = creationHash(raw)
	return p
}
func expectBatchChild(m sqlmock.Sqlmock, v BatchItem, p BatchPreviewItem, fail bool) {
	op := batchChild(testOp, v.ProductID)
	m.ExpectQuery(`SELECT s.id::text`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fixtureActor.Session))
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(fixtureActor.Tenant + ":" + op).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT operation_id::text.*online_catalog_operations`).WithArgs(fixtureActor.Tenant, op).WillReturnError(sql.ErrNoRows)
	batchRow(m, p.Before)
	update := m.ExpectExec(`UPDATE products SET`)
	if v.Action == "price" {
		update.WithArgs(fixtureActor.Tenant, v.ProductID, "3.75", int64(1))
	} else {
		update.WithArgs(fixtureActor.Tenant, v.ProductID, false, int64(1))
	}
	update.WillReturnResult(sqlmock.NewResult(0, 1))
	batchRow(m, p.After)
	m.ExpectQuery(`INSERT INTO online_catalog_operations`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
	event := m.ExpectExec(`INSERT INTO online_catalog_outbox`)
	if fail {
		event.WillReturnError(errors.New("private failure"))
	} else {
		event.WillReturnResult(sqlmock.NewResult(0, 1))
	}
}
func TestBatchOneCommitOrRollbackAllIncludingEarlierItem(t *testing.T) {
	for _, mode := range []string{"success", "second-event", "batch-audit", "commit", "preview-conflict", "stale-second"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			b := batchFixture()
			p := batchPreviewFixture(b)
			b.PreviewHash = p.PreviewHash
			batchLock(m, true)
			batchRow(m, batchProduct(1))
			second := batchProduct(2)
			if mode == "stale-second" {
				second.Version = 2
			}
			batchRow(m, second)
			want := ErrUnavailable
			if mode == "preview-conflict" {
				b.PreviewHash = strings.Repeat("0", 64)
				want = ErrConflict
				m.ExpectRollback()
			} else if mode == "stale-second" {
				want = ErrConflict
				m.ExpectRollback()
			} else {
				expectBatchChild(m, b.Items[0], p.Items[0], false)
				expectBatchChild(m, b.Items[1], p.Items[1], mode == "second-event")
				if mode == "second-event" {
					m.ExpectRollback()
				} else {
					insert := m.ExpectQuery(`INSERT INTO online_catalog_batches`)
					if mode == "batch-audit" {
						insert.WillReturnError(errors.New("private audit"))
						m.ExpectRollback()
					} else {
						insert.WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
						if mode == "commit" {
							m.ExpectCommit().WillReturnError(errors.New("reply lost"))
						} else {
							m.ExpectCommit()
						}
					}
				}
			}
			r, e := s.ApplyBatch(context.Background(), fixtureActor, b)
			if mode == "success" {
				if e != nil || len(r.Items) != 2 || r.CreatedAt.IsZero() {
					t.Fatal(r, e)
				}
			} else if !errors.Is(e, want) || r.OperationID != "" {
				t.Fatal("failure returned receipt", r, e)
			}
		})
	}
}
func TestBatchReplayWithoutCheckingCurrentProductsAndActorConflict(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			s, m := mockStore(t)
			b := batchFixture()
			p := batchPreviewFixture(b)
			b.PreviewHash = p.PreviewHash
			r := BatchReceipt{OperationID: testOp, ActorID: fixtureActor.User, PreviewHash: p.PreviewHash}
			for _, item := range p.Items {
				r.Items = append(r.Items, Receipt{OperationID: batchChild(testOp, item.Before.ID), ProductID: item.Before.ID, ActorID: fixtureActor.User, Action: item.Action, Before: item.Before, After: item.After})
			}
			raw, _ := json.Marshal(r)
			payload, _ := json.Marshal(b)
			m.ExpectBegin()
			m.ExpectQuery(`SELECT s.id::text`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fixtureActor.Session))
			m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_batches`).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "snapshot", "time", "hash"}).AddRow(testOp, fixtureActor.User, raw, time.Now(), creationHash(payload)))
			if changed {
				price := int64(376)
				b.Items[0].PriceCents = &price
				m.ExpectRollback()
			} else {
				m.ExpectCommit()
			}
			result, e := s.ApplyBatch(context.Background(), fixtureActor, b)
			if changed {
				if e != ErrConflict {
					t.Fatal(e)
				}
			} else if e != nil || len(result.Items) != 2 {
				t.Fatal(result, e)
			}
		})
	}
}
func TestBatchDecoderRejectsAmbiguousInputsAndSortsWithoutMutatingCaller(t *testing.T) {
	valid := fmt.Sprintf(`{"operation_id":%q,"items":[{"product_id":2,"action":"active","expected_version":1,"ativo":false},{"product_id":1,"action":"price","expected_version":1,"price_cents":375}]}`, testOp)
	b, e := DecodeBatch("application/json", []byte(valid), false)
	if e != nil || b.Items[0].ProductID != 1 || b.Items[1].Active == nil || *b.Items[1].Active {
		t.Fatal(b, e)
	}
	invalid := []string{
		strings.Replace(valid, `"product_id":2`, `"product_id":2,"product_id":3`, 1),
		strings.Replace(valid, `"product_id":2`, `"product_id":1`, 1),
		strings.Replace(valid, `"ativo":false`, `"ativo":null`, 1),
		strings.Replace(valid, `"price_cents":375`, `"price_cents":3.75`, 1),
		strings.Replace(valid, `"price_cents":375`, `"price_cents":-1`, 1),
		strings.Replace(valid, `"items":`, `"tenant_id":"foreign","items":`, 1),
		strings.Replace(valid, `"action":"price"`, `"action":"details"`, 1),
		valid + `{}`, `{"operation_id":"x","items":[]}`,
		strings.Replace(valid, `"expected_version":1`, `"expected_version":9007199254740991`, 1),
	}
	for _, raw := range invalid {
		if _, e := DecodeBatch("application/json", []byte(raw), false); e != ErrInput {
			t.Fatal("invalid accepted")
		}
	}
	if _, e := DecodeBatch("text/plain", []byte(valid), false); e != ErrInput {
		t.Fatal(e)
	}
	if _, e := DecodeBatch("application/json", []byte(valid), true); e != ErrInput {
		t.Fatal("apply without preview")
	}
	many := batchFixture()
	many.Items = make([]BatchItem, 101)
	if _, e := normalizeBatch(many, false); e != ErrInput {
		t.Fatal("limit")
	}
	s, m := mockStore(t)
	stock := fixtureActor
	stock.Role = "stock"
	if _, e := s.PreviewBatch(context.Background(), stock, b); e != ErrDenied {
		t.Fatal(e)
	}
	m.ExpectQuery(`SELECT operation_id::text.*AND actor_id=\$3`).WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnError(sql.ErrNoRows)
	if _, e := s.Batch(context.Background(), fixtureActor, testOp); e != ErrMissing {
		t.Fatal(e)
	}
}

func TestBatchRevokedSessionAndEmptyHistoryFailClosed(t *testing.T) {
	s, m := mockStore(t)
	b := batchFixture()
	b.PreviewHash = batchPreviewFixture(b).PreviewHash
	m.ExpectBegin()
	m.ExpectQuery(`SELECT s.id::text`).WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	if _, e := s.ApplyBatch(context.Background(), fixtureActor, b); e != ErrDenied {
		t.Fatal("revoked actor wrote batch", e)
	}
	m.ExpectQuery(`SELECT operation_id::text.*WHERE tenant_id=\$1 ORDER BY`).WithArgs(fixtureActor.Tenant, 2, 10).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "snapshot", "time", "hash"}))
	items, e := s.BatchHistory(context.Background(), fixtureActor, 2, 10)
	if e != nil || items == nil || len(items) != 0 {
		t.Fatal("history empty state", items, e)
	}
	if _, e := s.BatchHistory(context.Background(), fixtureActor, 11, 0); e != ErrInput {
		t.Fatal("unbounded history", e)
	}
}
