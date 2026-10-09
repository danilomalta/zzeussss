package onlinecatalog

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
)

func TestBarcodeLookupRequiresActiveScopedCodeAndProduct(t *testing.T) {
	for _, mode := range []string{"ok", "missing", "inactive-code", "inactive-product", "wrong-key", "commit", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, b, r := barcodeFixture()
			m.ExpectBegin()
			if mode == "revoked" {
				m.ExpectQuery(`SELECT s.id::text`).WillReturnError(sql.ErrNoRows)
			} else {
				undoLive(m)
				query := m.ExpectQuery(`SELECT id::text.*canonical_code=\$2 AND ativo=TRUE`).WithArgs(fixtureActor.Tenant, b.CanonicalCode)
				if mode == "missing" {
					query.WillReturnError(sql.ErrNoRows)
				} else {
					if mode == "inactive-code" {
						b.Active = false
					}
					if mode == "wrong-key" {
						b.Code = "96385074"
						b.CanonicalCode, _ = CanonicalBarcode(b.Code)
					}
					query.WillReturnRows(barcodeRows(b))
					if mode != "inactive-code" && mode != "wrong-key" {
						p := r.ProductAfter
						if mode == "inactive-product" {
							p.Active = false
						}
						batchRow(m, p)
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
			got, e := s.LookupBarcode(context.Background(), fixtureActor, v.Code)
			if mode == "ok" {
				if e != nil || got.Product != r.ProductAfter || got.Barcode != b {
					t.Fatal(got, e)
				}
			} else {
				want := ErrUnavailable
				if mode == "missing" || mode == "inactive-product" {
					want = ErrMissing
				}
				if mode == "revoked" {
					want = ErrDenied
				}
				if e != want || got.Barcode.ID != "" {
					t.Fatal(got, e)
				}
			}
		})
	}
}
func TestBarcodeListSnapshotCountsFiltersAndEmptyState(t *testing.T) {
	for _, mode := range []string{"ok", "empty", "count-mismatch", "filter", "foreign", "rows-error", "commit"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			_, b, r := barcodeFixture()
			state := "all"
			total := 1
			items := []Barcode{b}
			if mode == "empty" {
				total = 0
				items = nil
			}
			if mode == "count-mismatch" {
				items = nil
			}
			if mode == "filter" {
				state = "inactive"
			}
			if mode == "foreign" {
				items[0].ProductID = 2
			}
			m.ExpectBegin()
			undoLive(m)
			batchRow(m, r.ProductAfter)
			m.ExpectQuery(`SELECT count`).WithArgs(fixtureActor.Tenant, int64(1), state).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(total))
			rows := barcodeRows(items...)
			if mode == "rows-error" {
				rows.RowError(0, errors.New("private"))
			}
			m.ExpectQuery(`SELECT id::text.*ORDER BY created_at,id`).WithArgs(fixtureActor.Tenant, int64(1), state, 1, 0).WillReturnRows(rows)
			if mode == "ok" || mode == "empty" {
				m.ExpectCommit()
			} else if mode == "commit" {
				m.ExpectCommit().WillReturnError(errors.New("private"))
			} else {
				m.ExpectRollback()
			}
			got, e := s.ListBarcodes(context.Background(), fixtureActor, 1, state, 1, 0)
			if mode == "ok" || mode == "empty" {
				if e != nil || got.Items == nil || got.Total != total || len(got.Items) != total {
					t.Fatal(got, e)
				}
			} else if e != ErrUnavailable || len(got.Items) != 0 {
				t.Fatal(got, e)
			}
		})
	}
}
func TestBarcodeHistoryOriginalReceiptOwnRecoveryAndRoleBoundaries(t *testing.T) {
	s, m := mockStore(t)
	_, _, r := barcodeFixture()
	m.ExpectBegin()
	undoLive(m)
	batchRow(m, r.ProductAfter)
	m.ExpectQuery(`SELECT count`).WithArgs(fixtureActor.Tenant, int64(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	m.ExpectQuery(`SELECT operation_id::text.*ORDER BY created_at DESC`).WithArgs(fixtureActor.Tenant, int64(1), 1, 0).WillReturnRows(barcodeReceiptRows(r, strings.Repeat("a", 64)))
	m.ExpectCommit()
	page, e := s.BarcodeHistory(context.Background(), fixtureActor, 1, 1, 0)
	if e != nil || page.Total != 2 || !page.HasMore || page.Items[0].After.Code != r.After.Code {
		t.Fatal(page, e)
	}
	m.ExpectQuery(`SELECT operation_id::text.*AND actor_id=\$3`).WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnError(sql.ErrNoRows)
	if _, e = s.BarcodeOperation(context.Background(), fixtureActor, testOp); e != ErrMissing {
		t.Fatal(e)
	}
	a := fixtureActor
	a.Role = "cashier"
	v, _, _ := barcodeFixture()
	if _, e = (Store{}).ChangeBarcode(context.Background(), a, 1, "", "add", v); e != ErrDenied {
		t.Fatal(e)
	}
	a.Role = "stock"
	if _, e = (Store{}).BarcodeHistory(context.Background(), a, 1, 10, 0); e != ErrDenied {
		t.Fatal(e)
	}
	a.Role = "employee"
	if _, e = (Store{}).LookupBarcode(context.Background(), a, v.Code); e != ErrDenied {
		t.Fatal(e)
	}
	for _, state := range []string{"", "yes"} {
		if _, e = (Store{}).ListBarcodes(context.Background(), fixtureActor, 1, state, 1, 0); e != ErrInput {
			t.Fatal(e)
		}
	}
	for _, limit := range []int{0, 51} {
		if _, e = (Store{}).ListBarcodes(context.Background(), fixtureActor, 1, "all", limit, 0); e != ErrInput {
			t.Fatal(e)
		}
	}
}
