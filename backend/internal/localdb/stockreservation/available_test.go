package stockreservation

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"testing"

	_ "modernc.org/sqlite"
)

func TestHeldScopesStatesAndRejectsAggregateOverflow(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE production_material_reservations(tenant_id TEXT,store_id TEXT,id TEXT,location_id TEXT,status TEXT)`,
		`CREATE TABLE production_material_items(tenant_id TEXT,store_id TEXT,reservation_id TEXT,product_id TEXT,quantity_milli INTEGER)`,
		`INSERT INTO production_material_reservations VALUES('t','s','a','l','active'),('t','s','b','l','released'),('t','s','c','l','consumed'),('other','s','d','l','active'),('t','other','e','l','active'),('t','s','f','other','active')`,
		`INSERT INTO production_material_items VALUES('t','s','a','p',7),('t','s','b','p',100),('t','s','c','p',100),('other','s','d','p',100),('t','other','e','p',100),('t','s','f','p',100)`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	held, err := HeldTx(ctx, tx, "t", "s", "p", "l")
	if err != nil || held != 7 {
		t.Fatal("scope/state", held, err)
	}
	free, err := FreeTx(ctx, tx, "t", "s", "p", "l", 10)
	if err != nil || free != 3 {
		t.Fatal(free, err)
	}
	if _, err = FreeTx(ctx, tx, "t", "s", "p", "l", 6); !errors.Is(err, ErrUnavailable) {
		t.Fatal("overcommitted", err)
	}
	if _, err = FreeTx(ctx, tx, "t", "s", "p", "l", -1); !errors.Is(err, ErrUnavailable) {
		t.Fatal("negative", err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE production_material_items SET quantity_milli=? WHERE reservation_id='a'`, math.MaxInt64); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO production_material_items VALUES('t','s','a','p',1)`); err != nil {
		t.Fatal(err)
	}
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = HeldTx(ctx, tx, "t", "s", "p", "l"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("aggregate overflow", err)
	}
}
