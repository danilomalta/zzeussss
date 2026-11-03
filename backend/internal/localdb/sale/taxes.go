package sale

import (
	"context"
	"database/sql"
	"errors"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
)

func priceTaxesTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, refs []string, items []QuoteItem, total int64) ([]*fiscal.SaleTaxBinding, int64, error) {
	if len(refs) == 0 {
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
	return bindings, total, nil
}
