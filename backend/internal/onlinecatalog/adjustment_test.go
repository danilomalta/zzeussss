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

func adjustmentFixture() (AdjustmentInput, BatchPreview) {
	before := batchProduct(1)
	after := before
	after.Version = 2
	after.PriceCents = 375
	p := BatchPreview{OperationID: testOp, Items: []BatchPreviewItem{{Action: "price", Before: before, After: after}}}
	raw, _ := json.Marshal(struct {
		Tenant, Actor, Operation string
		Items                    []BatchPreviewItem
	}{fixtureActor.Tenant, fixtureActor.User, testOp, p.Items})
	p.PreviewHash = creationHash(raw)
	v := AdjustmentInput{OperationID: testOp, Reason: "Reajuste conferido", RateBasisPoints: 5000, Items: []AdjustmentItem{{ProductID: 1, ExpectedVersion: 1}}}
	v.PreviewHash = adjustmentPreview(v, p).PreviewHash
	return v, p
}
func TestAdjustmentUsesExactHalfUpAndRejectsOverflowNoRateAndUnsafeBounds(t *testing.T) {
	for _, tc := range []struct{ price, rate, want int64 }{{101, 5000, 152}, {101, -5000, 51}, {1, 5000, 2}, {1, -5000, 1}, {100, -10000, 0}, {100, 10000, 200}, {MaxPrice, -1, 999899999999}} {
		got, e := adjustedPrice(tc.price, tc.rate)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	for _, tc := range []struct{ price, rate int64 }{{MaxPrice, 1}, {MaxPrice + 1, -1}, {-1, 1}, {1, 10001}, {1, -10001}, {1, 0}} {
		if _, e := adjustedPrice(tc.price, tc.rate); e != ErrInput {
			t.Fatal("invalid arithmetic accepted")
		}
	}
}
func TestAdjustmentDecoderRequiresBoundedUniqueVersionedSelection(t *testing.T) {
	v, _ := adjustmentFixture()
	v.PreviewHash = ""
	raw, _ := json.Marshal(v)
	if _, e := DecodeAdjustment("application/json", raw, false); e != nil {
		t.Fatal(e)
	}
	for _, body := range []string{string(raw) + `{}`, strings.Replace(string(raw), `"reason":`, `"reason":"x","reason":`, 1), strings.Replace(string(raw), `"rate_basis_points":5000`, `"rate_basis_points":5000.0`, 1), strings.Replace(string(raw), `"rate_basis_points":5000`, `"rate_basis_points":null`, 1), strings.Replace(string(raw), `"product_id":1`, `"product_id":1,"price_cents":50`, 1), strings.Replace(string(raw), `"expected_version":1`, `"expected_version":1,"expected_version":2`, 1), strings.Replace(string(raw), `"reason":`, `"tenant_id":"foreign","reason":`, 1)} {
		if _, e := DecodeAdjustment("application/json", []byte(body), false); e != ErrInput {
			t.Fatal("unsafe body accepted")
		}
	}
	if _, e := DecodeAdjustment("text/plain", raw, false); e != ErrInput {
		t.Fatal(e)
	}
	if _, e := DecodeAdjustment("application/json", raw, true); e != ErrInput {
		t.Fatal("missing preview accepted")
	}
	v.Items = append(v.Items, v.Items[0])
	if _, e := normalizeAdjustment(v, false); e != ErrInput {
		t.Fatal("duplicate selection accepted")
	}
	v.Items = nil
	for i := 0; i < 101; i++ {
		v.Items = append(v.Items, AdjustmentItem{ProductID: int64(i + 1), ExpectedVersion: 1})
	}
	if _, e := normalizeAdjustment(v, false); e != ErrInput {
		t.Fatal("oversized selection accepted")
	}
}
func TestAdjustmentPreviewIsReadOnlyAndBindsSelectionRateAndReason(t *testing.T) {
	s, m := mockStore(t)
	v, p := adjustmentFixture()
	v.PreviewHash = ""
	m.ExpectBegin()
	undoLive(m)
	batchRow(m, p.Items[0].Before)
	batchRow(m, p.Items[0].Before)
	m.ExpectCommit()
	r, e := s.PreviewAdjustment(context.Background(), fixtureActor, v)
	if e != nil || r.PreviewHash != adjustmentPreview(v, p).PreviewHash || r.Rounding != "half_up" || r.Items[0].After.PriceCents != 375 {
		t.Fatal(r, e)
	}
	original := r.PreviewHash
	v.Reason = "Other"
	if adjustmentPreview(v, p).PreviewHash == original {
		t.Fatal("reason not bound")
	}
	v.Reason = r.Reason
	v.RateBasisPoints = 4999
	if adjustmentPreview(v, p).PreviewHash == original {
		t.Fatal("rate not bound")
	}
}
func TestAdjustmentAtomicMetadataOutboxAndCommitFailures(t *testing.T) {
	for _, mode := range []string{"success", "metadata", "outbox", "suppressed", "commit", "hash"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, p := adjustmentFixture()
			before := p.Items[0].Before
			after := p.Items[0].After
			child := batchChild(testOp, 1)
			m.ExpectBegin()
			undoLive(m)
			undoLock(m, "adjustment:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_adjustments`).WillReturnError(sql.ErrNoRows)
			undoLock(m, "batch:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_batches`).WillReturnError(sql.ErrNoRows)
			undoLock(m, child)
			batchRow(m, before)
			batchRow(m, before)
			if mode == "hash" {
				v.PreviewHash = strings.Repeat("a", 64)
				m.ExpectRollback()
			} else {
				undoLive(m)
				undoLock(m, "batch:"+testOp)
				m.ExpectQuery(`SELECT operation_id::text.*online_catalog_batches`).WillReturnError(sql.ErrNoRows)
				undoLock(m, child)
				batchRow(m, before)
				undoLive(m)
				undoLock(m, child)
				m.ExpectQuery(`SELECT operation_id::text.*online_catalog_operations`).WillReturnError(sql.ErrNoRows)
				batchRow(m, before)
				m.ExpectExec(`UPDATE products SET preco`).WithArgs(fixtureActor.Tenant, int64(1), "3.75", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
				batchRow(m, after)
				m.ExpectQuery(`INSERT INTO online_catalog_operations`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
				m.ExpectExec(`INSERT INTO online_catalog_outbox`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery(`INSERT INTO online_catalog_batches`).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
				q := m.ExpectQuery(`INSERT INTO online_catalog_adjustments`)
				if mode == "metadata" {
					q.WillReturnError(errors.New("private"))
					m.ExpectRollback()
				} else {
					q.WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
					event := m.ExpectExec(`INSERT INTO online_catalog_adjustment_outbox`)
					if mode == "outbox" {
						event.WillReturnError(errors.New("private"))
						m.ExpectRollback()
					} else if mode == "suppressed" {
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
			result, e := s.ApplyAdjustment(context.Background(), fixtureActor, v)
			if mode == "success" {
				if e != nil || result.Batch.Items[0].After.PriceCents != 375 || result.CreatedAt.IsZero() {
					t.Fatal(result, e)
				}
			} else {
				want := ErrUnavailable
				if mode == "hash" {
					want = ErrConflict
				}
				if e != want || result.OperationID != "" {
					t.Fatal(result, e)
				}
			}
		})
	}
}

func TestAdjustmentReplayIsStableAndRejectsChangedReasonOrActor(t *testing.T) {
	for _, mode := range []string{"replay", "changed", "actor"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, p := adjustmentFixture()
			r := AdjustmentReceipt{OperationID: testOp, ActorID: fixtureActor.User, Reason: v.Reason, RateBasisPoints: v.RateBasisPoints, Rounding: "half_up", PreviewHash: v.PreviewHash, Batch: BatchReceipt{OperationID: testOp, ActorID: fixtureActor.User, PreviewHash: p.PreviewHash, Items: []Receipt{{OperationID: batchChild(testOp, 1), ActorID: fixtureActor.User, ProductID: 1, Action: "price", Before: p.Items[0].Before, After: p.Items[0].After}}}}
			raw, _ := json.Marshal(r)
			payload, _ := json.Marshal(v)
			m.ExpectBegin()
			undoLive(m)
			undoLock(m, "adjustment:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_adjustments`).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "reason", "rate", "raw", "at", "hash"}).AddRow(testOp, fixtureActor.User, v.Reason, v.RateBasisPoints, raw, time.Now(), creationHash(payload)))
			a := fixtureActor
			if mode == "changed" {
				v.Reason = "Other"
			}
			if mode == "actor" {
				a.User = undoSourceOp
			}
			if mode == "replay" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.ApplyAdjustment(context.Background(), a, v)
			if mode == "replay" {
				if e != nil || got.Batch.Items[0].After.PriceCents != 375 {
					t.Fatal(got, e)
				}
			} else if e != ErrConflict {
				t.Fatal(e)
			}
		})
	}
	s, m := mockStore(t)
	m.ExpectQuery(`SELECT operation_id::text.*AND actor_id=\$3`).WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnError(sql.ErrNoRows)
	if _, e := s.Adjustment(context.Background(), fixtureActor, testOp); e != ErrMissing {
		t.Fatal(e)
	}
	m.ExpectQuery(`SELECT operation_id::text.*ORDER BY created_at`).WithArgs(fixtureActor.Tenant, 1, 0).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "reason", "rate", "snapshot", "at", "hash"}))
	if rows, e := s.AdjustmentHistory(context.Background(), fixtureActor, 1, 0); e != nil || rows == nil || len(rows) != 0 {
		t.Fatal(rows, e)
	}
	stock := fixtureActor
	stock.Role = "stock"
	v, _ := adjustmentFixture()
	if _, e := s.ApplyAdjustment(context.Background(), stock, v); e != ErrDenied {
		t.Fatal(e)
	}
}
func TestAdjustmentStaleNoOpMissingAndSuppressedSessionRejectBeforeMutation(t *testing.T) {
	for _, mode := range []string{"stale", "no-op", "missing", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, p := adjustmentFixture()
			v.PreviewHash = ""
			m.ExpectBegin()
			if mode == "revoked" {
				m.ExpectQuery(`SELECT s.id::text`).WillReturnError(sql.ErrNoRows)
			} else {
				undoLive(m)
				if mode == "missing" {
					m.ExpectQuery(`SELECT id,nome`).WillReturnError(sql.ErrNoRows)
				} else {
					before := p.Items[0].Before
					if mode == "stale" {
						before.Version = 2
					}
					if mode == "no-op" {
						before.PriceCents = 0
					}
					batchRow(m, before)
					if mode == "no-op" {
						batchRow(m, before)
					}
				}
			}
			m.ExpectRollback()
			_, e := s.PreviewAdjustment(context.Background(), fixtureActor, v)
			want := ErrConflict
			if mode == "no-op" {
				want = ErrInput
			}
			if mode == "missing" {
				want = ErrMissing
			}
			if mode == "revoked" {
				want = ErrDenied
			}
			if e != want {
				t.Fatal(e)
			}
		})
	}
}
