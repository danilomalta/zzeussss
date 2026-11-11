package fiscalcore

import (
	"math/big"
	"sort"
)

// FreightLargestRemainderV1 is an explicitly selected arithmetic method, not a
// tax rule or a choice of commercial allocation basis. The caller supplies the
// weights from its approved commercial contract (never inferred here).
const FreightLargestRemainderV1 = "largest_remainder_v1"

const freightMaxExact = int64(9007199254740991)

type FreightWeight struct {
	ItemID string `json:"item_id"`
	Weight int64  `json:"weight"`
}

type FreightAllocationInput struct {
	Method     string          `json:"method"`
	TotalCents int64           `json:"total_cents"`
	Items      []FreightWeight `json:"items"`
}

type FreightShare struct {
	ItemID      string `json:"item_id"`
	Weight      int64  `json:"weight"`
	AmountCents int64  `json:"amount_cents"`
}

func freightItemID(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}

// AllocateFreight performs only exact arithmetic. It does not approve a tax
// base, select the payer, create a quote, persist a snapshot or modify a sale.
// Output is canonical by item ID. Equal remainders are resolved by item ID,
// independently of input order, so a retry cannot shift the residual cent.
// A zero weight is explicit exclusion from this allocation, not a missing basis.
func AllocateFreight(in FreightAllocationInput) ([]FreightShare, error) {
	if in.Method != FreightLargestRemainderV1 || in.TotalCents < 0 || in.TotalCents > freightMaxExact || len(in.Items) == 0 || len(in.Items) > 500 {
		return nil, ErrInput
	}
	totalWeight := new(big.Int)
	seen := make(map[string]bool, len(in.Items))
	for _, item := range in.Items {
		if !freightItemID(item.ItemID) || seen[item.ItemID] || item.Weight < 0 || item.Weight > freightMaxExact {
			return nil, ErrInput
		}
		seen[item.ItemID] = true
		totalWeight.Add(totalWeight, big.NewInt(item.Weight))
	}
	if totalWeight.Sign() == 0 {
		return nil, ErrInput
	}
	type remainder struct {
		index int
		value *big.Int
	}
	shares := make([]FreightShare, len(in.Items))
	remainders := make([]remainder, len(in.Items))
	allocated := int64(0)
	for i, item := range in.Items {
		product := new(big.Int).Mul(big.NewInt(in.TotalCents), big.NewInt(item.Weight))
		quotient, rest := new(big.Int), new(big.Int)
		quotient.QuoRem(product, totalWeight, rest)
		amount := quotient.Int64() // 0 <= quotient <= TotalCents, already bounded.
		shares[i] = FreightShare{ItemID: item.ItemID, Weight: item.Weight, AmountCents: amount}
		remainders[i] = remainder{index: i, value: rest}
		allocated += amount // Sum of floors never exceeds TotalCents.
	}
	sort.Slice(remainders, func(i, j int) bool {
		comparison := remainders[i].value.Cmp(remainders[j].value)
		if comparison != 0 {
			return comparison > 0
		}
		return shares[remainders[i].index].ItemID < shares[remainders[j].index].ItemID
	})
	for i := int64(0); i < in.TotalCents-allocated; i++ {
		// Sum of fractional remainders is an integer smaller than item count.
		shares[remainders[i].index].AmountCents++
	}
	sort.Slice(shares, func(i, j int) bool { return shares[i].ItemID < shares[j].ItemID })
	return shares, nil
}
