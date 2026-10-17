package sale

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// QuoteInput carries no prices, payment approval, tenant or identity claims.
type QuoteInput struct {
	CashSessionID string `json:"cash_session_id"`
	Items         []Item `json:"items"`
}

type QuoteItem struct {
	Item
	Unit           string `json:"unit"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	TotalCents     int64  `json:"total_cents"`
	AvailableMilli int64  `json:"available_milli"`
}

// Quote is a current snapshot, NOT a reservation, price lock or payment receipt.
// Unsupported concessions are explicit; no browser approval is trusted.
type Quote struct {
	CashSessionID      string      `json:"cash_session_id"`
	Items              []QuoteItem `json:"items"`
	TotalCents         int64       `json:"total_cents"`
	PaymentMethods     []string    `json:"payment_methods"`
	DiscountsSupported bool        `json:"discounts_supported"`
	LoyaltySupported   bool        `json:"loyalty_supported"`
	Reserved           bool        `json:"reserved"`
}

// QuoteWithContract checks the same scope, entitlement, shift, prices, rounding
// and cumulative stock requirements as CompleteWithContract. Only the contract's
// observed clock can change. No sale, payment, stock, cash or event is created.
func QuoteWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in QuoteInput) (Quote, error) {
	if db == nil || !validQuote(in) {
		return Quote{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Quote{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, actor, device, identity.Sell, modules.POS); err != nil {
		return Quote{}, err
	}
	if err = checkShiftTx(ctx, tx, actor, device, in.CashSessionID); err != nil {
		return Quote{}, err
	}
	items, total, err := priceItemsTx(ctx, tx, actor, in.Items)
	if err != nil {
		return Quote{}, err
	}
	// The web client uses exact safe integers. Refuse an unrepresentable snapshot.
	const maxSafe int64 = 9007199254740991
	if total > maxSafe {
		return Quote{}, ErrInvalid
	}
	for _, item := range items {
		if item.QuantityMilli > maxSafe || item.UnitPriceCents > maxSafe || item.AvailableMilli > maxSafe {
			return Quote{}, ErrInvalid
		}
	}
	if err = tx.Commit(); err != nil {
		return Quote{}, err
	}
	return Quote{CashSessionID: in.CashSessionID, Items: items, TotalCents: total, PaymentMethods: []string{"cash"}}, nil
}

func validQuote(in QuoteInput) bool {
	if strings.TrimSpace(in.CashSessionID) == "" || len(in.CashSessionID) > 128 || len(in.Items) == 0 || len(in.Items) > 500 {
		return false
	}
	for _, item := range in.Items {
		if strings.TrimSpace(item.ProductID) == "" || len(item.ProductID) > 128 || strings.TrimSpace(item.LocationID) == "" || len(item.LocationID) > 128 || item.QuantityMilli <= 0 {
			return false
		}
	}
	return true
}

func checkShiftTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, sessionID string) error {
	var operator, sessionDevice string
	err := tx.QueryRowContext(ctx, `SELECT b.identity_id, c.device_id FROM cash_sessions c
		JOIN cash_session_operators b ON b.tenant_id=c.tenant_id AND b.session_id=c.id
		WHERE c.tenant_id=? AND c.store_id=? AND c.id=? AND c.closed_at IS NULL`, actor.TenantID, actor.StoreID, sessionID).Scan(&operator, &sessionDevice)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCashClosed
	}
	if err != nil {
		return err
	}
	if operator != actor.IdentityID || sessionDevice != device.DeviceID {
		return identity.ErrDenied
	}
	return nil
}

func priceItemsTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, items []Item) ([]QuoteItem, int64, error) {
	priced := make([]QuoteItem, 0, len(items))
	consumed := make(map[string]int64)
	var total int64
	for _, item := range items {
		var price int64
		var unit, kind string
		err := tx.QueryRowContext(ctx, `SELECT p.price_cents,p.unit,l.kind FROM products p
			JOIN stock_locations l ON l.tenant_id=p.tenant_id
			WHERE p.tenant_id=? AND p.id=? AND l.store_id=? AND l.id=?`, actor.TenantID, item.ProductID, actor.StoreID, item.LocationID).Scan(&price, &unit, &kind)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, ErrInvalid
		}
		if err != nil {
			return nil, 0, err
		}
		if kind != "shelf" || (unit == "unit" && item.QuantityMilli%1000 != 0) {
			return nil, 0, ErrInvalid
		}
		line, err := roundedCents(price, item.QuantityMilli)
		if err != nil || line > math.MaxInt64-total {
			return nil, 0, ErrInvalid
		}
		total += line
		key := item.ProductID + "\x00" + item.LocationID
		if consumed[key] > math.MaxInt64-item.QuantityMilli {
			return nil, 0, ErrInvalid
		}
		consumed[key] += item.QuantityMilli
		var balance int64
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements
			WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, actor.TenantID, actor.StoreID, item.ProductID, item.LocationID).Scan(&balance); err != nil {
			return nil, 0, err
		}
		if balance < consumed[key] {
			return nil, 0, ErrNoStock
		}
		priced = append(priced, QuoteItem{Item: item, Unit: unit, UnitPriceCents: price, TotalCents: line, AvailableMilli: balance})
	}
	if total <= 0 {
		return nil, 0, ErrInvalid
	}
	return priced, total, nil
}
