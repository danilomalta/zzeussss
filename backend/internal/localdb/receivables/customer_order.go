package receivables

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/supplycycle"
)

type CustomerOrderInput struct {
	MessageID      string `json:"message_id"`
	TermsReference string `json:"terms_reference"`
	Confirmed      bool   `json:"confirmed"`
}
type CustomerOrderOrigin struct {
	Source         supplycycle.CustomerBillingOrder `json:"source"`
	TermsReference string                           `json:"terms_reference"`
}

func billingOrderError(e error) error {
	switch {
	case errors.Is(e, purchases.ErrInvalid):
		return ErrInvalid
	case errors.Is(e, purchases.ErrNotFound):
		return ErrNotFound
	case errors.Is(e, purchases.ErrConflict):
		return ErrConflict
	default:
		return e
	}
}
func CustomerOrder(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (supplycycle.CustomerBillingOrder, error) {
	var out supplycycle.CustomerBillingOrder
	if db == nil || !validID(id) {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); e != nil {
		return out, e
	}
	out, e = supplycycle.CustomerBillingOrderTx(ctx, tx, a, d, id)
	if e != nil {
		return out, billingOrderError(e)
	}
	return out, tx.Commit()
}
func readCustomerOrderTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (*CustomerOrderOrigin, error) {
	var raw, message, decision, terms, tenant, store, order string
	e := tx.QueryRowContext(ctx, `SELECT source_json,message_id,decision_id,terms_reference,customer_tenant_id,customer_store_id,order_id FROM receivable_customer_orders WHERE tenant_id=? AND store_id=? AND document_id=?`, a.TenantID, a.StoreID, id).Scan(&raw, &message, &decision, &terms, &tenant, &store, &order)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var origin CustomerOrderOrigin
	if json.Unmarshal([]byte(raw), &origin) != nil || origin.Source.MessageID != message || origin.Source.DecisionID != decision || origin.TermsReference != terms || origin.Source.CustomerTenantID != tenant || origin.Source.CustomerStoreID != store || origin.Source.OrderID != order {
		return nil, ErrConflict
	}
	return &origin, nil
}
