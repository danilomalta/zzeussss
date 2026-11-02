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
	if c.Provider != "external" || c.DocumentModel != "55" {
		return nil, b, ErrUnavailable
	}
	p, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, b.ProfileID, 0)
	if err != nil {
		return nil, b, err
	}
	if p.Revision != b.ProfileRevision || p.Input.Model != "55" || p.Input.Environment != "homologation" || p.Input.CRT != c.CRT {
		return nil, b, fiscal.ErrConflict
	}
	return provider, b, nil
}

type CustomerNFePreview struct {
	SaleID           string                `json:"sale_id"`
	IntentID         string                `json:"intent_id"`
	TotalCents       int64                 `json:"total_cents"`
	Items            []fiscal.SaleItem     `json:"items"`
	MappingSupported bool                  `json:"mapping_supported"`
	Status           fiscal.CheckoutStatus `json:"status"`
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
	for _, item := range out.Items {
		if item.Tax == nil || focusTaxFields(map[string]any{}, item.Tax.Taxes) != nil {
			out.MappingSupported = false
		}
	}
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
