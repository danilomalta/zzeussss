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

func barcodeFixture() (BarcodeInput, Barcode, BarcodeReceipt) {
	input := BarcodeInput{OperationID: testOp, Code: "4006381333931", ExpectedVersion: 1, Reason: "Código conferido"}
	key, _ := CanonicalBarcode(input.Code)
	at := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	b := Barcode{ID: testOp, ProductID: 1, Code: input.Code, CanonicalCode: key, Active: true, Version: 1, CreatedAt: at, UpdatedAt: at}
	p := batchProduct(1)
	after := p
	after.Version++
	return input, b, BarcodeReceipt{OperationID: testOp, ActorID: fixtureActor.User, Action: "add", Reason: input.Reason, After: b, ProductBefore: p, ProductAfter: after, CreatedAt: at}
}
func barcodeRows(items ...Barcode) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "product", "code", "key", "active", "version", "created", "updated"})
	for _, b := range items {
		rows.AddRow(b.ID, b.ProductID, b.Code, b.CanonicalCode, b.Active, b.Version, b.CreatedAt, b.UpdatedAt)
	}
	return rows
}
func barcodeReceiptRows(r BarcodeReceipt, hash string) *sqlmock.Rows {
	raw, _ := json.Marshal(r)
	return sqlmock.NewRows([]string{"op", "actor", "product", "barcode", "action", "reason", "raw", "at", "hash"}).AddRow(r.OperationID, r.ActorID, r.After.ProductID, r.After.ID, r.Action, r.Reason, raw, r.CreatedAt, hash)
}
func TestBarcodeCheckDigitCanonicalEquivalenceAndNoCoercion(t *testing.T) {
	for _, code := range []string{"96385074", "036000291452", "4006381333931", "00000096385074", "0036000291452", "00036000291452"} {
		key, e := CanonicalBarcode(code)
		if e != nil || len(key) != 14 || !strings.HasSuffix(key, code) {
			t.Fatal(code, key, e)
		}
	}
	a, _ := CanonicalBarcode("036000291452")
	b, _ := CanonicalBarcode("0036000291452")
	if a != b {
		t.Fatal("equivalent forms differ")
	}
	for _, code := range []string{"", "00000000", "4006381333932", "1234", " 4006381333931", "4006381333931\n", "400638133393a", "４００６３８１３３３９３１", "4.006381333931e12", "+4006381333931"} {
		if _, e := CanonicalBarcode(code); e != ErrInput {
			t.Fatal(code, e)
		}
	}
	v, _, _ := barcodeFixture()
	raw, _ := json.Marshal(v)
	if _, e := DecodeBarcode("application/json", raw, "add"); e != nil {
		t.Fatal(e)
	}
	for _, body := range []string{string(raw) + `{}`, strings.Replace(string(raw), `"code":"4006381333931"`, `"code":4006381333931`, 1), strings.Replace(string(raw), `"code":`, `"code":null,"code":`, 1), strings.Replace(string(raw), `"expected_version":1`, `"expected_version":1.0`, 1), strings.Replace(string(raw), `"reason":`, `"tenant_id":"x","reason":`, 1), strings.Replace(string(raw), `"reason":`, `"reason":"x","reason":`, 1)} {
		if _, e := DecodeBarcode("application/json", []byte(body), "add"); e != ErrInput {
			t.Fatal("ambiguous input accepted", e)
		}
	}
	if _, e := DecodeBarcode("text/plain", raw, "add"); e != ErrInput {
		t.Fatal(e)
	}
	active := false
	state := BarcodeInput{OperationID: testOp, ExpectedVersion: 2, ExpectedCodeVersion: 1, Active: &active, Reason: "Inativado"}
	raw, _ = json.Marshal(state)
	if _, e := DecodeBarcode("application/json", raw, "active"); e != nil {
		t.Fatal(e)
	}
	state.Active = nil
	raw, _ = json.Marshal(state)
	if _, e := DecodeBarcode("application/json", raw, "active"); e != ErrInput {
		t.Fatal("missing boolean accepted")
	}
}
func expectBarcodeStart(m sqlmock.Sqlmock, v BarcodeInput) {
	m.ExpectBegin()
	undoLive(m)
	undoLock(m, "barcode-op:"+v.OperationID)
	m.ExpectQuery(`SELECT operation_id::text.*online_catalog_barcode_operations`).WithArgs(fixtureActor.Tenant, v.OperationID).WillReturnError(sql.ErrNoRows)
}
func TestBarcodeAddTransactionAndLateFailureNeverPublishPartialState(t *testing.T) {
	for _, mode := range []string{"ok", "barcode-trigger", "product-suppressed", "product-trigger", "receipt", "outbox", "outbox-suppressed", "commit"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, b, r := barcodeFixture()
			expectBarcodeStart(m, v)
			undoLock(m, "barcode-key:"+b.CanonicalCode)
			batchRow(m, r.ProductBefore)
			m.ExpectQuery(`SELECT count`).WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			m.ExpectQuery(`SELECT id::text.*canonical_code=\$2`).WithArgs(fixtureActor.Tenant, b.CanonicalCode).WillReturnError(sql.ErrNoRows)
			inserted := b
			if mode == "barcode-trigger" {
				inserted.ProductID = 2
			}
			m.ExpectQuery(`INSERT INTO online_catalog_barcodes`).WithArgs(fixtureActor.Tenant, testOp, int64(1), v.Code, b.CanonicalCode).WillReturnRows(barcodeRows(inserted))
			if mode != "barcode-trigger" {
				affected := int64(1)
				if mode == "product-suppressed" {
					affected = 0
				}
				m.ExpectExec(`UPDATE products SET catalog_version`).WithArgs(fixtureActor.Tenant, int64(1), int64(1)).WillReturnResult(sqlmock.NewResult(0, affected))
				if mode != "product-suppressed" {
					persisted := r.ProductAfter
					if mode == "product-trigger" {
						persisted.PriceCents++
					}
					batchRow(m, persisted)
					if mode != "product-trigger" {
						insert := m.ExpectQuery(`INSERT INTO online_catalog_barcode_operations`)
						if mode == "receipt" {
							insert.WillReturnError(errors.New("private"))
						} else {
							insert.WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.CreatedAt))
							out := m.ExpectExec(`INSERT INTO online_catalog_barcode_outbox`)
							if mode == "outbox" {
								out.WillReturnError(errors.New("private"))
							} else {
								n := int64(1)
								if mode == "outbox-suppressed" {
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
			got, e := s.ChangeBarcode(context.Background(), fixtureActor, 1, "", "add", v)
			if mode == "ok" {
				if e != nil || got.After != b || got.ProductAfter != r.ProductAfter || got.Before != nil {
					t.Fatal(got, e)
				}
			} else if e != ErrUnavailable || got.OperationID != "" {
				t.Fatal("partial receipt returned", got, e)
			}
		})
	}
}
func TestBarcodeStaleDuplicateCapacityAndMissingProductRejectBeforeWrites(t *testing.T) {
	for _, mode := range []string{"stale", "duplicate", "capacity", "missing", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, b, r := barcodeFixture()
			m.ExpectBegin()
			if mode == "revoked" {
				m.ExpectQuery(`SELECT s.id::text`).WillReturnError(sql.ErrNoRows)
			} else {
				undoLive(m)
				undoLock(m, "barcode-op:"+v.OperationID)
				m.ExpectQuery(`SELECT operation_id::text`).WillReturnError(sql.ErrNoRows)
				undoLock(m, "barcode-key:"+b.CanonicalCode)
				if mode == "missing" {
					m.ExpectQuery(`SELECT id,nome`).WillReturnError(sql.ErrNoRows)
				} else {
					p := r.ProductBefore
					if mode == "stale" {
						p.Version++
					}
					batchRow(m, p)
					if mode != "stale" {
						count := 0
						if mode == "capacity" {
							count = 50
						}
						m.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
						if mode == "duplicate" {
							b.Active = false
							m.ExpectQuery(`SELECT id::text`).WillReturnRows(barcodeRows(b))
						}
					}
				}
			}
			m.ExpectRollback()
			_, e := s.ChangeBarcode(context.Background(), fixtureActor, 1, "", "add", v)
			want := ErrConflict
			if mode == "missing" {
				want = ErrMissing
			}
			if mode == "revoked" {
				want = ErrDenied
			}
			if e != want {
				t.Fatal(mode, e)
			}
		})
	}
}
func TestBarcodeReplayUsesOriginalSnapshotsAndRejectsDifferentActorOrPayload(t *testing.T) {
	for _, mode := range []string{"same", "reason", "actor"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, _, r := barcodeFixture()
			payload, _ := json.Marshal(struct {
				Product         int64
				Barcode, Action string
				Input           BarcodeInput
			}{1, "", "add", v})
			hash := creationHash(payload)
			if mode == "actor" {
				r.ActorID = undoSourceOp
			}
			m.ExpectBegin()
			undoLive(m)
			undoLock(m, "barcode-op:"+v.OperationID)
			m.ExpectQuery(`SELECT operation_id::text`).WillReturnRows(barcodeReceiptRows(r, hash))
			if mode == "reason" {
				v.Reason = "Outro"
			}
			if mode == "same" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.ChangeBarcode(context.Background(), fixtureActor, 1, "", "add", v)
			if mode == "same" {
				if e != nil || got.ProductAfter.Version != 2 {
					t.Fatal(got, e)
				}
			} else if e != ErrConflict {
				t.Fatal(e)
			}
		})
	}
}
func TestBarcodeStateVersionedNoOpAndWrongProductFailClosed(t *testing.T) {
	for _, mode := range []string{"ok", "stale-code", "noop", "missing", "trigger"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			_, b, r := barcodeFixture()
			active := false
			v := BarcodeInput{OperationID: undoSourceOp, ExpectedVersion: 2, ExpectedCodeVersion: 1, Active: &active, Reason: "Código antigo"}
			expectBarcodeStart(m, v)
			batchRow(m, r.ProductAfter)
			query := m.ExpectQuery(`SELECT id::text.*FOR UPDATE`).WithArgs(fixtureActor.Tenant, int64(1), b.ID)
			if mode == "missing" {
				query.WillReturnError(sql.ErrNoRows)
			} else {
				if mode == "stale-code" {
					b.Version = 2
				}
				if mode == "noop" {
					b.Active = false
				}
				query.WillReturnRows(barcodeRows(b))
			}
			if mode == "ok" || mode == "trigger" {
				after := b
				after.Active = false
				after.Version++
				after.UpdatedAt = after.UpdatedAt.Add(time.Second)
				if mode == "trigger" {
					after.Active = true
				}
				m.ExpectQuery(`UPDATE online_catalog_barcodes`).WithArgs(fixtureActor.Tenant, int64(1), b.ID, false, int64(1)).WillReturnRows(barcodeRows(after))
				if mode == "ok" {
					m.ExpectExec(`UPDATE products`).WillReturnResult(sqlmock.NewResult(0, 1))
					p := r.ProductAfter
					p.Version++
					batchRow(m, p)
					m.ExpectQuery(`INSERT INTO online_catalog_barcode_operations`).WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(time.Now()))
					m.ExpectExec(`INSERT INTO online_catalog_barcode_outbox`).WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectCommit()
				} else {
					m.ExpectRollback()
				}
			} else {
				m.ExpectRollback()
			}
			got, e := s.ChangeBarcode(context.Background(), fixtureActor, 1, b.ID, "active", v)
			if mode == "ok" {
				if e != nil || got.Before == nil || got.After.Active || got.After.Version != 2 || got.ProductAfter.Version != 3 {
					t.Fatal(got, e)
				}
			} else {
				want := ErrConflict
				if mode == "missing" {
					want = ErrMissing
				}
				if mode == "trigger" {
					want = ErrUnavailable
				}
				if e != want {
					t.Fatal(e)
				}
			}
		})
	}
}
