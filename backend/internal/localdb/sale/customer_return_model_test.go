package sale

import "testing"

func TestCustomerReturnCumulativeProportionAndLargeProducts(t *testing.T) {
	for _, tc := range []struct{ total, sold, first, second, wantFirst, wantSecond int64 }{
		{2, 1500, 1000, 500, 1, 1},
		{1, 3, 1, 2, 0, 1},
		{100, 3, 1, 2, 33, 67},
		{9007199254740991, 9007199254740991, 9007199254740990, 1, 9007199254740990, 1},
	} {
		first := returnProportion(tc.total, tc.first, tc.sold)
		second := returnProportion(tc.total, tc.first+tc.second, tc.sold) - first
		if first != tc.wantFirst || second != tc.wantSecond || first+second != tc.total {
			t.Fatal(tc, first, second)
		}
	}
}
