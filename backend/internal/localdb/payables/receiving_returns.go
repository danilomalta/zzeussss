package payables

import (
	"context"
	"database/sql"
	"errors"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
)

type ReceivingReturnItem struct {
	ReturnID              string `json:"return_id"`
	CaseID                string `json:"case_id"`
	State                 string `json:"state"`
	RequestedMilli        int64  `json:"requested_milli"`
	AuthorizedMilli       int64  `json:"authorized_milli"`
	DispatchedMilli       int64  `json:"dispatched_milli"`
	SupplierReceivedMilli int64  `json:"supplier_received_milli"`
	RecoveredMilli        int64  `json:"recovered_milli"`
	FinancialInstruction  string `json:"financial_instruction"`
}
type ReceivingReturnReview struct {
	PayableID               string                `json:"payable_id"`
	ReceiptID               string                `json:"receipt_id"`
	Unit                    string                `json:"unit"`
	PayableState            string                `json:"payable_state"`
	TotalCents              int64                 `json:"total_cents"`
	CashAppliedCents        int64                 `json:"cash_applied_cents"`
	OutstandingCents        int64                 `json:"outstanding_cents"`
	FinancialEffectRecorded bool                  `json:"financial_effect_recorded"`
	ExternalVerified        bool                  `json:"external_verified"`
	ReviewRequired          bool                  `json:"review_required"`
	SourceComplete          bool                  `json:"source_complete"`
	Items                   []ReceivingReturnItem `json:"items"`
}

// Physical return records are not credits or refunds. Existing authoritative
// balances are projected unchanged, in the same transaction as return evidence.
func ReceivingReturns(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (ReceivingReturnReview, error) {
	out := ReceivingReturnReview{Items: []ReceivingReturnItem{}, SourceComplete: true}
	if !key(id) {
		return out, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	p, e := load(ctx, tx, a, d, id)
	if e != nil {
		return out, e
	}
	var receipt string
	e = tx.QueryRowContext(ctx, `SELECT receipt_id FROM payable_receiving_origins WHERE tenant_id=? AND store_id=? AND device_id=? AND payable_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&receipt)
	if errors.Is(e, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if e != nil {
		return out, e
	}
	rows, e := purchases.SupplierReturnsForAccountingTx(ctx, tx, a, d, receipt)
	if e != nil {
		return out, e
	}
	_, cash, _, e := cashForDocument(ctx, tx, a, d, p)
	if e != nil {
		return out, e
	}
	out.PayableID = id
	out.ReceiptID = receipt
	out.PayableState = p.State
	out.TotalCents = p.TotalCents
	out.CashAppliedCents = cash.AppliedCents
	out.OutstandingCents = cash.OutstandingCents
	if e = tx.QueryRowContext(ctx, `SELECT unit FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, receipt).Scan(&out.Unit); e != nil {
		return out, e
	}
	for _, v := range rows {
		if v.State != "cancelled" && v.State != "declined" {
			out.ReviewRequired = true
		}
		out.Items = append(out.Items, ReceivingReturnItem{v.ID, v.CaseID, v.State, v.RequestedMilli, v.AuthorizedMilli, v.DispatchedMilli, v.SupplierReceivedMilli, v.RecoveredMilli, v.FinancialInstruction})
	}
	return out, tx.Commit()
}
