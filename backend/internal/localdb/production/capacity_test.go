package production

import (
	"errors"
	"testing"
)

func TestCapacityExactUnitRatiosAndBatchFloor(t *testing.T) {
	for _, c := range []struct {
		stockUnit, recipeUnit string
		stock, required, want int64
	}{
		{"kg", "g", 4000, 500000, 8},
		{"g", "kg", 4000000, 500, 8},
		{"liter", "ml", 1000, 333000, 3},
		{"ml", "liter", 999999, 1000, 0},
		{"g", "g", 999, 1000, 0},
		{"unit", "unit", 0, 1000, 0},
		{"g", "g", MaxQuantity, MaxQuantity, 1},
		{"kg", "g", MaxQuantity, MaxQuantity, 1000},
	} {
		n, d, err := UnitRatio(c.stockUnit, c.recipeUnit)
		if err != nil {
			t.Fatal(err)
		}
		got, err := batchCount(c.stock, c.required, n, d)
		if err != nil || got != c.want {
			t.Fatalf("%+v got %d: %v", c, got, err)
		}
	}
	for _, pair := range [][2]string{{"ml", "g"}, {"liter", "kg"}, {"unit", "g"}, {"meter", "kg"}, {"unknown", "unknown"}} {
		if _, _, err := UnitRatio(pair[0], pair[1]); !errors.Is(err, ErrCapacity) {
			t.Fatal("unsafe conversion", pair, err)
		}
	}
	for _, c := range [][4]int64{{-1, 1, 1, 1}, {1, 0, 1, 1}, {MaxQuantity + 1, 1, 1, 1}, {MaxQuantity, 1, 1000, 1}, {1, 1, 0, 1}, {1, 1, 1, 0}} {
		if _, err := batchCount(c[0], c[1], c[2], c[3]); !errors.Is(err, ErrCapacity) {
			t.Fatal("unsafe bound", c, err)
		}
	}
}
