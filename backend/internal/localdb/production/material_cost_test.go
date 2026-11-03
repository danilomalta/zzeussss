package production

import (
	"math/big"
	"testing"
)

func TestMaterialCostExactHalfUpAndSafeBounds(t *testing.T) {
	for _, tc := range []struct{ n, d, want int64 }{{0, 1, 0}, {1, 3, 0}, {1, 2, 1}, {1499, 1000, 1}, {1500, 1000, 2}, {MaxQuantity, 1, MaxQuantity}} {
		n, err := roundedMaterialCost(new(big.Rat).SetFrac(big.NewInt(tc.n), big.NewInt(tc.d)))
		if err != nil || n != tc.want {
			t.Fatal(tc, n, err)
		}
	}
	for _, r := range []*big.Rat{big.NewRat(-1, 1), new(big.Rat).SetInt(new(big.Int).Add(big.NewInt(MaxQuantity), big.NewInt(1)))} {
		if _, err := roundedMaterialCost(r); err == nil {
			t.Fatal("unsafe amount accepted")
		}
	}
	exact := exactCost(big.NewRat(100001, 1000))
	if exact.Numerator != "100001" || exact.Denominator != "1000" {
		t.Fatal(exact)
	}
}
