package fiscalcore

import (
	"errors"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestFreightAllocationExactResidualAndStableIdentity(t *testing.T) {
	in := FreightAllocationInput{Method: FreightLargestRemainderV1, TotalCents: 100, Items: []FreightWeight{{"c", 1}, {"a", 1}, {"b", 1}, {"excluded", 0}}}
	before := append([]FreightWeight(nil), in.Items...)
	want := []FreightShare{{"a", 1, 34}, {"b", 1, 33}, {"c", 1, 33}, {"excluded", 0, 0}}
	got, err := AllocateFreight(in)
	if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(before, in.Items) {
		t.Fatalf("allocation or input mutation: %v %v", got, err)
	}
	in.Items = []FreightWeight{{"excluded", 0}, {"b", 1}, {"a", 1}, {"c", 1}}
	repeated, err := AllocateFreight(in)
	if err != nil || !reflect.DeepEqual(repeated, got) {
		t.Fatal("item order changed residual allocation")
	}
	// A caller modifying its result cannot mutate a later calculation.
	got[0].AmountCents = 999
	repeated, err = AllocateFreight(in)
	if err != nil || !reflect.DeepEqual(repeated, want) {
		t.Fatal("result aliases another request")
	}
}

func TestFreightAllocationLimitsAndExplicitMethod(t *testing.T) {
	valid := FreightAllocationInput{Method: FreightLargestRemainderV1, TotalCents: 1, Items: []FreightWeight{{"item", 1}}}
	cases := map[string]func(*FreightAllocationInput){
		"missing-method":  func(v *FreightAllocationInput) { v.Method = "" },
		"unknown-method":  func(v *FreightAllocationInput) { v.Method = "by-price" },
		"negative-total":  func(v *FreightAllocationInput) { v.TotalCents = -1 },
		"overflow-total":  func(v *FreightAllocationInput) { v.TotalCents = freightMaxExact + 1 },
		"missing-items":   func(v *FreightAllocationInput) { v.Items = nil },
		"missing-id":      func(v *FreightAllocationInput) { v.Items[0].ItemID = "" },
		"control-id":      func(v *FreightAllocationInput) { v.Items[0].ItemID = "sensitive\nvalue" },
		"long-id":         func(v *FreightAllocationInput) { v.Items[0].ItemID = strings.Repeat("a", 129) },
		"negative-weight": func(v *FreightAllocationInput) { v.Items[0].Weight = -1 },
		"overflow-weight": func(v *FreightAllocationInput) { v.Items[0].Weight = freightMaxExact + 1 },
		"no-basis":        func(v *FreightAllocationInput) { v.Items[0].Weight = 0 },
		"duplicate-id":    func(v *FreightAllocationInput) { v.Items = append(v.Items, v.Items[0]) },
		"too-many-items":  func(v *FreightAllocationInput) { v.Items = make([]FreightWeight, 501) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := valid
			in.Items = append([]FreightWeight(nil), valid.Items...)
			change(&in)
			out, err := AllocateFreight(in)
			if !errors.Is(err, ErrInput) || out != nil || err.Error() != ErrInput.Error() {
				t.Fatal("invalid input accepted or input disclosed")
			}
		})
	}
}

func TestFreightAllocationExactArithmeticAtMaximum(t *testing.T) {
	// The product total*weight exceeds int64; summing weights also exceeds
	// the exact-integer range of JavaScript. Neither uses floating point.
	items := make([]FreightWeight, 500)
	for i := range items {
		items[i] = FreightWeight{ItemID: string(rune('A'+i/26)) + string(rune('A'+i%26)), Weight: freightMaxExact}
	}
	for _, total := range []int64{0, 1, 499, 500, 501, freightMaxExact} {
		out, err := AllocateFreight(FreightAllocationInput{Method: FreightLargestRemainderV1, TotalCents: total, Items: items})
		if err != nil {
			t.Fatal(err)
		}
		sum := new(big.Int)
		for _, share := range out {
			if share.AmountCents < 0 || share.AmountCents > total {
				t.Fatal("allocation outside range")
			}
			sum.Add(sum, big.NewInt(share.AmountCents))
		}
		if sum.Cmp(big.NewInt(total)) != 0 {
			t.Fatal("allocation lost or duplicated cents")
		}
	}
}

func TestFreightAllocationParallelAndRationalBounds(t *testing.T) {
	in := FreightAllocationInput{Method: FreightLargestRemainderV1, TotalCents: 1234567, Items: []FreightWeight{{"x", 7}, {"y", 11}, {"z", 13}}}
	want, err := AllocateFreight(in)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := AllocateFreight(in)
			if err != nil || !reflect.DeepEqual(out, want) {
				t.Error("parallel allocation changed result")
			}
		}()
	}
	wg.Wait()
	var sum int64
	for _, share := range want {
		exact := new(big.Rat).SetFrac(new(big.Int).Mul(big.NewInt(in.TotalCents), big.NewInt(share.Weight)), big.NewInt(31))
		diff := new(big.Rat).Sub(new(big.Rat).SetInt64(share.AmountCents), exact)
		diff.Abs(diff)
		if diff.Cmp(big.NewRat(1, 1)) >= 0 {
			t.Fatal("allocation differs by at least one cent from exact proportion")
		}
		sum += share.AmountCents
	}
	if sum != in.TotalCents {
		t.Fatal("inexact total")
	}
}
