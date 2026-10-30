package fiscal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type CheckoutStatus struct {
	CanReprocess bool   `json:"can_reprocess"`
	State        string `json:"state"`
	IntentID     string `json:"intent_id,omitempty"`
	OperationID  string `json:"operation_id"`
	ErrorCode    string `json:"error_code,omitempty"`
	Key          string `json:"key,omitempty"`
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

// Candidate requests are content-addressed before committing the sale. Only the
// request_hash frozen by checkout_plan_sales can ever be recovered. A rejected
// finalization therefore cannot replace/poison the committed original request.
func checkoutRequestOperation(a identity.Scope, d identity.DeviceContext, sale, hash string) string {
	sum := sha256.Sum256([]byte(a.TenantID + "/" + a.StoreID + "/" + d.DeviceID + "/" + sale + "/" + hash))
	return "checkout-request-" + hex.EncodeToString(sum[:])
}
func SaveCheckoutRequest(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, sale, hash string, body []byte, token string) error {
	if !idPattern.MatchString(sale) || len(hash) != 64 || len(body) == 0 || len(body) > 128*1024 || !json.Valid(body) || license == nil {
		return ErrInvalid
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.Sell, modules.Fiscal); err != nil {
		return err
	}
	var id string
	if err = tx.QueryRowContext(ctx, `SELECT checkout_id FROM checkout_plans WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_identity_id=? AND checkout_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, sale).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	var committed string
	err = tx.QueryRowContext(ctx, `SELECT request_hash FROM checkout_plan_sales WHERE tenant_id=? AND checkout_id=?`, a.TenantID, sale).Scan(&committed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && committed != hash {
		return ErrConflict
	}
	op := checkoutRequestOperation(a, d, sale, hash)
	var old []byte
	err = tx.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND operation_id=? AND event_type='checkout.fiscal_request'`, a.TenantID, a.StoreID, d.DeviceID, sale, op).Scan(&old)
	if err == nil {
		if !bytes.Equal(old, body) {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	event, err := localdb.NewID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'checkout.fiscal_request',1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, op, sale, string(body), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ReadCheckoutRequest is server-only: never attach the original invoice to the
// public checkout response. It retains the original actor/device restrictions.
func ReadCheckoutRequest(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, sale, token string) ([]byte, error) {
	if !idPattern.MatchString(sale) || license == nil {
		return nil, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return nil, err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.Sell, modules.Fiscal); err != nil {
		return nil, err
	}
	var hash string
	err = tx.QueryRowContext(ctx, `SELECT f.request_hash FROM checkout_plan_sales f JOIN checkout_plans p ON p.tenant_id=f.tenant_id AND p.checkout_id=f.checkout_id JOIN sales s ON s.tenant_id=f.tenant_id AND s.id=f.sale_id WHERE f.tenant_id=? AND f.store_id=? AND f.device_id=? AND p.actor_identity_id=? AND f.checkout_id=? AND s.status='committed'`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, sale).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var body []byte
	err = tx.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND operation_id=? AND event_type='checkout.fiscal_request'`, a.TenantID, a.StoreID, d.DeviceID, sale, checkoutRequestOperation(a, d, sale, hash)).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || len(body) > 128*1024 || !json.Valid(body) {
		return nil, ErrInvalid
	}
	return body, tx.Commit()
}
