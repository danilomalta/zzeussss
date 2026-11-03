package supplycycle

import (
	"context"
	"database/sql"
	"errors"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
)

// CustomerBillingOrder proves origin/quantity only. Trade v1 contains no agreed
// price, payment terms, delivery acceptance or permission to charge the buyer.
type CustomerBillingOrder struct {
	CustomerTenantID  string `json:"customer_tenant_id"`
	CustomerStoreID   string `json:"customer_store_id"`
	MessageID         string `json:"message_id"`
	DecisionID        string `json:"decision_id"`
	OrderID           string `json:"order_id"`
	CustomerReference string `json:"customer_reference"`
	ProductID         string `json:"product_id"`
	Unit              string `json:"unit"`
	AcceptedMilli     int64  `json:"accepted_milli"`
	OfferSHA256       string `json:"offer_sha256"`
}

func CustomerBillingOrderTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (CustomerBillingOrder, error) {
	var out CustomerBillingOrder
	if !key(id) {
		return out, purchases.ErrInvalid
	}
	if e := identity.CanOperateTx(ctx, tx, a, d, identity.ViewOrders); e != nil {
		return out, e
	}
	offer, env, peer, direction, e := tradeMessageTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	if direction != "received" || offer.Kind != "order" || peer.Role != "customer" || peer.Status != "active" || offer.Target != tradeParty(d) {
		return out, purchases.ErrConflict
	}
	hash := TradeEnvelopeSHA256(env)
	var decisionID string
	e = tx.QueryRowContext(ctx, `SELECT id FROM company_trade_messages WHERE tenant_id=? AND store_id=? AND direction='sent' AND kind='decision' AND offer_sha256=?`, a.TenantID, a.StoreID, hash).Scan(&decisionID)
	if errors.Is(e, sql.ErrNoRows) {
		return out, purchases.ErrConflict
	}
	if e != nil {
		return out, e
	}
	decision, _, decisionPeer, dir, e := tradeMessageTx(ctx, tx, a, decisionID)
	if e != nil {
		return out, e
	}
	if dir != "sent" || decisionPeer.ID != peer.ID || decision.Kind != "decision" || decision.Decision != "confirmed" || decision.AcceptedMilli <= 0 || decision.AcceptedMilli > offer.QuantityMilli || decision.Source != tradeParty(d) || decision.Target != offer.Source || decision.OrderID != offer.OrderID || decision.ProductID != offer.ProductID || decision.Unit != offer.Unit || decision.OfferSHA256 != hash {
		return out, purchases.ErrConflict
	}
	return CustomerBillingOrder{CustomerTenantID: offer.Source.TenantID, CustomerStoreID: offer.Source.StoreID, MessageID: id, DecisionID: decisionID, OrderID: offer.OrderID, CustomerReference: peer.ID, ProductID: offer.ProductID, Unit: offer.Unit, AcceptedMilli: decision.AcceptedMilli, OfferSHA256: hash}, nil
}
