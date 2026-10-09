package onlinecatalog

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestSearchEscapesLiteralWildcardsAndScopesAllArguments(t *testing.T) {
	s, m := mockStore(t)
	m.ExpectQuery(`SELECT id,nome`).WithArgs(fixtureActor.Tenant, "inactive", `%50\%\_\\item%`, 2, 3).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}).AddRow(5, "Item", "", "ABC", "2.50", false, 2))
	items, e := s.Search(context.Background(), fixtureActor, `50%_\item`, "inactive", 2, 3)
	if e != nil || len(items) != 1 || items[0].Active || items[0].PriceCents != 250 {
		t.Fatal(items, e)
	}
}
func TestSearchEmptyStateIsArrayAndInvalidInputNeverQueries(t *testing.T) {
	s, m := mockStore(t)
	m.ExpectQuery(`SELECT id,nome`).WithArgs(fixtureActor.Tenant, "all", "", 50, 0).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "sku", "price", "active", "version"}))
	items, e := s.Search(context.Background(), fixtureActor, "", "all", 50, 0)
	if e != nil || items == nil || len(items) != 0 {
		t.Fatal(items, e)
	}
	if _, e = s.Search(context.Background(), fixtureActor, "", "unknown", 50, 0); e != ErrInput {
		t.Fatal(e)
	}
}
