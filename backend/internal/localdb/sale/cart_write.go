package sale

import (
	"context"
	"database/sql"
	"errors"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// SaveCart persists a versioned current snapshot, without reserving any stock.
func SaveCart(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in CartSaveInput) (CartResult, error) {
	if !cartKey(in.OperationID) || !cartKey(in.CartID) || in.ExpectedRevision < 0 || in.ExpectedRevision > 4096 || !cartText(in.Label, 160, false) || !validQuote(QuoteInput{CashSessionID: "validation", Items: in.Items}) {
		return CartResult{}, ErrInvalid
	}
	tx, e := cartBegin(ctx, db, c, a, d, true)
	if e != nil {
		return CartResult{}, e
	}
	defer tx.Rollback()
	h := planHash(in)
	if r, ok, e := cartReplay(ctx, tx, a, d, in.OperationID, h); ok || e != nil {
		return r, e
	}
	v, e := cartGetTx(ctx, tx, a, d, in.CartID)
	if errors.Is(e, ErrNotFound) {
		// A private cart belonging to another actor cannot be adopted by ID.
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM pos_carts WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, in.CartID).Scan(&count); e != nil {
			return CartResult{}, e
		}
		if count != 0 || in.ExpectedRevision != 0 {
			return CartResult{}, ErrConflict
		}
		saleID, e := localdb.NewID()
		if e != nil {
			return CartResult{}, e
		}
		paymentID, e := localdb.NewID()
		if e != nil {
			return CartResult{}, e
		}
		v = Cart{ID: in.CartID, State: "suspended", SaleID: saleID, PaymentID: paymentID, CreatedAt: cartNow()}
	} else if e != nil {
		return CartResult{}, e
	}
	if v.Revision >= 4096 {
		return CartResult{}, ErrCartLimit
	}
	if v.State != "suspended" || v.Revision != in.ExpectedRevision {
		return CartResult{}, ErrConflict
	}
	items, total, e := priceItemsTx(ctx, tx, a, in.Items)
	if e != nil {
		return CartResult{}, e
	}
	if total > checkoutMaxSafe {
		return CartResult{}, ErrInvalid
	}
	for _, i := range items {
		if i.QuantityMilli > checkoutMaxSafe || i.UnitPriceCents > checkoutMaxSafe || i.AvailableMilli > checkoutMaxSafe {
			return CartResult{}, ErrInvalid
		}
	}
	v.Items = items
	v.TotalCents = total
	v.Label = in.Label
	v.Revision++
	v.UpdatedAt = cartNow()
	return cartFinish(ctx, tx, a, d, CartResult{OperationID: in.OperationID, Action: "save", Reason: "", Cart: v}, h)
}
func ActCart(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in CartActionInput) (CartResult, error) {
	if !cartKey(id) || !cartKey(in.OperationID) || in.ExpectedRevision < 1 || in.ExpectedRevision > 4096 || !cartText(in.Reason, 255, false) || in.ReceivedCents < 0 || in.ReceivedCents > checkoutMaxSafe {
		return CartResult{}, ErrInvalid
	}
	if in.Action != "claim" && in.Action != "release" && in.Action != "cancel" && in.Action != "finalize_cash" {
		return CartResult{}, ErrInvalid
	}
	if (in.Action == "claim" && !cartKey(in.CashSessionID)) || (in.Action != "claim" && in.CashSessionID != "") || (in.Action != "finalize_cash" && in.ReceivedCents != 0) {
		return CartResult{}, ErrInvalid
	}
	tx, e := cartBegin(ctx, db, c, a, d, true)
	if e != nil {
		return CartResult{}, e
	}
	defer tx.Rollback()
	h := planHash(struct {
		ID    string
		Input CartActionInput
	}{id, in})
	if r, ok, e := cartReplay(ctx, tx, a, d, in.OperationID, h); ok || e != nil {
		return r, e
	}
	v, e := cartGetTx(ctx, tx, a, d, id)
	if e != nil {
		return CartResult{}, e
	}
	if v.Revision != in.ExpectedRevision {
		return CartResult{}, ErrConflict
	}
	if v.Revision >= 4096 {
		return CartResult{}, ErrCartLimit
	}
	switch in.Action {
	case "claim":
		if v.State != "suspended" {
			return CartResult{}, ErrConflict
		}
		if e = checkShiftTx(ctx, tx, a, d, in.CashSessionID); e != nil {
			return CartResult{}, e
		}
		items, total, e := priceItemsTx(ctx, tx, a, cartItems(v))
		if e != nil {
			return CartResult{}, e
		}
		if total != v.TotalCents || !samePlanPrices(items, v.Items) {
			return CartResult{}, ErrConflict
		}
		v.CashSessionID = in.CashSessionID
		v.State = "claimed"
	case "release":
		if v.State != "claimed" {
			return CartResult{}, ErrConflict
		}
		v.State = "suspended"
		v.CashSessionID = ""
	case "cancel":
		if v.State != "suspended" {
			return CartResult{}, ErrConflict
		}
		v.State = "cancelled"
	case "finalize_cash":
		if v.State != "claimed" || in.ReceivedCents < v.TotalCents {
			return CartResult{}, ErrConflict
		}
		if e = checkShiftTx(ctx, tx, a, d, v.CashSessionID); e != nil {
			return CartResult{}, e
		}
		items, total, e := priceItemsTx(ctx, tx, a, cartItems(v))
		if e != nil {
			return CartResult{}, e
		}
		if total != v.TotalCents || !samePlanPrices(v.Items, items) {
			return CartResult{}, ErrConflict
		}
		// Stable sale/payment IDs and all legacy journal/stock/outbox effects commit
		// with the cart event. There is no intermediate payment approval state.
		_, e = completeTx(ctx, tx, a, d, Input{OperationID: in.OperationID, SaleID: v.SaleID, CashSessionID: v.CashSessionID, Items: cartItems(v), Payments: []Payment{{Method: "cash", AmountCents: v.TotalCents}}}, func(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) error {
			return c.RequireTx(ctx, tx, a, d, identity.Sell, modules.POS)
		}, []string{v.PaymentID})
		if e != nil {
			return CartResult{}, e
		}
		v.State = "completed"
		v.SaleOperationID = in.OperationID
		v.ReceivedCents = in.ReceivedCents
		v.ChangeCents = in.ReceivedCents - v.TotalCents
	}
	v.Revision++
	v.UpdatedAt = cartNow()
	return cartFinish(ctx, tx, a, d, CartResult{OperationID: in.OperationID, Action: in.Action, Reason: in.Reason, Cart: v}, h)
}
func cartItems(v Cart) []Item {
	out := make([]Item, len(v.Items))
	for i, it := range v.Items {
		out[i] = it.Item
	}
	return out
}
