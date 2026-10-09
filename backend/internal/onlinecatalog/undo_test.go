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

const undoSourceOp = "55555555-5555-4555-8555-555555555555"

func undoFixture() (UndoInput, BatchReceipt, BatchPreview) {
	before := batchProduct(1)
	after := before
	after.PriceCents = 375
	after.Version = 2
	source := BatchReceipt{OperationID: undoSourceOp, ActorID: fixtureActor.User, PreviewHash: strings.Repeat("a", 64), Items: []Receipt{{OperationID: batchChild(undoSourceOp, 1), ProductID: 1, ActorID: fixtureActor.User, Action: "price", Before: before, After: after}}}
	afterUndo := before
	afterUndo.Version = 3
	p := BatchPreview{OperationID: testOp, Items: []BatchPreviewItem{{Action: "price", Before: after, After: afterUndo}}}
	raw, _ := json.Marshal(struct {
		Tenant, Actor, Operation string
		Items                    []BatchPreviewItem
	}{fixtureActor.Tenant, fixtureActor.User, testOp, p.Items})
	p.PreviewHash = creationHash(raw)
	u := UndoInput{OperationID: testOp, SourceOperationID: undoSourceOp, Reason: "Corrige reajuste"}
	u.PreviewHash = undoPreview(u, p).PreviewHash
	return u, source, p
}
func undoSourceRow(m sqlmock.Sqlmock, source BatchReceipt, excluded bool) {
	raw, _ := json.Marshal(source)
	m.ExpectQuery(`SELECT operation_id::text.*online_catalog_batches`).WithArgs(fixtureActor.Tenant, undoSourceOp).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "snapshot", "created", "hash"}).AddRow(source.OperationID, source.ActorID, raw, time.Now(), strings.Repeat("a", 64)))
	m.ExpectQuery(`SELECT EXISTS.*online_catalog_undos`).WithArgs(fixtureActor.Tenant, undoSourceOp).WillReturnRows(sqlmock.NewRows([]string{"excluded"}).AddRow(excluded))
}
func undoLive(m sqlmock.Sqlmock) {
	m.ExpectQuery(`SELECT s.id::text`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fixtureActor.Session))
}
func undoLock(m sqlmock.Sqlmock, key string) {
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(fixtureActor.Tenant + ":" + key).WillReturnResult(sqlmock.NewResult(0, 1))
}
func TestUndoPreviewRestoresValuesButNeverRollsVersionBack(t *testing.T) {
	s, m := mockStore(t)
	u, source, p := undoFixture()
	u.PreviewHash = ""
	m.ExpectBegin()
	undoLive(m)
	undoSourceRow(m, source, false)
	batchRow(m, source.Items[0].After)
	m.ExpectCommit()
	result, e := s.PreviewUndo(context.Background(), fixtureActor, u)
	if e != nil || result.PreviewHash != undoPreview(u, p).PreviewHash || result.Items[0].After.PriceCents != 250 || result.Items[0].After.Version != 3 {
		t.Fatal(result, e)
	}
}
func TestUndoLateFailuresRollbackCompensationLinkProductsAndEvents(t *testing.T) {
	for _, mode := range []string{"success", "metadata", "outbox", "suppressed-outbox", "commit", "stale", "rewritten"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			u, source, p := undoFixture()
			before := source.Items[0].After
			m.ExpectBegin()
			undoLive(m)
			undoLock(m, "undo:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_undos`).WithArgs(fixtureActor.Tenant, testOp).WillReturnError(sql.ErrNoRows)
			undoLock(m, "undo-source:"+undoSourceOp)
			undoSourceRow(m, source, false)
			undoLock(m, "batch:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_batches`).WithArgs(fixtureActor.Tenant, testOp).WillReturnError(sql.ErrNoRows)
			child := batchChild(testOp, 1)
			undoLock(m, child)
			if mode == "stale" {
				before.Version = 3
			}
			if mode == "rewritten" {
				before.PriceCents = 376
			}
			batchRow(m, before)
			want := ErrUnavailable
			if mode == "stale" || mode == "rewritten" {
				want = ErrConflict
				m.ExpectRollback()
			} else {
				undoLive(m)
				undoLock(m, "batch:"+testOp)
				m.ExpectQuery(`SELECT operation_id::text.*online_catalog_batches`).WithArgs(fixtureActor.Tenant, testOp).WillReturnError(sql.ErrNoRows)
				undoLock(m, child)
				batchRow(m, before)
				undoLive(m)
				undoLock(m, child)
				m.ExpectQuery(`SELECT operation_id::text.*online_catalog_operations`).WithArgs(fixtureActor.Tenant, child).WillReturnError(sql.ErrNoRows)
				batchRow(m, before)
				m.ExpectExec(`UPDATE products SET preco`).WithArgs(fixtureActor.Tenant, int64(1), "2.50", int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				batchRow(m, p.Items[0].After)
				m.ExpectQuery(`INSERT INTO online_catalog_operations`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
				m.ExpectExec(`INSERT INTO online_catalog_outbox`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery(`INSERT INTO online_catalog_batches`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
				link := m.ExpectQuery(`INSERT INTO online_catalog_undos`)
				if mode == "metadata" {
					link.WillReturnError(errors.New("private"))
					m.ExpectRollback()
				} else {
					link.WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
					event := m.ExpectExec(`INSERT INTO online_catalog_undo_outbox`)
					if mode == "outbox" {
						event.WillReturnError(errors.New("private"))
						m.ExpectRollback()
					} else if mode == "suppressed-outbox" {
						event.WillReturnResult(sqlmock.NewResult(0, 0))
						m.ExpectRollback()
					} else {
						event.WillReturnResult(sqlmock.NewResult(0, 1))
						if mode == "commit" {
							m.ExpectCommit().WillReturnError(errors.New("lost reply"))
						} else {
							m.ExpectCommit()
						}
					}
				}
			}
			result, e := s.ApplyUndo(context.Background(), fixtureActor, u)
			if mode == "success" {
				if e != nil || result.SourceOperationID != undoSourceOp || result.Batch.Items[0].After.PriceCents != 250 || result.CreatedAt.IsZero() {
					t.Fatal(result, e)
				}
			} else if !errors.Is(e, want) || result.OperationID != "" {
				t.Fatal(result, e)
			}
		})
	}
}
func TestUndoReplaySurvivesLaterEditsButRejectsChangedReason(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			s, m := mockStore(t)
			u, _, p := undoFixture()
			batch := BatchReceipt{OperationID: testOp, ActorID: fixtureActor.User, PreviewHash: p.PreviewHash, Items: []Receipt{{OperationID: batchChild(testOp, 1), ActorID: fixtureActor.User, ProductID: 1, Action: "price", Before: p.Items[0].Before, After: p.Items[0].After}}}
			r := UndoReceipt{OperationID: testOp, SourceOperationID: undoSourceOp, ActorID: fixtureActor.User, Reason: u.Reason, PreviewHash: u.PreviewHash, Batch: batch}
			raw, _ := json.Marshal(r)
			payload, _ := json.Marshal(u)
			m.ExpectBegin()
			undoLive(m)
			undoLock(m, "undo:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_undos`).WillReturnRows(sqlmock.NewRows([]string{"op", "source", "actor", "reason", "snapshot", "time", "hash"}).AddRow(testOp, undoSourceOp, fixtureActor.User, u.Reason, raw, time.Now(), creationHash(payload)))
			if changed {
				u.Reason = "Outro motivo"
				m.ExpectRollback()
			} else {
				m.ExpectCommit()
			}
			result, e := s.ApplyUndo(context.Background(), fixtureActor, u)
			if changed {
				if e != ErrConflict {
					t.Fatal(e)
				}
			} else if e != nil || result.OperationID != testOp {
				t.Fatal(result, e)
			}
		})
	}
}
func TestUndoDecoderPermissionsLookupAndDuplicateCompensation(t *testing.T) {
	u, source, _ := undoFixture()
	u.PreviewHash = ""
	raw, _ := json.Marshal(u)
	valid, e := DecodeUndo("application/json", raw, false)
	if e != nil || valid.Reason != u.Reason {
		t.Fatal(e)
	}
	for _, body := range []string{string(raw) + `{}`, strings.Replace(string(raw), `"reason":`, `"reason":"x","reason":`, 1), strings.Replace(string(raw), `"reason":"Corrige reajuste"`, `"reason":null`, 1), strings.Replace(string(raw), `"reason":`, `"items":[],"reason":`, 1), strings.Replace(string(raw), undoSourceOp, testOp, 1)} {
		if _, e := DecodeUndo("application/json", []byte(body), false); e != ErrInput {
			t.Fatal("ambiguous undo accepted")
		}
	}
	if _, e := DecodeUndo("text/plain", raw, false); e != ErrInput {
		t.Fatal(e)
	}
	if _, e := DecodeUndo("application/json", raw, true); e != ErrInput {
		t.Fatal("apply without preview")
	}
	s, m := mockStore(t)
	stock := fixtureActor
	stock.Role = "stock"
	if _, e := s.PreviewUndo(context.Background(), stock, u); e != ErrDenied {
		t.Fatal(e)
	}
	m.ExpectBegin()
	undoLive(m)
	undoSourceRow(m, source, true)
	m.ExpectRollback()
	if _, e := s.PreviewUndo(context.Background(), fixtureActor, u); e != ErrConflict {
		t.Fatal("second undo or chain accepted", e)
	}
	m.ExpectQuery(`SELECT operation_id::text.*AND actor_id=\$3`).WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnError(sql.ErrNoRows)
	if _, e := s.Undo(context.Background(), fixtureActor, testOp); e != ErrMissing {
		t.Fatal(e)
	}
}
