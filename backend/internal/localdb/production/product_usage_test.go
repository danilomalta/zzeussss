package production

import "testing"

func TestProductUsageExactRoleQuantityAndOverflow(t *testing.T) {
	in := PublishInput{OperationID: "op", RecipeID: "recipe", VersionID: "version", Name: "Recipe", OutputProductID: "bread", OutputUnit: "unit", YieldMilli: 10000, Ingredients: []Ingredient{{ProductID: "flour", Unit: "g", QuantityMilli: 500001}}}
	for _, test := range []struct {
		id, role, unit string
		qty            int64
	}{{"bread", "output", "unit", 30000}, {"flour", "ingredient", "g", 1500003}} {
		role, unit, qty, err := usageFields(in, test.id, 3)
		if err != nil || role != test.role || unit != test.unit || qty != test.qty {
			t.Fatal(role, unit, qty, err)
		}
	}
	for _, test := range []struct {
		id      string
		batches int64
	}{{"missing", 1}, {"flour", MaxQuantity}, {"bread", 0}} {
		if _, _, _, err := usageFields(in, test.id, test.batches); err != ErrTrace {
			t.Fatal(test, err)
		}
	}
	in.YieldMilli = 0
	if _, _, _, err := usageFields(in, "bread", 1); err != ErrTrace {
		t.Fatal(err)
	}
}
