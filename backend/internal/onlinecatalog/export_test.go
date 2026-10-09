package onlinecatalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func exportRows(items ...Product) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "nome", "descricao", "sku", "preco", "ativo", "version"})
	for _, p := range items {
		rows.AddRow(p.ID, p.Name, p.Description, p.SKU, fmt.Sprintf("%d.%02d", p.PriceCents/100, p.PriceCents%100), p.Active, p.Version)
	}
	return rows
}
func exportCount(m sqlmock.Sqlmock, q ExportQuery, pattern string, total int64) {
	m.ExpectQuery(`SELECT count\(\*\) FROM products`).WithArgs(fixtureActor.Tenant, q.Active, pattern).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(total))
}
func TestExportCSVExactMoneyQuotingAndImportRoundTrip(t *testing.T) {
	p := batchProduct(1)
	p.SKU = `000123;"çafé"`
	p.PriceCents = MaxPrice
	p.Version = 2
	csv, e := buildExportCSV([]Product{p}, "price")
	if e != nil {
		t.Fatal(e)
	}
	rows, e := parseImport(csv)
	if e != nil || len(rows) != 1 || rows[0].SKU != p.SKU || rows[0].Item.ExpectedVersion != 2 || *rows[0].Item.PriceCents != MaxPrice {
		t.Fatal("CSV not exact", e)
	}
	csv, e = buildExportCSV([]Product{p}, "active")
	if e != nil {
		t.Fatal(e)
	}
	rows, e = parseImport(csv)
	if e != nil || rows[0].Item.Active == nil || !*rows[0].Item.Active || rows[0].Item.PriceCents != nil {
		t.Fatal("wrong field exported")
	}
	for _, sku := range []string{"=2+2", "+SUM(A1)", "-42", "@name", " ABC", "ABC ", "A\nB", ""} {
		p.SKU = sku
		if _, e := buildExportCSV([]Product{p}, "price"); e != ErrConflict {
			t.Fatal("unsafe or altered SKU exported")
		}
	}
	p.SKU = "ABC"
	p.Version = MaxVersion
	if _, e := buildExportCSV([]Product{p}, "price"); e != ErrConflict {
		t.Fatal("unusable version exported")
	}
}
func TestExportCountAndPageUseSameScopedSnapshot(t *testing.T) {
	s, m := mockStore(t)
	q := ExportQuery{Action: "price", Query: `50%_\item`, Active: "inactive", Limit: 2, Offset: 3}
	p := batchProduct(5)
	p.SKU = "ABC"
	p.Active = false
	m.ExpectBegin()
	undoLive(m)
	exportCount(m, q, `%50\%\_\\item%`, 5)
	m.ExpectQuery(`SELECT id,nome.*ORDER BY id ASC`).WithArgs(fixtureActor.Tenant, "inactive", `%50\%\_\\item%`, 2, 3).WillReturnRows(exportRows(p, batchProduct(6)))
	m.ExpectQuery(`SELECT clock_timestamp`).WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(time.Now()))
	m.ExpectCommit()
	r, e := s.Export(context.Background(), fixtureActor, q)
	if e != nil || r.Total != 5 || r.HasMore || r.NextOffset != nil || len(r.Items) != 2 || r.SourceHash != creationHash([]byte(r.CSV)) || r.GeneratedAt.IsZero() {
		t.Fatal(r, e)
	}
}
func TestExportPaginationEmptyStateAndNoHiddenPartialPage(t *testing.T) {
	for _, mode := range []string{"more", "empty", "beyond", "count-mismatch", "formula", "rows-error", "commit", "timestamp"} {
		t.Run(mode, func(t *testing.T) {
			s, m := mockStore(t)
			q := ExportQuery{Action: "active", Active: "all", Limit: 1}
			p := batchProduct(1)
			total := int64(2)
			items := []Product{p}
			if mode == "empty" {
				total = 0
				items = nil
			}
			if mode == "beyond" {
				q.Offset = 5
				items = nil
			}
			if mode == "formula" {
				items[0].SKU = "=2+2"
			}
			if mode == "count-mismatch" {
				items = nil
			}
			m.ExpectBegin()
			undoLive(m)
			exportCount(m, q, "", total)
			query := m.ExpectQuery(`SELECT id,nome`).WithArgs(fixtureActor.Tenant, "all", "", 1, q.Offset)
			if mode == "rows-error" {
				query.WillReturnRows(exportRows(p).RowError(0, errors.New("private")))
			} else {
				query.WillReturnRows(exportRows(items...))
			}
			fail := mode == "count-mismatch" || mode == "formula" || mode == "rows-error"
			if fail {
				m.ExpectRollback()
			} else {
				stamp := m.ExpectQuery(`SELECT clock_timestamp`)
				if mode == "timestamp" {
					stamp.WillReturnError(errors.New("private"))
					m.ExpectRollback()
				} else {
					stamp.WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(time.Now()))
					if mode == "commit" {
						m.ExpectCommit().WillReturnError(errors.New("private"))
					} else {
						m.ExpectCommit()
					}
				}
			}
			r, e := s.Export(context.Background(), fixtureActor, q)
			switch mode {
			case "formula":
				if e != ErrConflict || r.CSV != "" {
					t.Fatal(r, e)
				}
			case "count-mismatch", "rows-error", "commit", "timestamp":
				if e != ErrUnavailable || r.CSV != "" {
					t.Fatal(r, e)
				}
			default:
				if e != nil || r.Items == nil {
					t.Fatal(r, e)
				}
				if mode == "more" && (r.NextOffset == nil || *r.NextOffset != 1 || !r.HasMore) {
					t.Fatal(r)
				}
				if mode != "more" && (r.HasMore || r.NextOffset != nil || r.CSV != importHeader) {
					t.Fatal(r)
				}
			}
		})
	}
}
func TestExportInvalidQueriesAndLiveRevocationFailClosed(t *testing.T) {
	valid := ExportQuery{Action: "price", Active: "all", Limit: 100}
	for _, q := range []ExportQuery{{Action: "details", Active: "all", Limit: 10}, {Action: "price", Active: "yes", Limit: 10}, {Action: "price", Active: "all", Limit: 101}, {Action: "price", Active: "all", Limit: 0}, {Action: "price", Active: "all", Limit: 1, Offset: -1}, {Action: "price", Active: "all", Limit: 1, Offset: 1000000001}, {Action: "price", Active: "all", Limit: 1, Query: strings.Repeat("a", 121)}, {Action: "price", Active: "all", Limit: 1, Query: "a\x00"}} {
		if _, e := (Store{}).Export(context.Background(), fixtureActor, q); e != ErrInput {
			t.Fatal(e)
		}
	}
	stock := fixtureActor
	stock.Role = "stock"
	if _, e := (Store{}).Export(context.Background(), stock, valid); e != ErrDenied {
		t.Fatal(e)
	}
	s, m := mockStore(t)
	m.ExpectBegin()
	m.ExpectQuery(`SELECT s.id::text`).WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	if _, e := s.Export(context.Background(), fixtureActor, valid); e != ErrDenied {
		t.Fatal("revoked session exported", e)
	}
}
