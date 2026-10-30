package fiscal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

type CheckoutStatus struct {
	State       string `json:"state"`
	IntentID    string `json:"intent_id,omitempty"`
	OperationID string `json:"operation_id"`
	ErrorCode   string `json:"error_code,omitempty"`
	Key         string `json:"key,omitempty"`
}

// RecordCheckoutStatus appends an audit/status event to the existing durable
// outbox. It does not retry emission, replace a sale or modify the fiscal journal.
func RecordCheckoutStatus(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, sale string, state CheckoutStatus) error {
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'checkout.fiscal_status',1,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, "checkout-fiscal-"+id, sale, string(body), time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z07:00"))
	return err
}

// CheckoutHomologationInvoice maps only frozen sale/tax data. The caller supplies
// descriptive issuer/address fields, never authoritative prices or tax values.
func CheckoutHomologationInvoice(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id, token string, n NumberAssignment, in fiscalcore.Invoice) (fiscalcore.Invoice, error) {
	if in.Environment != 2 || in.Key != (fiscalcore.KeyInput{}) {
		return fiscalcore.Invoice{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return in, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return in, err
	}
	intent, err := intentTx(ctx, tx, a, id)
	if err != nil {
		return in, err
	}
	p := intent.Snapshot.Profile
	if intent.ActorID != a.IdentityID || intent.DeviceID != d.DeviceID || p.Environment != "homologation" || p.Model != "65" {
		return in, ErrInvalid
	}
	issued, err := time.Parse(time.RFC3339Nano, n.IssuedAt)
	if err != nil || !fiscalcore.ValidAccessKey(n.AccessKey) || n.IntentID != id {
		return in, ErrInvalid
	}
	in.Key = fiscalcore.KeyInput{UF: p.State, IssuedAt: issued, CNPJ: p.CNPJ, Model: p.Model, Series: p.Series, Number: n.Number, Emission: 1, NumericCode: n.AccessKey[35:43]}
	in.CRT = p.CRT
	in.Direction = "1"
	in.Destination = "1"
	in.FinalConsumer = "1"
	in.Issuer.CNPJ = p.CNPJ
	in.Issuer.Name = p.LegalName
	in.Issuer.IE = p.StateRegistration
	in.Issuer.Address.UF = p.State
	in.Issuer.Address.CityCode = p.Municipality
	in.Lines = nil
	in.Payments = nil
	in.TaxTotals = nil
	for _, item := range intent.Snapshot.Items {
		if item.Tax == nil {
			return in, ErrInvalid
		}
		var name string
		if err = tx.QueryRowContext(ctx, `SELECT name FROM products WHERE tenant_id=? AND id=?`, a.TenantID, item.ProductID).Scan(&name); err != nil {
			return in, err
		}
		in.Lines = append(in.Lines, fiscalcore.InvoiceLine{ProductID: item.ProductID, Description: name, Unit: item.Tax.CatalogUnit, Quantity: fmt.Sprintf("%d.%03d", item.QuantityMilli/1000, item.QuantityMilli%1000), UnitPrice: fmt.Sprintf("%d.%02d", item.UnitPriceCents/100, item.UnitPriceCents%100), Discount: "0", Freight: "0", Insurance: "0", Other: "0", TaxQuoteOperationID: item.Tax.QuoteOperationID})
	}
	for _, payment := range intent.Snapshot.Payments {
		if payment.Method != "cash" || payment.Status != "confirmed" {
			return in, ErrInvalid
		}
		in.Payments = append(in.Payments, fiscalcore.InvoicePayment{Method: "01", Amount: fmt.Sprintf("%d.%02d", payment.AmountCents/100, payment.AmountCents%100)})
	}
	if err = applyProductionTaxQuotesTx(ctx, tx, a, d, &in, p.ProfileID, intent.Input.ProfileRevision); err != nil {
		return in, err
	}
	if err = validateInvoiceSnapshot(in, intent.Snapshot, n.AccessKey, 2); err != nil {
		return in, err
	}
	return in, tx.Commit()
}
