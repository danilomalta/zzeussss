package receivables

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/retail"
)

type ReversalInput struct {
	OperationID           string `json:"operation_id"`
	OriginalOperationID   string `json:"original_operation_id"`
	OriginalDeviceID      string `json:"original_device_id"`
	SessionID             string `json:"session_id"`
	AmountCents           int64  `json:"amount_cents"`
	ExpectedReversedCents int64  `json:"expected_reversed_cents"`
	Reason                string `json:"reason"`
}

// ReverseCash compensates a receipt without cancelling the sale, fiscal note or stock.
func ReverseCash(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, id string, in ReversalInput) (Result, error) {
	if !validID(in.OriginalOperationID) || !validID(in.OriginalDeviceID) || !validID(in.SessionID) || in.AmountCents < 1 || in.AmountCents > MaxSafe || in.ExpectedReversedCents < 0 || !text(in.Reason, 500) {
		return Result{}, ErrInvalid
	}
	return mutate(ctx, db, c, a, d, token, in.OperationID, id, "reverse_cash", in, func(tx *sql.Tx) error {
		cfg, e := retail.ConfigTx(ctx, tx, a.TenantID)
		if e != nil {
			return e
		}
		if !cfg.RefundEnabled {
			return retail.ErrDisabled
		}
		if e = retail.RoleTx(ctx, tx, a, cfg.RefundRoles); e != nil {
			return e
		}
		var amount, already int64
		var part string
		e = tx.QueryRowContext(ctx, `SELECT installment_id,amount_cents FROM receivable_cash_receipts WHERE tenant_id=? AND store_id=? AND document_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, id, in.OriginalDeviceID, in.OriginalOperationID).Scan(&part, &amount)
		if e != nil {
			return ErrNotFound
		}
		e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(amount_cents),0) FROM receivable_cash_reversals WHERE tenant_id=? AND store_id=? AND document_id=? AND original_device_id=? AND original_operation_id=?`, a.TenantID, a.StoreID, id, in.OriginalDeviceID, in.OriginalOperationID).Scan(&already)
		if e != nil {
			return e
		}
		if already != in.ExpectedReversedCents || in.AmountCents > amount-already {
			return ErrConflict
		}
		movement, e := register.MoveTxWithContract(ctx, tx, c, a, d, register.MovementInput{OperationID: "arr-" + digest([]string{a.TenantID, a.StoreID, in.OperationID}), SessionID: in.SessionID, Kind: "withdrawal", AmountCents: in.AmountCents, Reason: "Receivable compensation " + id})
		if e != nil {
			return e
		}
		if movement.Repeated {
			return ErrConflict
		}
		return checked(ctx, tx, `INSERT INTO receivable_cash_reversals VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, id, part, in.OperationID, in.OriginalDeviceID, in.OriginalOperationID, in.AmountCents, movement.MovementID)
	})
}
