package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func exportAdjustmentFixture() AdjustmentReceipt {
	v, p := adjustmentFixture()
	return AdjustmentReceipt{OperationID: testOp, ActorID: fixtureActor.User, Reason: v.Reason, RateBasisPoints: v.RateBasisPoints, Rounding: "half_up", PreviewHash: v.PreviewHash, CreatedAt: time.Date(2026, 10, 9, 10, 0, 0, 123, time.UTC), Batch: BatchReceipt{OperationID: testOp, ActorID: fixtureActor.User, PreviewHash: p.PreviewHash, Items: []Receipt{{OperationID: batchChild(testOp, 1), ActorID: fixtureActor.User, ProductID: 1, Action: "price", Before: p.Items[0].Before, After: p.Items[0].After}}}}
}
func adjustmentReportRows(items ...AdjustmentReceipt) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"op", "actor", "reason", "rate", "snapshot", "at", "hash"})
	for _, r := range items {
		raw, _ := json.Marshal(r)
		rows.AddRow(r.OperationID, r.ActorID, r.Reason, r.RateBasisPoints, raw, r.CreatedAt, strings.Repeat("a", 64))
	}
	return rows
}
func TestAdjustmentExportProtectsSpreadsheetTextKeepsExactSnapshotsAndFiltersRows(t *testing.T) {
	r := exportAdjustmentFixture()
	r.Reason = `=HYPERLINK("site"); reajuste`
	r.Batch.Items[0].Before.SKU = "000123"
	r.Batch.Items[0].After.SKU = "000123"
	r.Batch.Items[0].Before.Name = `@Item;"çafé"`
	r.Batch.Items[0].After.Name = r.Batch.Items[0].Before.Name
	out, n, e := buildAdjustmentExportCSV([]AdjustmentReceipt{r}, 0)
	if e != nil || n != 1 {
		t.Fatal(e)
	}
	reader := csv.NewReader(strings.NewReader(out))
	reader.Comma = ';'
	records, e := reader.ReadAll()
	if e != nil || len(records) != 2 || len(records[1]) != 18 {
		t.Fatal(e)
	}
	row := records[1]
	if row[5] != "'000123" || row[6] != "'@Item;\"çafé\"" || row[11] != "250" || row[12] != "375" || row[13] != "2.50" || row[14] != "3.75" || row[15] != "5000" || row[16] != "'"+r.Reason {
		t.Fatal(row)
	}
	if !strings.HasPrefix(row[0], "'") || row[2] != "'2026-10-09T10:00:00.000000123Z" {
		t.Fatal(row)
	}
	empty, n, e := buildAdjustmentExportCSV([]AdjustmentReceipt{r}, 2)
	if e != nil || n != 0 || empty != adjustmentExportHeader {
		t.Fatal(empty, n, e)
	}
	r.Batch.Items[0].Before.Name = "unsafe\nname"
	if _, _, e = buildAdjustmentExportCSV([]AdjustmentReceipt{r}, 0); e != ErrUnavailable {
		t.Fatal(e)
	}
}
func TestAdjustmentExportBoundedFiltersAndRoles(t *testing.T) {
	valid := AdjustmentExportQuery{Limit: 10}
	cases := []AdjustmentExportQuery{{Limit: 11}, {Limit: 0}, {Limit: 1, Offset: -1}, {Limit: 1, Offset: 1000000001}, {Limit: 1, ActorID: "foreign"}, {Limit: 1, ProductID: -1}, {Limit: 1, ProductID: MaxVersion + 1}, {Limit: 1, From: "2026-10-01T00:00:00Z"}, {Limit: 1, From: "x", Until: "y"}, {Limit: 1, From: "2026-10-02T00:00:00Z", Until: "2026-10-01T00:00:00Z"}, {Limit: 1, From: "2025-01-01T00:00:00Z", Until: "2026-10-01T00:00:00Z"}, {Limit: 1, From: "1999-01-01T00:00:00Z", Until: "1999-02-01T00:00:00Z"}}
	for _, q := range cases {
		if _, e := (Store{}).ExportAdjustments(context.Background(), fixtureActor, q); e != ErrInput {
			t.Fatal(q, e)
		}
	}
	valid.From = "2026-10-09T00:00:00-03:00"
	valid.Until = "2026-10-10T00:00:00-03:00"
	if ValidateAdjustmentExport(valid) != nil {
		t.Fatal("timezone rejected")
	}
	for _, role := range []string{"stock", "cashier", "employee", "accountant"} {
		a := fixtureActor
		a.Role = role
		if _, e := (Store{}).ExportAdjustments(context.Background(), a, valid); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
func TestAdjustmentExportSnapshotPagingAndUTCFilters(t *testing.T) {
	s, m := mockStore(t)
	r := exportAdjustmentFixture()
	q := AdjustmentExportQuery{Limit: 1, Offset: 3, ActorID: fixtureActor.User, ProductID: 1, From: "2026-10-09T00:00:00-03:00", Until: "2026-10-10T00:00:00-03:00"}
	from := time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC)
	until := from.Add(24 * time.Hour)
	m.ExpectBegin()
	undoLive(m)
	m.ExpectQuery(`SELECT count`).WithArgs(fixtureActor.Tenant, fixtureActor.User, from, until, "1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	m.ExpectQuery(`SELECT operation_id::text.*ORDER BY created_at DESC,operation_id DESC`).WithArgs(fixtureActor.Tenant, fixtureActor.User, from, until, "1", 1, 3).WillReturnRows(adjustmentReportRows(r))
	m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(time.Now()))
	m.ExpectCommit()
	page, e := s.ExportAdjustments(context.Background(), fixtureActor, q)
	if e != nil || page.Total != 5 || page.RowCount != 1 || !page.HasMore || page.NextOffset == nil || *page.NextOffset != 4 || page.TextPrefix != "'" || page.SourceHash != creationHash([]byte(page.CSV)) || len(page.Items) != 1 {
		t.Fatal(page, e)
	}
}
func TestAdjustmentExportFailuresNeverReturnPartialReport(t *testing.T) {
	for _, mode := range []string{"empty", "beyond", "count", "rows", "snapshot", "actor", "product", "date", "order", "commit", "stamp", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			q := AdjustmentExportQuery{Limit: 2}
			r := exportAdjustmentFixture()
			items := []AdjustmentReceipt{r}
			total := int64(1)
			if mode == "actor" {
				q.ActorID = undoSourceOp
			}
			if mode == "product" {
				q.ProductID = 2
			}
			if mode == "date" {
				q.From = "2026-10-10T00:00:00Z"
				q.Until = "2026-10-11T00:00:00Z"
			}
			if mode == "empty" {
				items = nil
				total = 0
			}
			if mode == "beyond" {
				q.Offset = 5
				items = nil
			}
			if mode == "count" {
				items = nil
			}
			if mode == "snapshot" {
				items[0].Batch.Items[0].After.PriceCents++
			}
			if mode == "order" {
				items = append(items, r)
				total = 2
			}
			m.ExpectBegin()
			if mode == "revoked" {
				m.ExpectQuery(`SELECT s.id::text`).WillReturnError(sql.ErrNoRows)
				m.ExpectRollback()
			} else {
				undoLive(m)
				m.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(total))
				rows := adjustmentReportRows(items...)
				if mode == "rows" {
					rows.RowError(0, errors.New("private"))
				}
				m.ExpectQuery(`SELECT operation_id::text`).WillReturnRows(rows)
				fail := mode == "count" || mode == "rows" || mode == "snapshot" || mode == "actor" || mode == "product" || mode == "date" || mode == "order"
				if fail {
					m.ExpectRollback()
				} else if mode == "stamp" {
					m.ExpectQuery(`SELECT clock_timestamp`).WillReturnError(errors.New("private"))
					m.ExpectRollback()
				} else {
					m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(time.Now()))
					if mode == "commit" {
						m.ExpectCommit().WillReturnError(errors.New("private"))
					} else {
						m.ExpectCommit()
					}
				}
			}
			page, e := s.ExportAdjustments(context.Background(), fixtureActor, q)
			if mode == "empty" || mode == "beyond" {
				if e != nil || page.Items == nil || page.CSV != adjustmentExportHeader || page.RowCount != 0 || page.HasMore {
					t.Fatal(page, e)
				}
			} else {
				want := ErrUnavailable
				if mode == "revoked" {
					want = ErrDenied
				}
				if e != want || page.CSV != "" || len(page.Items) != 0 {
					t.Fatal(page, e)
				}
			}
		})
	}
}
