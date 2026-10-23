package purchases

import (
	"math/big"
	"testing"
)

func TestReceivingBoardExactGroupsAboveJavaScriptLimit(t *testing.T) {
	groups := map[[3]string]*boardAccumulator{}
	r := ReceivingOverview{SupplierID: "s", ProductID: "p", Unit: "kg", PlannedMilli: MaxQuantity, RemainingMilli: MaxQuantity, CommercialStatus: "local_not_sent"}
	boardGroupAdd(groups, r)
	boardGroupAdd(groups, r)
	g := groups[[3]string{"s", "p", "kg"}]
	want := new(big.Int).Mul(big.NewInt(MaxQuantity), big.NewInt(2)).String()
	if g.planned.String() != want || g.pending.String() != want || g.row.PendingOrders != 2 {
		t.Fatal(g, want)
	}
}
func TestReceivingBoardGroupsSeparateUnitsAndCancelledRemainder(t *testing.T) {
	groups := map[[3]string]*boardAccumulator{}
	r := ReceivingOverview{SupplierID: "s", ProductID: "p", Unit: "kg", PlannedMilli: 1000, RemainingMilli: 1000, CommercialStatus: "cancelled"}
	boardGroupAdd(groups, r)
	r.Unit = "unit"
	r.CommercialStatus = "local_not_sent"
	boardGroupAdd(groups, r)
	if len(groups) != 2 || groups[[3]string{"s", "p", "kg"}].pending.Sign() != 0 || groups[[3]string{"s", "p", "kg"}].remaining.String() != "1000" || groups[[3]string{"s", "p", "unit"}].pending.String() != "1000" {
		t.Fatal(groups)
	}
}
func TestReceivingBoardPeriodTimezoneBoundsAndStates(t *testing.T) {
	in := BoardInput{From: "2026-01-02T07:00:00-03:00", To: "2026-01-02T08:00:00-03:00", State: "pending", Limit: 50}
	from, to, e := boardRange(in)
	if e != nil || from.Hour() != 10 || to.Hour() != 11 {
		t.Fatal(from, to, e)
	}
	for _, bad := range []BoardInput{{From: "bad", To: in.To, State: "all", Limit: 50}, {From: in.From, To: in.From, State: "all", Limit: 50}, {From: in.From, To: in.To, State: "paid", Limit: 50}, {From: "2025-01-01T00:00:00Z", To: "2026-02-01T00:00:00Z", State: "all", Limit: 50}, {From: in.From, To: in.To, State: "all", Limit: 0}} {
		if _, _, e := boardRange(bad); e == nil {
			t.Fatal(bad)
		}
	}
}
func TestReceivingBoardStateFilterDoesNotReopenCancelledOrder(t *testing.T) {
	r := ReceivingOverview{CommercialStatus: "cancelled", RemainingMilli: 5000}
	if boardMatches(r, "pending") || boardMatches(r, "received") || !boardMatches(r, "cancelled") {
		t.Fatal(r)
	}
	r.CommercialStatus = "local_not_sent"
	r.RemainingMilli = 0
	if !boardMatches(r, "received") || boardMatches(r, "pending") || boardMatches(r, "review") {
		t.Fatal(r)
	}
}
