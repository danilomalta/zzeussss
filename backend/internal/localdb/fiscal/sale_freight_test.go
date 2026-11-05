package fiscal

import (
	"encoding/json"
	"errors"
	"testing"

	"titansystem-backend/internal/fiscalcore"
)

func freightContractTest() *SaleFreightContract {
	return &SaleFreightContract{SaleID: "synthetic-sale",
		Allocation:  fiscalcore.FreightAllocationInput{Method: fiscalcore.FreightLargestRemainderV1, TotalCents: 101, Items: []fiscalcore.FreightWeight{{ItemID: "one", Weight: 1}, {ItemID: "two", Weight: 1}}},
		FreightMode: "0", RefundPolicy: "proportional_with_item", SourceReference: "synthetic-source", ReviewReference: "synthetic-review",
	}
}

func TestSaleFreightCompleteImmutableGroupAndResidual(t *testing.T) {
	f := freightContractTest()
	q := TaxQuote{Input: TaxQuoteInput{OperationID: "one", Freight: f, Context: fiscalcore.TaxContext{Model: "55"}, Case: fiscalcore.TaxCase{Operation: "sale"}, Item: fiscalcore.TaxItem{Freight: "0.51"}}}
	if n, e := FreightQuoteCents(q); e != nil || n != 51 {
		t.Fatal(n, e)
	}
	q.Input.OperationID, q.Input.Item.Freight = "two", "0.50"
	if n, e := FreightQuoteCents(q); e != nil || n != 50 {
		t.Fatal(n, e)
	}
	bindings := []SaleTaxBinding{{QuoteOperationID: "one", Freight: f}, {QuoteOperationID: "two", Freight: f}}
	if e := ValidateSaleFreightBindings(bindings); e != nil {
		t.Fatal(e)
	}
	if e := ValidateSaleFreightOrigin(bindings, "different-sale"); !errors.Is(e, ErrConflict) {
		t.Fatal("commercial origin changed")
	}
	for _, mode := range []string{"missing", "duplicate", "mixed", "changed-terms", "foreign-reference"} {
		t.Run(mode, func(t *testing.T) {
			raw, _ := json.Marshal(bindings)
			var copy []SaleTaxBinding
			_ = json.Unmarshal(raw, &copy)
			switch mode {
			case "missing":
				copy = copy[:1]
			case "duplicate":
				copy[1].QuoteOperationID = "one"
			case "mixed":
				copy[1].Freight = nil
			case "changed-terms":
				copy[1].Freight.FreightMode = "1"
			case "foreign-reference":
				copy[1].QuoteOperationID = "other"
			}
			if !errors.Is(ValidateSaleFreightBindings(copy), ErrConflict) {
				t.Fatal("invalid group admitted")
			}
		})
	}
	q.Input.Item.Freight = "0.51"
	if _, e := FreightQuoteCents(q); !errors.Is(e, ErrInvalid) {
		t.Fatal("residual moved between items")
	}
	if e := ValidateSaleFreightBindings([]SaleTaxBinding{{QuoteOperationID: "legacy"}}); e != nil {
		t.Fatal("legacy behavior changed")
	}
}

func TestSaleFreightExplicitTermsAndTaxTotal(t *testing.T) {
	for _, mode := range []string{"source", "review", "refund", "method", "mode", "zero", "model", "operation"} {
		t.Run(mode, func(t *testing.T) {
			f := freightContractTest()
			q := TaxQuote{Input: TaxQuoteInput{OperationID: "one", Freight: f, Context: fiscalcore.TaxContext{Model: "55"}, Case: fiscalcore.TaxCase{Operation: "sale"}, Item: fiscalcore.TaxItem{Freight: "0.51"}}}
			switch mode {
			case "source":
				f.SourceReference = ""
			case "review":
				f.ReviewReference = ""
			case "refund":
				f.RefundPolicy = ""
			case "method":
				f.Allocation.Method = ""
			case "mode":
				f.FreightMode = "9"
			case "zero":
				f.Allocation.TotalCents = 0
			case "model":
				q.Input.Context.Model = "65"
			case "operation":
				q.Input.Case.Operation = "return"
			}
			if _, e := FreightQuoteCents(q); !errors.Is(e, ErrInvalid) {
				t.Fatal("missing terms defaulted")
			}
		})
	}
	f := freightContractTest()
	q := TaxQuote{Input: TaxQuoteInput{OperationID: "one", Freight: f, Context: fiscalcore.TaxContext{Model: "55"}, Case: fiscalcore.TaxCase{Operation: "sale"}, Item: fiscalcore.TaxItem{Freight: "0.51"}}, Result: fiscalcore.TaxResult{Gross: "10.00", Net: "10.51", Total: "10.51", GoodsXML: &fiscalcore.GoodsTaxMapping{Version: 1, Totals: map[string]string{"vST": "0.00", "vFCPST": "0.00", "vIPI": "0.00"}}}}
	b, e := saleTaxBinding(q, TaxPolicy{})
	if e != nil || b.AdditionalCents != 51 {
		t.Fatal("freight not included exactly once", b, e)
	}
	q.Result.Net = "10.50"
	if _, e = saleTaxBinding(q, TaxPolicy{}); !errors.Is(e, ErrInvalid) {
		t.Fatal("unbalanced tax quote admitted")
	}
}
