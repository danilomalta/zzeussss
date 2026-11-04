package fiscalproduction

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/localdb/fiscal"
)

// Customer NFe follows the owner-only fiscal-registration gate, not POS permissions alone.
func (r *ProviderRegistry) CustomerNFe55Provider(ctx context.Context, a RequestContext) (FiscalProvider, ProviderBinding, error) {
	return r.customerFocusProvider(ctx, a, "55")
}
func (r *ProviderRegistry) CustomerDeferredNFCeProvider(ctx context.Context, a RequestContext) (FiscalProvider, ProviderBinding, error) {
	return r.customerFocusProvider(ctx, a, "65")
}
func (r *ProviderRegistry) customerFocusProvider(ctx context.Context, a RequestContext, model string) (FiscalProvider, ProviderBinding, error) {
	if err := authorizeCustomerCredentials(ctx, a); err != nil {
		return nil, ProviderBinding{}, err
	}
	provider, b, err := r.Resolve(a)
	if err != nil {
		return nil, b, err
	}
	c := r.entries[a.Actor.TenantID].Configuration
	if focus, ok := provider.(*FocusProvider); ok {
		c = focus.configuration
	}
	if c.Environment != "homologation" {
		return nil, b, ErrCheckoutProduction
	}
	if c.CertificateType == "A3" {
		return nil, b, ErrFocusA3
	}
	if c.Provider != "external" || (c.DocumentModel != model && !(model == "65" && c.DocumentModel == "")) {
		return nil, b, ErrUnavailable
	}
	p, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, b.ProfileID, 0)
	if err != nil {
		return nil, b, err
	}
	if p.Revision != b.ProfileRevision || p.Input.Model != model || p.Input.Environment != "homologation" || p.Input.CRT != c.CRT {
		return nil, b, fiscal.ErrConflict
	}
	return provider, b, nil
}

// PrepareCustomerDeferredNFe55 records an explicit commercial payment method,
// never a receipt. The existing selection transaction validates the AR origin,
// session and owner; its stable operation survives a lost response/client reset.
func (r *ProviderRegistry) PrepareCustomerDeferredNFe55(ctx context.Context, a RequestContext, sale, code string) error {
	provider, _, err := r.CustomerNFe55Provider(ctx, a)
	if err != nil {
		return err
	}
	// ValidateCheckout is intentionally NFC-e-only. Keep the model-55 gate
	// separate; do not relax that existing checkout boundary.
	if focus, ok := provider.(*FocusProvider); ok {
		if err = focus.validateModel(ctx, a, 2, "55"); err != nil {
			return err
		}
	}
	return fiscal.SelectDeferredPayment(ctx, a.DB, a.License, a.Actor, a.Device, a.Token, sale, "nfe55-"+focusReference(a.Actor.TenantID, sale), code)
}

type CustomerNFePreview struct {
	DeferredPaymentCode string                `json:"deferred_payment_code,omitempty"`
	AccountantApproved  bool                  `json:"accountant_approved"`
	SaleID              string                `json:"sale_id"`
	IntentID            string                `json:"intent_id"`
	TotalCents          int64                 `json:"total_cents"`
	Items               []fiscal.SaleItem     `json:"items"`
	MappingSupported    bool                  `json:"mapping_supported"`
	Status              fiscal.CheckoutStatus `json:"status"`
}

func (r *ProviderRegistry) PreviewCustomerNFe55(ctx context.Context, a RequestContext, sale string) (CustomerNFePreview, error) {
	_, b, err := r.CustomerNFe55Provider(ctx, a)
	if err != nil {
		return CustomerNFePreview{}, err
	}
	intent, err := fiscal.Prepare(ctx, a.DB, a.License, a.Actor, a.Device, fiscal.IntentInput{OperationID: focusReference(a.Actor.TenantID, sale), SaleID: sale, ProfileID: b.ProfileID, ProfileRevision: b.ProfileRevision}, a.Token)
	if err != nil {
		return CustomerNFePreview{}, err
	}
	out := CustomerNFePreview{SaleID: sale, IntentID: intent.ID, TotalCents: intent.Snapshot.TotalCents, Items: intent.Snapshot.Items, MappingSupported: true, Status: fiscal.CheckoutStatus{State: "not_requested"}}
	if intent.Snapshot.DeferredPayment != nil {
		out.DeferredPaymentCode = intent.Snapshot.DeferredPayment.Code
	}
	for _, item := range out.Items {
		if item.Tax == nil || focusTaxFields(map[string]any{}, item.Tax.Taxes) != nil {
			out.MappingSupported = false
		}
	}
	out.AccountantApproved = accountantApprovedIntent(ctx, a, intent)
	var raw string
	// No original recipient request is returned, saved to a client, or logged.
	err = a.DB.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND event_type='checkout.fiscal_status' ORDER BY created_at DESC,event_id DESC LIMIT 1`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, sale).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return CustomerNFePreview{}, ErrUnavailable
	}
	if err == nil {
		if json.Unmarshal([]byte(raw), &out.Status) != nil {
			return CustomerNFePreview{}, ErrUnavailable
		}
	}
	return out, nil
}

// Approval follows the frozen policy revision, never a newer publication.
// Missing legacy evidence is unapproved; an owner's publication alone is not an
// accountant's approval. No rule is recalculated during emission or recovery.
func accountantApprovedIntent(ctx context.Context, a RequestContext, intent fiscal.Intent) bool {
	if len(intent.Snapshot.Items) == 0 {
		return false
	}
	for _, item := range intent.Snapshot.Items {
		if item.Tax == nil {
			return false
		}
		q, e := fiscal.ReadTaxQuote(ctx, a.DB, a.Actor, a.Device, item.Tax.QuoteOperationID)
		if e != nil {
			return false
		}
		policy, e := fiscal.ReadTaxPolicy(ctx, a.DB, a.Actor, a.Device, q.Input.PolicyID, q.Input.PolicyRevision)
		if e != nil || !policy.Input.AccountantApproved() {
			return false
		}
	}
	return true
}
