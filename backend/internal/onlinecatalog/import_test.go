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

const importHeader = "sku;expected_version;preco;ativo\n"

func importFixture() (ImportInput, BatchPreview) {
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
	v := ImportInput{OperationID: testOp, Reason: "Reajuste conferido", CSV: importHeader + "1;1;3,75;\n"}
	v.PreviewHash = importPreview(v, p).PreviewHash
	return v, p
}
func importSKUrow(m sqlmock.Sqlmock, p Product) {
	m.ExpectQuery(`SELECT id,nome.*sku=\$2`).WithArgs(fixtureActor.Tenant, p.SKU).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}).AddRow(p.ID, p.Name, p.Description, p.SKU, fmt.Sprintf("%d.%02d", p.PriceCents/100, p.PriceCents%100), p.Active, p.Version))
}
func TestImportCSVExactMoneyAndBoundedRows(t *testing.T) {
	for _, tc := range []struct {
		price string
		cents int64
	}{{"0", 0}, {"0,01", 1}, {"3.7", 370}, {"9999999999,99", MaxPrice}} {
		rows, e := parseImport(importHeader + "áçúcar;1;" + tc.price + ";\n")
		if e != nil || len(rows) != 1 || *rows[0].Item.PriceCents != tc.cents {
			t.Fatal(tc, e)
		}
	}
	rows, e := parseImport("sku;expected_version;preco;ativo\r\n\"SKU;A\";2;;false\r\n")
	if e != nil || rows[0].SKU != "SKU;A" || *rows[0].Item.Active {
		t.Fatal(rows, e)
	}
	invalid := []string{"", importHeader, "sku,expected_version,preco,ativo\nA,1,2,\n", importHeader + "A;1;1e3;\n", importHeader + "A;1;-2;\n", importHeader + "A;1;+2;\n", importHeader + "A;1;1.234,56;\n", importHeader + "A;1;1.001;\n", importHeader + "A;1;.5;\n", importHeader + "A;1;1.;\n", importHeader + "A;1;10000000000;\n", importHeader + "A;01;2;\n", importHeader + "A;1;2;true\n", importHeader + "A;1;;sim\n", importHeader + "A;1;;\n", importHeader + "A;1;2;\n A ;1;3;\n", importHeader + "A;1;2;;\n", importHeader + "\"A\nB\";1;2;\n", importHeader + string([]byte{255}) + ";1;2;\n", strings.Repeat("a", MaxImportBytes+1)}
	for _, src := range invalid {
		if _, e := parseImport(src); e != ErrInput {
			t.Fatal("unsafe CSV accepted", e)
		}
	}
	var many strings.Builder
	many.WriteString(importHeader)
	for i := 0; i < MaxBatchItems; i++ {
		fmt.Fprintf(&many, "SKU%d;1;2;\n", i)
	}
	if rows, e := parseImport(many.String()); e != nil || len(rows) != 100 {
		t.Fatal(e)
	}
	many.WriteString("extra;1;2;\n")
	if _, e := parseImport(many.String()); e != ErrInput {
		t.Fatal("row limit failed")
	}
}
func TestImportJSONStrictAndHashesBindReasonBytesAndActor(t *testing.T) {
	v, p := importFixture()
	v.PreviewHash = ""
	raw, _ := json.Marshal(v)
	if _, e := DecodeImport("application/json", raw, false); e != nil {
		t.Fatal(e)
	}
	for _, body := range []string{string(raw) + `{}`, strings.Replace(string(raw), `"reason":`, `"reason":"x","reason":`, 1), strings.Replace(string(raw), `"csv":`, `"tenant_id":"other","csv":`, 1), strings.Replace(string(raw), `"reason":"Reajuste conferido"`, `"reason":null`, 1)} {
		if _, e := DecodeImport("application/json", []byte(body), false); e != ErrInput {
			t.Fatal("ambiguous input accepted")
		}
	}
	if _, e := DecodeImport("text/csv", raw, false); e != ErrInput {
		t.Fatal(e)
	}
	if _, e := DecodeImport("application/json", raw, true); e != ErrInput {
		t.Fatal("hash optional")
	}
	original := importPreview(v, p)
	v.CSV = strings.Replace(v.CSV, "3,75", "3.75", 1)
	if importPreview(v, p).PreviewHash == original.PreviewHash {
		t.Fatal("bytes not bound")
	}
	v.Reason = "Other"
	if importPreview(v, p).PreviewHash == original.PreviewHash {
		t.Fatal("reason not bound")
	}
}
func TestImportPreviewLooksUpExactTenantSKUWithoutWrites(t *testing.T) {
	s, m := mockStore(t)
	v, p := importFixture()
	v.PreviewHash = ""
	m.ExpectBegin()
	undoLive(m)
	importSKUrow(m, p.Items[0].Before)
	batchRow(m, p.Items[0].Before)
	m.ExpectCommit()
	got, e := s.PreviewImport(context.Background(), fixtureActor, v)
	if e != nil || got.PreviewHash != importPreview(v, p).PreviewHash || got.Items[0].After.PriceCents != 375 {
		t.Fatal(got, e)
	}
}
func TestImportUnknownAndAmbiguousSKUNeverModifyProducts(t *testing.T) {
	for _, mode := range []string{"missing", "ambiguous", "unavailable", "stale"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, p := importFixture()
			v.PreviewHash = ""
			m.ExpectBegin()
			undoLive(m)
			q := m.ExpectQuery(`SELECT id,nome.*sku=\$2`).WithArgs(fixtureActor.Tenant, "1")
			row := sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"})
			switch mode {
			case "unavailable":
				q.WillReturnError(errors.New("private"))
			case "missing":
				q.WillReturnRows(row)
			case "ambiguous":
				q.WillReturnRows(row.AddRow(1, "Item", "", "1", "2.50", true, 1).AddRow(2, "Item2", "", "1", "2.50", true, 1))
			case "stale":
				q.WillReturnRows(row.AddRow(1, p.Items[0].Before.Name, "", "1", "2.50", true, 2))
				stale := p.Items[0].Before
				stale.Version = 2
				batchRow(m, stale)
			}
			m.ExpectRollback()
			_, e := s.PreviewImport(context.Background(), fixtureActor, v)
			want := ErrConflict
			if mode == "missing" {
				want = ErrMissing
			}
			if mode == "unavailable" {
				want = ErrUnavailable
			}
			if e != want {
				t.Fatal(e)
			}
		})
	}
}
func TestImportAtomicMetadataOutboxAndCommitFailures(t *testing.T) {
	for _, mode := range []string{"success", "metadata", "outbox", "suppressed", "commit", "hash"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, p := importFixture()
			before := p.Items[0].Before
			after := p.Items[0].After
			child := batchChild(testOp, 1)
			m.ExpectBegin()
			undoLive(m)
			undoLock(m, "import:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_imports`).WillReturnError(sql.ErrNoRows)
			undoLock(m, "batch:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_batches`).WillReturnError(sql.ErrNoRows)
			importSKUrow(m, before)
			undoLock(m, child)
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
				q := m.ExpectQuery(`INSERT INTO online_catalog_imports`)
				if mode == "metadata" {
					q.WillReturnError(errors.New("private"))
					m.ExpectRollback()
				} else {
					q.WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now()))
					event := m.ExpectExec(`INSERT INTO online_catalog_import_outbox`)
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
			result, e := s.ApplyImport(context.Background(), fixtureActor, v)
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
func TestImportReplayBeforeSKULookupAndScopedReceipt(t *testing.T) {
	for _, mode := range []string{"replay", "changed", "actor"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			v, p := importFixture()
			r := ImportReceipt{OperationID: testOp, ActorID: fixtureActor.User, Reason: v.Reason, SourceHash: creationHash([]byte(v.CSV)), PreviewHash: v.PreviewHash, Batch: BatchReceipt{OperationID: testOp, ActorID: fixtureActor.User, PreviewHash: p.PreviewHash, Items: []Receipt{{OperationID: batchChild(testOp, 1), ActorID: fixtureActor.User, ProductID: 1, Action: "price", Before: p.Items[0].Before, After: p.Items[0].After}}}}
			raw, _ := json.Marshal(r)
			payload, _ := json.Marshal(v)
			m.ExpectBegin()
			undoLive(m)
			undoLock(m, "import:"+testOp)
			m.ExpectQuery(`SELECT operation_id::text.*online_catalog_imports`).WillReturnRows(sqlmock.NewRows([]string{"op", "actor", "reason", "source", "raw", "at", "hash"}).AddRow(testOp, fixtureActor.User, v.Reason, r.SourceHash, raw, time.Now(), creationHash(payload)))
			a := fixtureActor
			if mode == "changed" {
				v.Reason = "other"
			}
			if mode == "actor" {
				a.User = undoSourceOp
			}
			if mode == "replay" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.ApplyImport(context.Background(), a, v)
			if mode == "replay" {
				if e != nil || got.OperationID != testOp {
					t.Fatal(got, e)
				}
			} else if e != ErrConflict {
				t.Fatal(e)
			}
		})
	}
	s, m := mockStore(t)
	m.ExpectQuery(`SELECT operation_id::text.*AND actor_id=\$3`).WithArgs(fixtureActor.Tenant, testOp, fixtureActor.User).WillReturnError(sql.ErrNoRows)
	if _, e := s.Import(context.Background(), fixtureActor, testOp, false); e != ErrMissing {
		t.Fatal(e)
	}
	m.ExpectQuery(`SELECT operation_id::text.*online_catalog_imports`).WithArgs(fixtureActor.Tenant, testOp).WillReturnError(sql.ErrNoRows)
	if _, e := s.Import(context.Background(), fixtureActor, testOp, true); e != ErrMissing {
		t.Fatal(e)
	}
	stock := fixtureActor
	stock.Role = "stock"
	v, _ := importFixture()
	if _, e := s.ApplyImport(context.Background(), stock, v); e != ErrDenied {
		t.Fatal(e)
	}
}
