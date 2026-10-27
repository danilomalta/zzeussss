package sale

import (
	"strings"
	"testing"
)

func TestCheckoutPlanBoundsRejectUnrepresentableOrAmbiguousAllocations(t *testing.T) {
	base := func() PlanInput {
		return PlanInput{OperationID: "op", CheckoutID: "cart", CashSessionID: "shift", ExpectedTotalCents: 31500, Items: []Item{{ProductID: "p", LocationID: "shelf", QuantityMilli: 1000}}, Parts: []PlanPartInput{{PartID: "a", Method: "cash", AmountCents: 31500}}}
	}
	for name, change := range map[string]func(*PlanInput){
		"unsafe total": func(in *PlanInput) {
			in.ExpectedTotalCents = checkoutMaxSafe + 1
			in.Parts[0].AmountCents = in.ExpectedTotalCents
		},
		"overflow sum": func(in *PlanInput) {
			in.Parts = []PlanPartInput{{PartID: "a", Method: "cash", AmountCents: checkoutMaxSafe}, {PartID: "b", Method: "pix", AmountCents: checkoutMaxSafe}}
		},
		"too many parts": func(in *PlanInput) { in.Parts = make([]PlanPartInput, 21) },
		"empty parts":    func(in *PlanInput) { in.Parts = nil },
		"spaced id":      func(in *PlanInput) { in.CheckoutID = " cart" },
		"long id":        func(in *PlanInput) { in.CheckoutID = strings.Repeat("x", 129) },
		"empty item":     func(in *PlanInput) { in.Items = nil },
	} {
		t.Run(name, func(t *testing.T) {
			in := base()
			change(&in)
			if validPlan(in) {
				t.Fatal("invalid allocation accepted")
			}
		})
	}
	in := base()
	in.ExpectedTotalCents = checkoutMaxSafe
	in.Parts[0].AmountCents = checkoutMaxSafe
	if !validPlan(in) {
		t.Fatal("exact safe boundary rejected")
	}
}
