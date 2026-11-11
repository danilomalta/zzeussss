package fiscal

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"titansystem-backend/internal/fiscalcore"
)

// SaleFreightContract is explicit customer input frozen BEFORE payment. It is
// not a default tax rule or proof of accountant approval. Unsupported refund
// policies remain refused; the supported policy uses the existing cumulative
// item refund allocation, including that item's freight and additional taxes.
type SaleFreightContract struct {
	SaleID          string                            `json:"sale_id"`
	Allocation      fiscalcore.FreightAllocationInput `json:"allocation"`
	FreightMode     string                            `json:"freight_mode"`
	RefundPolicy    string                            `json:"refund_policy"`
	SourceReference string                            `json:"source_reference"`
	ReviewReference string                            `json:"review_reference"`
}

func (f *SaleFreightContract) canonical() ([]byte, error) {
	if f == nil || !idPattern.MatchString(f.SaleID) || !shortText(f.SourceReference) || !shortText(f.ReviewReference) || f.RefundPolicy != "proportional_with_item" || f.Allocation.TotalCents <= 0 || len(f.Allocation.Items) > 100 {
		return nil, ErrInvalid
	}
	switch f.FreightMode {
	case "0", "1", "2", "3", "4":
	default:
		return nil, ErrInvalid
	}
	shares, err := fiscalcore.AllocateFreight(f.Allocation)
	if err != nil {
		return nil, ErrInvalid
	}
	copy := *f
	copy.Allocation.Items = make([]fiscalcore.FreightWeight, len(shares))
	for i, share := range shares {
		if !idPattern.MatchString(share.ItemID) {
			return nil, ErrInvalid
		}
		copy.Allocation.Items[i] = fiscalcore.FreightWeight{ItemID: share.ItemID, Weight: share.Weight}
	}
	return json.Marshal(copy)
}

func shortText(s string) bool {
	if len(s) == 0 || len(s) > 200 || !utf8.ValidString(s) || strings.TrimSpace(s) != s {
		return false
	}
	for _, c := range s {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}

func saleFreightCents(q TaxQuote) (int64, error) {
	if q.Input.Freight == nil {
		return 0, nil
	}
	if _, err := q.Input.Freight.canonical(); err != nil || q.Input.Context.Model != "55" || q.Input.Case.Operation != "sale" {
		return 0, ErrInvalid
	}
	shares, err := fiscalcore.AllocateFreight(q.Input.Freight.Allocation)
	if err != nil {
		return 0, ErrInvalid
	}
	for _, share := range shares {
		if share.ItemID == q.Input.OperationID {
			cents, err := taxCents(q.Input.Item.Freight)
			if err != nil || cents != share.AmountCents {
				return 0, ErrInvalid
			}
			return cents, nil
		}
	}
	return 0, ErrInvalid
}

// ValidateSaleFreightBindings requires the WHOLE allocation in the sale, with
// identical terms. A subset, unrelated quote or changed responsibility cannot
// consume a group's residual or attach freight to a different commercial sale.
func ValidateSaleFreightBindings(bindings []SaleTaxBinding) error {
	var contract *SaleFreightContract
	var canonical []byte
	for _, binding := range bindings {
		if binding.Freight != nil {
			var err error
			canonical, err = binding.Freight.canonical()
			if err != nil {
				return err
			}
			contract = binding.Freight
			break
		}
	}
	if contract == nil {
		return nil
	}
	if len(bindings) != len(contract.Allocation.Items) {
		return ErrConflict
	}
	seen := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		body, err := binding.Freight.canonical()
		if err != nil || string(body) != string(canonical) || seen[binding.QuoteOperationID] {
			return ErrConflict
		}
		seen[binding.QuoteOperationID] = true
	}
	for _, item := range contract.Allocation.Items {
		if !seen[item.ItemID] {
			return ErrConflict
		}
	}
	return nil
}

// FreightQuoteCents validates explicit allocation terms without recalculating tax.
func FreightQuoteCents(q TaxQuote) (int64, error) { return saleFreightCents(q) }

// ValidateSaleFreightOrigin binds the before-payment contract to the immutable
// checkout/sale identity. Read-only catalogue previews do not assign a sale.
func ValidateSaleFreightOrigin(bindings []SaleTaxBinding, saleID string) error {
	if err := ValidateSaleFreightBindings(bindings); err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.Freight != nil && binding.Freight.SaleID != saleID {
			return ErrConflict
		}
	}
	return nil
}
