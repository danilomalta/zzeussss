package fiscal

import (
	"context"
	"database/sql"

	"errors"
	"time"
	"titansystem-backend/internal/core/modules"

	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitewrite"
)

// Official fields: https://campos.focusnfe.com.br/nfe/NotaFiscalXML.html
// This describes an agreed deferred method, NEVER a confirmed payment.
type DeferredPayment struct {
	SelectedBy string `json:"selected_by"`
	Code       string `json:"code"`
	DocumentID string `json:"document_id"`
	TotalCents int64  `json:"total_cents"`
}

func (v DeferredPayment) Valid(total int64) bool {
	return (v.Code == "21" || v.Code == "91") && idPattern.MatchString(v.DocumentID) && idPattern.MatchString(v.SelectedBy) && total > 0 && v.TotalCents == total
}

func deferredSourceTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, sale string) (DeferredPayment, error) {
	var v DeferredPayment
	var total int64
	var status, device, documentState string
	var payments int
	err := tx.QueryRowContext(ctx, `SELECT c.document_id,r.total_cents,s.total_cents,s.status,s.device_id,r.state,(SELECT count(*) FROM sale_payments p WHERE p.tenant_id=s.tenant_id AND p.sale_id=s.id) FROM receivable_credit_sales c JOIN receivable_documents r ON r.tenant_id=c.tenant_id AND r.store_id=c.store_id AND r.id=c.document_id JOIN sales s ON s.tenant_id=c.tenant_id AND s.store_id=c.store_id AND s.id=c.sale_id WHERE c.tenant_id=? AND c.store_id=? AND c.sale_id=?`, a.TenantID, a.StoreID, sale).Scan(&v.DocumentID, &v.TotalCents, &total, &status, &device, &documentState, &payments)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	if documentState == "cancelled" || status != "committed" || device != d.DeviceID || payments != 0 || total <= 0 || total != v.TotalCents {
		return v, ErrConflict
	}
	var installments int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(amount_cents),0) FROM receivable_installments WHERE tenant_id=? AND store_id=? AND document_id=?`, a.TenantID, a.StoreID, v.DocumentID).Scan(&installments); err != nil {
		return v, err
	}
	if installments != total {
		return v, ErrIntegrity
	}
	return v, nil
}

// SelectDeferredPayment freezes the operator's explicit choice. No amounts,
// customer data, cash or debt are accepted from the caller or changed here.
func SelectDeferredPayment(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, sale, operation, code string) error {
	if db == nil || c == nil || !idPattern.MatchString(sale) || !idPattern.MatchString(operation) || (code != "21" && code != "91") {
		return ErrInvalid
	}
	return sqlitewrite.Run(ctx, db, ErrConflict, func(tx *sql.Tx) error {
		if err := fiscalSessionTx(ctx, tx, a, d, token); err != nil {
			return err
		}
		if err := c.RequireTx(ctx, tx, a, d, identity.Sell, modules.Fiscal); err != nil {
			return err
		}
		var role string
		if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
			return err
		}
		if role != "owner" {
			return identity.ErrDenied
		}
		v, err := deferredSourceTx(ctx, tx, a, d, sale)
		if err != nil {
			return err
		}
		v.Code = code
		v.SelectedBy = a.IdentityID

		var old DeferredPayment
		var operationID string
		err = tx.QueryRowContext(ctx, `SELECT document_id,payment_code,total_cents,actor_id,operation_id FROM fiscal_deferred_payments WHERE tenant_id=? AND store_id=? AND device_id=? AND sale_id=?`, a.TenantID, a.StoreID, d.DeviceID, sale).Scan(&old.DocumentID, &old.Code, &old.TotalCents, &old.SelectedBy, &operationID)
		if err == nil {
			if old != v || operationID != operation {
				return ErrConflict
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var used int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_intents WHERE tenant_id=? AND store_id=? AND sale_id=?`, a.TenantID, a.StoreID, sale).Scan(&used); err != nil {
			return err
		}
		if used != 0 {
			return ErrConflict
		}
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_deferred_payments WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, operation).Scan(&used); err != nil {
			return err
		}
		if used != 0 {
			return ErrConflict
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_deferred_payments VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, sale, operation, a.IdentityID, v.DocumentID, v.Code, v.TotalCents, time.Now().UTC().Format(time.RFC3339Nano))
		return err

	})
}
func deferredPaymentTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, sale string, total int64) (*DeferredPayment, error) {
	source, err := deferredSourceTx(ctx, tx, a, d, sale)
	if err != nil {
		return nil, err
	}

	var v DeferredPayment
	err = tx.QueryRowContext(ctx, `SELECT document_id,payment_code,total_cents,actor_id FROM fiscal_deferred_payments WHERE tenant_id=? AND store_id=? AND device_id=? AND sale_id=?`, a.TenantID, a.StoreID, d.DeviceID, sale).Scan(&v.DocumentID, &v.Code, &v.TotalCents, &v.SelectedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	if !v.Valid(total) || v.SelectedBy != a.IdentityID || v.DocumentID != source.DocumentID || v.TotalCents != source.TotalCents {
		return nil, ErrIntegrity
	}

	return &v, nil
}

// DeferredPaymentForSale returns no customer profile or invoice request.
func DeferredPaymentForSale(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, sale string) (*DeferredPayment, error) {
	if db == nil || c == nil || !idPattern.MatchString(sale) {
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
	if err = c.RequireTx(ctx, tx, a, d, identity.Sell, modules.Fiscal); err != nil {
		return nil, err
	}
	var role string
	if err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
		return nil, err
	}
	if role != "owner" {
		return nil, identity.ErrDenied
	}
	source, err := deferredSourceTx(ctx, tx, a, d, sale)
	if err != nil {
		return nil, err
	}
	v, err := deferredPaymentTx(ctx, tx, a, d, sale, source.TotalCents)
	if err != nil {
		return nil, err
	}
	return v, tx.Commit()
}
