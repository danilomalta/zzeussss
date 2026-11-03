package stock

import (
	"math/big"
	"testing"
)

func TestManagementRestockExactCeilAndWholeUnits(t *testing.T) {
	for _, tc := range []struct {
		n, d int64
		unit string
		want int64
	}{{1, 3, "kg", 1}, {1000, 3, "unit", 1000}, {1001, 1, "unit", 2000}, {0, 1, "unit", 0}, {2000, 1, "unit", 2000}} {
		v, err := ceilStock(big.NewRat(tc.n, tc.d), tc.unit)
		if err != nil || v != tc.want {
			t.Fatal(tc, v, err)
		}
	}
	if _, err := ceilStock(big.NewRat(MaxAvailabilityQuantity, 1), "unit"); err == nil {
		t.Fatal("rounded integer overflow")
	}
}
