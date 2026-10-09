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

func barcodeBatchFixture() (BarcodeBatchInput, BarcodeBatchPreview, BarcodeBatchReceipt) {
	v, _, single := barcodeFixture()
	b := BarcodeBatchInput{OperationID: testOp, Reason: v.Reason, Items: []BarcodeBatchItem{{ProductID: 1, ExpectedVersion: 1, Code: v.Code}}}
	p := BarcodeBatchPreview{OperationID: b.OperationID, Reason: b.Reason, Items: []BarcodeBatchPreviewItem{{Code: v.Code, CanonicalCode: single.After.CanonicalCode, Before: single.ProductBefore, After: single.ProductAfter}}}
	raw, _ := json.Marshal(struct {
		Tenant, Actor string
		Preview       BarcodeBatchPreview
	}{fixtureActor.Tenant, fixtureActor.User, p})
	p.PreviewHash = creationHash(raw)
	b.PreviewHash = p.PreviewHash
	child := barcodeBatchChild(b.OperationID, 1)
	single.OperationID = child
	single.After.ID = child
	return b, p, BarcodeBatchReceipt{OperationID: b.OperationID, ActorID: fixtureActor.User, Reason: b.Reason, PreviewHash: b.PreviewHash, Items: []BarcodeReceipt{single}, CreatedAt: single.CreatedAt}
}
func barcodeBatchRows(r BarcodeBatchReceipt, hash string) *sqlmock.Rows {
	raw, _ := json.Marshal(r)
	return sqlmock.NewRows([]string{"op", "actor", "raw", "at", "hash"}).AddRow(r.OperationID, r.ActorID, raw, r.CreatedAt, hash)
}
func expectBarcodeBatchPreview(m sqlmock.Sqlmock, p BarcodeBatchPreview, count int, conflict bool) {
	for _, v := range p.Items {
		batchRow(m, v.Before)
		m.ExpectQuery(`SELECT count`).WithArgs(fixtureActor.Tenant, v.Before.ID).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(count))
		if count < MaxProductBarcodes {
			q := m.ExpectQuery(`SELECT id::text.*canonical_code=\$2`).WithArgs(fixtureActor.Tenant, v.CanonicalCode)
			if conflict {
				_, b, _ := barcodeFixture()
				q.WillReturnRows(barcodeRows(b))
			} else {
				q.WillReturnError(sql.ErrNoRows)
			}
		}
	}
}
func TestBarcodeBatchInputRejectsAmbiguityDuplicateCanonicalAndProduct(t *testing.T) {
	b, _, _ := barcodeBatchFixture()
	raw, _ := json.Marshal(b)
	if _, e := DecodeBarcodeBatch("application/json", raw, true); e != nil {
		t.Fatal(e)
	}
	b.PreviewHash = ""
	raw, _ = json.Marshal(b)
	if _, e := DecodeBarcodeBatch("application/json", raw, false); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{string(raw) + `{}`, strings.Replace(string(raw), `"code":"4006381333931"`, `"code":4006381333931`, 1), strings.Replace(string(raw), `"code":`, `"code":null,"code":`, 1), strings.Replace(string(raw), `"expected_version":1`, `"expected_version":1.0`, 1), strings.Replace(string(raw), `"reason":`, `"tenant_id":"x","reason":`, 1), strings.Replace(string(raw), `"product_id":1`, `"product_id":1,"product_id":1`, 1), strings.Replace(string(raw), `"items":`, `"preview_hash":"x","items":`, 1)} {
		if _, e := DecodeBarcodeBatch("application/json", []byte(bad), false); e != ErrInput {
			t.Fatal("bad body accepted", e)
		}
	}
	for _, mode := range []string{"empty", "too-many", "product", "equivalent", "digit", "version", "reason", "hash"} {
		x := b
		x.Items = append([]BarcodeBatchItem(nil), b.Items...)
		switch mode {
		case "empty":
			x.Items = nil
		case "too-many":
			for len(x.Items) <= 50 {
				x.Items = append(x.Items, x.Items[0])
			}
		case "product":
			x.Items = append(x.Items, x.Items[0])
		case "equivalent":
			x.Items = append(x.Items, BarcodeBatchItem{ProductID: 2, ExpectedVersion: 1, Code: "04006381333931"})
		case "digit":
			x.Items[0].Code = "4006381333932"
		case "version":
			x.Items[0].ExpectedVersion = MaxVersion
		case "reason":
			x.Reason = " "
		case "hash":
			x.PreviewHash = "a"
		}
		if _, e := normalizeBarcodeBatch(x, false); e != ErrInput {
			t.Fatal(mode, e)
		}
	}
	if _, e := DecodeBarcodeBatch("text/plain", raw, false); e != ErrInput {
		t.Fatal(e)
	}
	if _, e := DecodeBarcodeBatch("application/json", make([]byte, 32769), false); e != ErrInput {
		t.Fatal(e)
	}
	// Normalization must not mutate the caller's slice or invent child identities.
	b.Items = append([]BarcodeBatchItem{{ProductID: 2, ExpectedVersion: 1, Code: "96385074"}}, b.Items...)
	normalized, e := normalizeBarcodeBatch(b, false)
	if e != nil || normalized.Items[0].ProductID != 1 || b.Items[0].ProductID != 2 || barcodeBatchChild(testOp, 1) == batchChild(testOp, 1) {
		t.Fatal("canonical input/namespace", e)
	}
}
func TestBarcodeBatchPreviewOnlyReadsAndRejectsCapacityConflictRevocation(t *testing.T) {
	for _, mode := range []string{"ok", "capacity", "duplicate", "stale", "missing", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			b, p, _ := barcodeBatchFixture()
			b.PreviewHash = ""
			m.ExpectBegin()
			if mode == "revoked" {
				m.ExpectQuery(`SELECT`).WillReturnError(sql.ErrNoRows)
			} else {
				undoLive(m)
				if mode == "missing" {
					m.ExpectQuery(`SELECT id`).WillReturnError(sql.ErrNoRows)
				} else if mode == "stale" {
					wrong := p.Items[0].Before
					wrong.Version = 2
					batchRow(m, wrong)
				} else {
					count := 0
					if mode == "capacity" {
						count = 50
					}
					expectBarcodeBatchPreview(m, p, count, mode == "duplicate")
				}
			}
			if mode == "ok" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.PreviewBarcodeBatch(context.Background(), fixtureActor, b)
			if mode == "ok" {
				if e != nil || got.PreviewHash != p.PreviewHash {
					t.Fatal(got, e)
				}
			} else if e == nil {
				t.Fatal("failure accepted")
			}
		})
	}
}
func expectBarcodeBatchStart(m sqlmock.Sqlmock, b BarcodeBatchInput, r BarcodeBatchReceipt) {
	m.ExpectBegin()
	undoLive(m)
	undoLock(m, "barcode-batch:"+b.OperationID)
	m.ExpectQuery(`SELECT operation_id::text.*online_catalog_barcode_batches`).WithArgs(fixtureActor.Tenant, b.OperationID).WillReturnError(sql.ErrNoRows)
	for _, item := range r.Items {
		undoLock(m, "barcode-op:"+item.OperationID)
		m.ExpectQuery(`SELECT operation_id::text.*online_catalog_barcode_operations`).WillReturnError(sql.ErrNoRows)
	}
	for _, item := range r.Items {
		undoLock(m, "barcode-key:"+item.After.CanonicalCode)
	}
}
func TestBarcodeBatchApplyLateFailureRollsBackItemsReceiptAndBothOutboxes(t *testing.T) {
	for _, mode := range []string{"ok", "hash", "item-outbox", "receipt", "snapshot-trigger", "batch-outbox", "suppressed", "commit"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			b, p, r := barcodeBatchFixture()
			expectBarcodeBatchStart(m, b, r)
			expectBarcodeBatchPreview(m, p, 0, false)
			if mode == "hash" {
				b.PreviewHash = strings.Repeat("a", 64)
			} else {
				single := r.Items[0]
				undoLock(m, "barcode-op:"+single.OperationID)
				m.ExpectQuery(`SELECT operation_id::text.*online_catalog_barcode_operations`).WillReturnError(sql.ErrNoRows)
				undoLock(m, "barcode-key:"+single.After.CanonicalCode)
				batchRow(m, single.ProductBefore)
				m.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
				m.ExpectQuery(`SELECT id::text.*canonical_code=\$2`).WillReturnError(sql.ErrNoRows)
				m.ExpectQuery(`INSERT INTO online_catalog_barcodes`).WillReturnRows(barcodeRows(single.After))
				m.ExpectExec(`UPDATE products`).WillReturnResult(sqlmock.NewResult(0, 1))
				batchRow(m, single.ProductAfter)
				m.ExpectQuery(`INSERT INTO online_catalog_barcode_operations`).WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(single.CreatedAt))
				out := m.ExpectExec(`INSERT INTO online_catalog_barcode_outbox`)
				if mode == "item-outbox" {
					out.WillReturnError(errors.New("private"))
				} else {
					out.WillReturnResult(sqlmock.NewResult(0, 1))
					rec := m.ExpectQuery(`INSERT INTO online_catalog_barcode_batches`)
					if mode == "receipt" {
						rec.WillReturnError(errors.New("private"))
					} else {
						rec.WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.CreatedAt))
						persisted := r
						if mode == "snapshot-trigger" {
							persisted.Reason = "changed"
							persisted.Items = append([]BarcodeReceipt(nil), r.Items...)
							persisted.Items[0].Reason = persisted.Reason
						}
						raw, _ := json.Marshal(b)
						m.ExpectQuery(`SELECT operation_id::text.*online_catalog_barcode_batches`).WillReturnRows(barcodeBatchRows(persisted, creationHash(raw)))
						if mode != "snapshot-trigger" {
							out := m.ExpectExec(`INSERT INTO online_catalog_barcode_batch_outbox`)
							if mode == "batch-outbox" {
								out.WillReturnError(errors.New("private"))
							} else {
								n := int64(1)
								if mode == "suppressed" {
									n = 0
								}
								out.WillReturnResult(sqlmock.NewResult(0, n))
							}
						}
					}
				}
			}
			if mode == "ok" {
				m.ExpectCommit()
			} else if mode == "commit" {
				m.ExpectCommit().WillReturnError(errors.New("private"))
			} else {
				m.ExpectRollback()
			}
			got, e := s.ApplyBarcodeBatch(context.Background(), fixtureActor, b)
			if mode == "ok" {
				if e != nil || len(got.Items) != 1 || got.Items[0].OperationID != r.Items[0].OperationID {
					t.Fatal(got, e)
				}
			} else if e == nil || got.OperationID != "" {
				t.Fatal("partial receipt published", e)
			}
		})
	}
}
func TestBarcodeBatchReplayPrecedesCurrentProductAndRejectsDifferentPayloadActor(t *testing.T) {
	for _, mode := range []string{"ok", "payload", "actor"} {
		s, m := mockStore(t)
		b, _, r := barcodeBatchFixture()
		raw, _ := json.Marshal(b)
		hash := creationHash(raw)
		if mode == "payload" {
			b.Reason = "changed"
		}
		if mode == "actor" {
			r.ActorID = "22222222-2222-4222-8222-222222222222"
			r.Items[0].ActorID = r.ActorID
		}
		m.ExpectBegin()
		undoLive(m)
		undoLock(m, "barcode-batch:"+b.OperationID)
		m.ExpectQuery(`SELECT operation_id::text.*online_catalog_barcode_batches`).WillReturnRows(barcodeBatchRows(r, hash))
		if mode == "ok" {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		_, e := s.ApplyBarcodeBatch(context.Background(), fixtureActor, b)
		if mode == "ok" && e != nil || mode != "ok" && e != ErrConflict {
			t.Fatal(mode, e)
		}
	}
}
func TestBarcodeBatchReceiptLookupIsLiveScopedAndMalformedSnapshotsFailClosed(t *testing.T) {
	b, _, r := barcodeBatchFixture()
	raw, _ := json.Marshal(b)
	hash := creationHash(raw)
	s, m := mockStore(t)
	m.ExpectBegin()
	undoLive(m)
	m.ExpectQuery(`SELECT operation_id::text.*AND actor_id=\$3`).WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnRows(barcodeBatchRows(r, hash))
	m.ExpectCommit()
	if _, e := s.BarcodeBatch(context.Background(), fixtureActor, testOp); e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"empty", "child", "actor", "price", "reason", "code", "time", "hash"} {
		_, _, bad := barcodeBatchFixture()
		switch mode {
		case "empty":
			bad.Items = nil
		case "child":
			bad.Items[0].After.ID = testOp
		case "actor":
			bad.Items[0].ActorID = "22222222-2222-4222-8222-222222222222"
		case "price":
			bad.Items[0].ProductAfter.PriceCents++
		case "reason":
			bad.Items[0].Reason = "other"
		case "code":
			bad.Items[0].After.Code = "bad"
		case "time":
			bad.Items[0].CreatedAt = time.Time{}
		case "hash":
			bad.PreviewHash = "bad"
		}
		s, m := mockStore(t)
		m.ExpectQuery(`snapshot`).WillReturnRows(barcodeBatchRows(bad, hash))
		if _, _, e := scanBarcodeBatch(s.DB.QueryRow(`SELECT snapshot`)); e != ErrUnavailable {
			t.Fatal(mode, e)
		}
	}
	a := fixtureActor
	a.Role = "cashier"
	if _, e := s.ApplyBarcodeBatch(context.Background(), a, b); e != ErrDenied {
		t.Fatal(e)
	}
}

func TestBarcodeBatchRejectsPreviouslyPublishedChildBeforeProductLocks(t *testing.T) {
	s, m := mockStore(t)
	b, _, r := barcodeBatchFixture()
	m.ExpectBegin()
	undoLive(m)
	undoLock(m, "barcode-batch:"+b.OperationID)
	m.ExpectQuery(`SELECT operation_id::text.*online_catalog_barcode_batches`).WillReturnError(sql.ErrNoRows)
	undoLock(m, "barcode-op:"+r.Items[0].OperationID)
	m.ExpectQuery(`SELECT operation_id::text.*online_catalog_barcode_operations`).WillReturnRows(barcodeReceiptRows(r.Items[0], strings.Repeat("a", 64)))
	m.ExpectRollback()
	if _, e := s.ApplyBarcodeBatch(context.Background(), fixtureActor, b); e != ErrConflict {
		t.Fatal(e)
	}
}
