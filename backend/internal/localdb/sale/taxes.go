package sale

import (
	"context"
	"database/sql"
	"errors"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
)

func priceTaxesTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, refs []string, items []QuoteItem, total int64, commercialID string, freightSelected bool) ([]*fiscal.SaleTaxBinding, int64, error) {
	if len(refs) == 0 {
		if freightSelected {
			return nil, 0, ErrInvalid
		}
		return nil, total, nil
	}
	if len(refs) != len(items) || len(refs) > 100 {
		return nil, 0, ErrInvalid
	}
	bindings := make([]*fiscal.SaleTaxBinding, len(items))
	seen := map[string]bool{}
	for i, ref := range refs {
		if seen[ref] {
			return nil, 0, ErrInvalid
		}
		seen[ref] = true
		item := items[i]
		binding, err := fiscal.ResolveSaleTaxTx(ctx, tx, a, d, ref, item.ProductID, item.Unit, item.QuantityMilli, item.UnitPriceCents, item.TotalCents)
		if err != nil {
			if errors.Is(err, fiscal.ErrInvalid) {
				return nil, 0, ErrInvalid
			}
			return nil, 0, ErrConflict
		}
		if i > 0 {
			left, right := binding.Case, bindings[0].Case
			left.CEST = ""
			right.CEST = ""
			left.ProductOrigin = ""
			right.ProductOrigin = ""
			if left != right || binding.Context != bindings[0].Context {
				return nil, 0, ErrConflict
			}
		}
		if i > 0 && (binding.ProfileID != bindings[0].ProfileID || binding.ProfileRevision != bindings[0].ProfileRevision) {
			return nil, 0, ErrConflict
		}
		if binding.AdditionalCents > checkoutMaxSafe-total {
			return nil, 0, ErrInvalid
		}
		total += binding.AdditionalCents
		bindings[i] = &binding
	}
	all := make([]fiscal.SaleTaxBinding, len(bindings))
	for i, b := range bindings {
		all[i] = *b
	}
	hasFreight := false
	for _, b := range all {
		hasFreight = hasFreight || b.Freight != nil
	}
	if hasFreight != freightSelected {
		return nil, 0, ErrInvalid
	}
	if err := fiscal.ValidateSaleFreightBindings(all); err != nil {
		return nil, 0, ErrConflict
	}
	if commercialID != "" {
		if err := fiscal.ValidateSaleFreightOrigin(all, commercialID); err != nil {
			return nil, 0, ErrConflict
		}
	}
	return bindings, total, nil
}
