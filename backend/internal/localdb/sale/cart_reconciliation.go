package sale

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// CartReconciliation is a current diagnostic. It never updates the saved cart,
// reserves materials, approves a payment or guarantees a later finalization.
type CartReconciliation struct {
	CartID          string   `json:"cart_id"`
	Revision        int64    `json:"revision"`
	State           string   `json:"state"`
	Status          string   `json:"status"`
	Blockers        []string `json:"blockers"`
	CheckedAt       string   `json:"checked_at"`
	SavedTotalCents int64    `json:"saved_total_cents"`
	CurrentQuote    *Quote   `json:"current_quote,omitempty"`
	SaleID          string   `json:"sale_id"`
	Reserved        bool     `json:"reserved"`
}

func ReconcileCart(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, session string) (CartReconciliation, error) {
	out := CartReconciliation{Blockers: []string{}, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if !cartKey(id) || !cartKey(session) {
		return out, ErrInvalid
	}
	tx, e := cartBegin(ctx, db, c, a, d, true)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	v, e := cartGetTx(ctx, tx, a, d, id)
	if e != nil {
		return out, e
	}
	out.CartID = v.ID
	out.Revision = v.Revision
	out.State = v.State
	out.SavedTotalCents = v.TotalCents
	out.SaleID = v.SaleID
	if v.State == "completed" || v.State == "cancelled" {
		out.Status = "terminal"
		return out, tx.Commit()
	}
	out.Status = "blocked"
	if v.State == "claimed" && session != v.CashSessionID {
		out.Blockers = append(out.Blockers, "claimed_in_another_shift")
	}
	if e = checkShiftTx(ctx, tx, a, d, session); e != nil {
		if errors.Is(e, ErrCashClosed) {
			out.Blockers = append(out.Blockers, "cash_closed")
		} else {
			return out, e
		}
	}
	items, total, e := priceItemsTx(ctx, tx, a, cartItems(v))
	if e != nil {
		switch {
		case errors.Is(e, ErrNoStock):
			out.Blockers = append(out.Blockers, "insufficient_stock")
		case errors.Is(e, ErrInvalid):
			out.Blockers = append(out.Blockers, "invalid_catalog_or_quantity")
		default:
			// Catalog invalidation has its own typed error; do not hide storage errors.
			if errors.Is(e, catalog.ErrProductInactive) {
				out.Blockers = append(out.Blockers, "inactive_product")
			} else {
				return out, e
			}
		}
	} else {
		// Browser-facing values remain exact safe integers.
		if total > checkoutMaxSafe {
			return out, ErrInvalid
		}
		for _, i := range items {
			if i.QuantityMilli > checkoutMaxSafe || i.UnitPriceCents > checkoutMaxSafe || i.AvailableMilli > checkoutMaxSafe {
				return out, ErrInvalid
			}
		}
		out.CurrentQuote = &Quote{CashSessionID: session, Items: items, TotalCents: total, PaymentMethods: []string{"cash"}}
		if total != v.TotalCents || !samePlanPrices(v.Items, items) {
			out.Blockers = append(out.Blockers, "saved_prices_or_units_changed")
		}
	}
	if len(out.Blockers) == 0 {
		out.Status = "ready"
	}
	return out, tx.Commit() // Only the entitlement's observed clock may advance.
}
