package fiscal

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/big"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/identity"
)

// SaleTaxBinding is frozen with the payment, not added after a sale completes.
type SaleTaxBinding struct {
	CatalogUnit      string                `json:"catalog_unit"`
	Context          fiscalcore.TaxContext `json:"context"`
	Case             fiscalcore.TaxCase    `json:"case"`
	QuoteOperationID string                `json:"quote_operation_id"`
	ResultSHA256     string                `json:"result_sha256"`
	ProfileID        string                `json:"profile_id"`
	ProfileRevision  int                   `json:"profile_revision"`
	AdditionalCents  int64                 `json:"additional_cents"`
	Taxes            []fiscalcore.TaxNode  `json:"taxes"`
	Totals           map[string]string     `json:"totals"`
}

func taxCents(s string) (int64, error) {
	n, ok := new(big.Rat).SetString(s)
	if !ok || !decimalPattern.MatchString(s) {
		return 0, ErrInvalid
	}
	n.Mul(n, big.NewRat(100, 1))
	if !n.IsInt() || !n.Num().IsInt64() || n.Sign() < 0 || n.Num().Int64() > 9007199254740991 {
		return 0, ErrInvalid
	}
	return n.Num().Int64(), nil
}
func saleTaxBinding(q TaxQuote, p TaxPolicy) (SaleTaxBinding, error) {
	var out SaleTaxBinding
	mapped := q.Result.GoodsXML
	if mapped == nil || mapped.Version != 1 {
		return out, ErrInvalid
	}
	net, e1 := taxCents(q.Result.Net)
	total, e2 := taxCents(q.Result.Total)
	if e1 != nil || e2 != nil || total < net {
		return out, ErrInvalid
	}
	extra := int64(0)
	for _, name := range []string{"vST", "vFCPST", "vIPI"} {
		n, err := taxCents(mapped.Totals[name])
		if err != nil || n > 9007199254740991-extra {
			return out, ErrInvalid
		}
		extra += n
	}
	if extra != total-net {
		return out, ErrInvalid
	}
	return SaleTaxBinding{CatalogUnit: q.CatalogUnit, Context: q.Input.Context, Case: q.Input.Case, QuoteOperationID: q.Input.OperationID, ResultSHA256: q.ResultSHA256, ProfileID: p.Input.ProfileID, ProfileRevision: p.Input.ProfileRevision, AdditionalCents: extra, Taxes: mapped.Taxes, Totals: mapped.Totals}, nil
}

// ResolveSaleTaxTx checks current policy/profile/date, exact catalogue price and
// quantity. Callers enforce POS+Fiscal license and a current session in this Tx.
func ResolveSaleTaxTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, operation, product, unit string, quantity, price, total int64) (SaleTaxBinding, error) {
	var out SaleTaxBinding
	if !idPattern.MatchString(operation) {
		return out, ErrInvalid
	}
	q, err := taxQuoteTx(ctx, tx, a, d, operation)
	if err != nil {
		return out, err
	}
	p, err := taxPolicyTx(ctx, tx, a, q.Input.PolicyID, 0)
	if err != nil {
		return out, err
	}
	profile, err := profileTx(ctx, tx, a, p.Input.ProfileID, 0)
	if err != nil {
		return out, err
	}
	if !p.Input.Active || p.Revision != q.Input.PolicyRevision || p.SHA256 != q.PolicySHA256 || profile.Revision != p.Input.ProfileRevision || profile.Input.Timezone == "" {
		return out, ErrConflict
	}
	zone, err := time.LoadLocation(profile.Input.Timezone)
	if err != nil {
		return out, ErrInvalid
	}
	now := time.Now()
	created, err := time.Parse(time.RFC3339Nano, q.CreatedAt)
	if err != nil || created.After(now) || now.Sub(created) > 10*time.Minute || q.Input.Context.Date != now.In(zone).Format("2006-01-02") {
		return out, ErrConflict
	}
	item := q.Input.Item
	if q.CatalogUnit == "" || q.CatalogUnit != unit || item.ProductID != product || !equalScaled(item.Quantity, quantity, 1000) || !equalScaled(item.UnitPrice, price, 100) || !equalScaled(q.Result.Net, total, 100) {
		return out, ErrConflict
	}
	for _, v := range []string{item.Discount, item.Freight, item.Insurance, item.Other} {
		if !equalScaled(v, 0, 1) {
			return out, ErrInvalid
		}
	}
	return saleTaxBinding(q, p)
}
func ValidateSaleSessionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, token string) error {
	return fiscalSessionTx(ctx, tx, a, d, token)
}

// StoredSaleTaxesTx verifies historical evidence without requiring an active
// policy, new quote, license or session from the original sale.
func StoredSaleTaxesTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, sale string) (map[string]SaleTaxBinding, error) {
	rows, err := tx.QueryContext(ctx, `SELECT item_id,device_id,quote_operation_id,binding_json FROM sale_tax_bindings WHERE tenant_id=? AND store_id=? AND sale_id=?`, a.TenantID, a.StoreID, sale)
	if err != nil {
		return nil, err
	}
	type record struct{ item, device, op, body string }
	records := []record{}
	for rows.Next() {
		var r record
		if err = rows.Scan(&r.item, &r.device, &r.op, &r.body); err != nil {
			rows.Close()
			return nil, err
		}
		records = append(records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := map[string]SaleTaxBinding{}
	for _, r := range records {
		original := d
		original.DeviceID = r.device
		q, err := taxQuoteTx(ctx, tx, a, original, r.op)
		if err != nil {
			return nil, err
		}
		p, err := taxPolicyTx(ctx, tx, a, q.Input.PolicyID, q.Input.PolicyRevision)
		if err != nil {
			return nil, err
		}
		expected, err := saleTaxBinding(q, p)
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(expected)
		if string(raw) != r.body {
			return nil, ErrIntegrity
		}
		var product string
		var quantity, price, total int64
		if err = tx.QueryRowContext(ctx, `SELECT product_id,quantity_milli,unit_price_cents,total_cents FROM sale_items WHERE tenant_id=? AND sale_id=? AND id=?`, a.TenantID, sale, r.item).Scan(&product, &quantity, &price, &total); err != nil {
			return nil, err
		}
		if q.Input.Item.ProductID != product || !equalScaled(q.Input.Item.Quantity, quantity, 1000) || !equalScaled(q.Input.Item.UnitPrice, price, 100) || !equalScaled(q.Result.Net, total, 100) {
			return nil, ErrIntegrity
		}
		out[r.item] = expected
	}
	return out, nil
}

// TaxPricingRequiredTx reads the active owner-configured profile revisions. It
// does not reinterpret a draft policy or retroactively change a completed sale.
func TaxPricingRequiredTx(ctx context.Context, tx *sql.Tx, a identity.Scope) (bool, error) {
	var required bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM fiscal_profiles p WHERE p.tenant_id=? AND p.store_id=? AND json_extract(p.request_json,'$.require_tax_pricing')=1 AND NOT EXISTS(SELECT 1 FROM fiscal_profiles newer WHERE newer.tenant_id=p.tenant_id AND newer.store_id=p.store_id AND newer.profile_id=p.profile_id AND newer.revision>p.revision))`, a.TenantID, a.StoreID).Scan(&required)
	return required, err
}
