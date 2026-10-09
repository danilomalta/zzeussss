package production

import (
	"errors"
	"testing"
)

func TestPlannedTraceIngredientsExactArithmeticAndOverflow(t *testing.T) {
	recipe := Version{PublishInput: PublishInput{OperationID: "op", RecipeID: "r", VersionID: "v", Name: "Recipe", OutputProductID: "out", OutputUnit: "g", YieldMilli: 1, Ingredients: []Ingredient{{ProductID: "flour", Unit: "g", QuantityMilli: 2}}}}
	order := Order{VersionID: "v", Recipe: recipe, PlannedBatches: 3, PlannedOutputMilli: 3}
	items, err := plannedTraceIngredients(order)
	if err != nil || len(items) != 1 || items[0].PlannedMilli != 6 || items[0].ReservedMilli != 0 || items[0].ConsumedMilli != 0 {
		t.Fatal(items, err)
	}
	order.PlannedBatches = MaxQuantity
	order.PlannedOutputMilli = MaxQuantity
	if _, err = plannedTraceIngredients(order); !errors.Is(err, ErrTrace) {
		t.Fatal("ingredient overflow", err)
	}
	order.PlannedBatches = 1
	order.PlannedOutputMilli = 1
	order.Recipe.Ingredients[0].QuantityMilli = 0
	if _, err = plannedTraceIngredients(order); !errors.Is(err, ErrTrace) {
		t.Fatal("zero must fail before division", err)
	}
	order.Recipe.Ingredients[0].QuantityMilli = 1
	order.VersionID = "other"
	if _, err = plannedTraceIngredients(order); !errors.Is(err, ErrTrace) {
		t.Fatal("frozen version mismatch", err)
	}
}
