// Package management provides bounded, read-only reports for one authorized
// company and store. It never ranks another business or sends supplier data.
package management

import (
	"errors"
	"math/big"
	"time"
)

const MaxExact int64 = 9007199254740991
const MaxSourceRows = 100000

var ErrInvalid = errors.New("invalid report request")
var ErrLimit = errors.New("report source limit exceeded; select a smaller period")
var ErrIntegrity = errors.New("report source inconsistent")

type Window struct {
	From time.Time
	To   time.Time
}
type SalesInput struct {
	Current  Window
	Previous Window
	Limit    int
	Offset   int
}
type ProductSales struct {
	ProductID                string `json:"product_id"`
	SKU                      string `json:"sku"`
	Name                     string `json:"name"`
	Unit                     string `json:"unit"`
	QuantityMilli            int64  `json:"quantity_milli"`
	RevenueCents             int64  `json:"revenue_cents"`
	PreviousQuantityMilli    int64  `json:"previous_quantity_milli"`
	PreviousRevenueCents     int64  `json:"previous_revenue_cents"`
	RevenueDeltaCents        int64  `json:"revenue_delta_cents"`
	RevenueChangeBasisPoints *int64 `json:"revenue_change_basis_points"`
	ChangeBasis              string `json:"change_basis"`
	ABCCategory              string `json:"abc_category"`
}
type SalesReport struct {
	From                 string         `json:"from"`
	To                   string         `json:"to"`
	CompareFrom          string         `json:"compare_from"`
	CompareTo            string         `json:"compare_to"`
	CurrentSeconds       int64          `json:"current_seconds"`
	PreviousSeconds      int64          `json:"previous_seconds"`
	EqualDuration        bool           `json:"equal_duration"`
	Basis                string         `json:"basis"`
	ABCBasis             string         `json:"abc_basis"`
	PaymentConfirmed     bool           `json:"payment_confirmed"`
	Profit               bool           `json:"profit"`
	SourceRows           int            `json:"source_rows"`
	Items                []ProductSales `json:"items"`
	TotalProducts        int            `json:"total_products"`
	Limit                int            `json:"limit"`
	Offset               int            `json:"offset"`
	RevenueCents         int64          `json:"revenue_cents"`
	PreviousRevenueCents int64          `json:"previous_revenue_cents"`
}

func validWindow(w Window) bool {
	return !w.From.IsZero() && !w.To.IsZero() && w.From.Before(w.To) && w.To.Sub(w.From) <= 366*24*time.Hour && w.From.Year() >= 2000 && w.To.Year() <= 2200
}
func windowSeconds(w Window) int64 { return int64(w.To.Sub(w.From) / time.Second) }
func exactInt(v *big.Int) (int64, error) {
	if !v.IsInt64() || v.Sign() < 0 || v.Cmp(big.NewInt(MaxExact)) > 0 {
		return 0, ErrIntegrity
	}
	return v.Int64(), nil
}
func change(current, previous int64) (*int64, string) {
	if previous == 0 {
		if current == 0 {
			return nil, "no_baseline"
		}
		return nil, "new_activity"
	}
	n := new(big.Int).Mul(big.NewInt(current-previous), big.NewInt(10000))
	n.Quo(n, big.NewInt(previous))
	if !n.IsInt64() || n.Cmp(big.NewInt(MaxExact)) > 0 || n.Cmp(big.NewInt(-MaxExact)) < 0 {
		return nil, "change_out_of_range"
	}
	v := n.Int64()
	return &v, "comparable_totals"
}
