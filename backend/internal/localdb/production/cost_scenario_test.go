package production

import (
	"testing"
)

func TestCostScenarioKnownUnknownNegativeAndZeroRevenue(t *testing.T) {
	for _, test := range []struct {
		price         int64
		wantMargin    bool
		wantNumerator string
	}{{0, false, "-100"}, {50, true, "-50"}, {100, true, "0"}, {200, true, "100"}} {
		t.Run(test.wantNumerator, func(t *testing.T) {
			zero := int64(0)
			m := MaterialCostEstimate{OutputQuantityMilli: 1000, MaterialCost: ExactCost{"100", "1"}}
			in := CostScenarioInput{Batches: 1, Labor: &LaborScenario{Workers: 1}, OverheadCents: &zero, SalePriceCents: &test.price}
			out, err := calculateScenario(m, in)
			if err != nil || out.Contribution == nil || out.Contribution.Numerator != test.wantNumerator || (out.MarginBasisPoints != nil) != test.wantMargin || out.Profit {
				t.Fatal(out, err)
			}
		})
	}
}
func TestCostScenarioAggregateRoundAndOverflow(t *testing.T) {
	m := MaterialCostEstimate{OutputQuantityMilli: 1000, MaterialCost: ExactCost{"1", "2"}}
	in := CostScenarioInput{Labor: &LaborScenario{Minutes: 1, Workers: 1, HourlyCostCents: 1}}
	out, err := calculateScenario(m, in)
	if err != nil || out.KnownCost.Numerator != "31" || out.KnownCost.Denominator != "60" || out.KnownCostCents != 1 {
		t.Fatal(out, err)
	}
	in.Labor = &LaborScenario{Minutes: 525600, Workers: 10000, HourlyCostCents: MaxQuantity}
	if _, err = calculateScenario(m, in); err == nil {
		t.Fatal("overflow accepted")
	}
	for _, tc := range [][2]int64{{0, 1}, {1, 0}, {-1, 1}, {MaxQuantity, 2}} {
		if _, err = boundedPositiveProduct(tc[0], tc[1]); err == nil {
			t.Fatal(tc)
		}
	}
}
