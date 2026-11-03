package stock

import (
	"context"
	"database/sql"
	"math"
	_ "modernc.org/sqlite"
	"testing"
	"titansystem-backend/internal/localdb/identity"
)

func TestExactAvailabilityTotalRejectsFinalOverflowAllowsCancellingMovements(t *testing.T) {
	for _, test := range []struct {
		values  []int64
		want    int64
		invalid bool
	}{{[]int64{-1}, 0, true}, {[]int64{MaxAvailabilityQuantity, 1}, 0, true}, {[]int64{math.MaxInt64, math.MaxInt64}, 0, true}, {nil, 0, false}, {[]int64{MaxAvailabilityQuantity}, MaxAvailabilityQuantity, false}, {[]int64{math.MaxInt64, math.MaxInt64, -math.MaxInt64, -math.MaxInt64, 1001}, 1001, false}} {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		db.SetMaxOpenConns(1)
		if _, err = db.Exec(`CREATE TABLE stock_movements(tenant_id TEXT,store_id TEXT,product_id TEXT,location_id TEXT,quantity_milli INTEGER)`); err != nil {
			t.Fatal(err)
		}
		for _, q := range test.values {
			if _, err = db.Exec(`INSERT INTO stock_movements VALUES('t','s','p','l',?)`, q); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = db.Exec(`INSERT INTO stock_movements VALUES('other','s','p','l',9),('t','other','p','l',9),('t','s','other','l',9),('t','s','p','other',9)`); err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		n, err := physicalAvailabilityTx(context.Background(), tx, identity.Scope{TenantID: "t", StoreID: "s"}, "p", "l")
		tx.Rollback()
		if test.invalid {
			if err != ErrAvailability {
				t.Fatal(test, err)
			}
		} else if err != nil || n != test.want {
			t.Fatal(test, n, err)
		}
	}
	for _, id := range []string{"", " x", "x\x00", "x\n"} {
		if validAvailabilityID(id) {
			t.Fatal(id)
		}
	}
}
