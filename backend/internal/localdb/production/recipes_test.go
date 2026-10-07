package production

import (
	"errors"
	"testing"
)

func TestNormalizeExactBoundsAndCanonicalReplay(t *testing.T) {
	in := PublishInput{OperationID: "op", RecipeID: "r", VersionID: "v", Name: " Receita ", OutputProductID: "out", OutputUnit: "unit", YieldMilli: MaxQuantity,
		Ingredients: []Ingredient{{"b", "ml", 1}, {"a", "g", MaxQuantity}}}
	normalized, first, err := normalize(in)
	if err != nil || normalized.Name != "Receita" || normalized.Ingredients[0].ProductID != "a" {
		t.Fatalf("normalize %+v %v", normalized, err)
	}
	if in.Ingredients[0].ProductID != "b" {
		t.Fatal("caller slice mutated")
	}
	in.Ingredients[0], in.Ingredients[1] = in.Ingredients[1], in.Ingredients[0]
	_, second, err := normalize(in)
	if err != nil || first != second {
		t.Fatalf("unstable canonical request %v", err)
	}
	in.YieldMilli++
	if _, _, err = normalize(in); !errors.Is(err, ErrInvalid) {
		t.Fatal("yield overflow", err)
	}
}
