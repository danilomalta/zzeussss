package purchases

import (
	"context"
	"database/sql"
	"titansystem-backend/internal/localdb/identity"
)

// ReceiptEvidenceTx validates receipt, stock movement, audit and any later void
// in the caller's snapshot. It neither commits nor changes the transaction.
func ReceiptEvidenceTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Receipt, error) {
	if tx == nil || !validSearchID(id) {
		return Receipt{}, ErrInvalid
	}
	for _, p := range []identity.Permission{identity.ViewOrders, identity.ManageStock} {
		if e := identity.CanOperateTx(ctx, tx, a, d, p); e != nil {
			return Receipt{}, e
		}
	}
	return checkedReceiptEvidenceTx(ctx, tx, a, id)
}

// ReceiptForPayableTx authorizes a financial evidence read, not a stock action.
// It validates the same receipt, physical movement, audit and later reversal
// within the caller's transaction. Financial writes still require their own
// signed Finance entitlement, ManageCash and independent approval rules.
func ReceiptForPayableTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Receipt, error) {
	if tx == nil || !validSearchID(id) {
		return Receipt{}, ErrInvalid
	}
	if e := identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); e != nil {
		return Receipt{}, e
	}
	return checkedReceiptEvidenceTx(ctx, tx, a, id)
}

func checkedReceiptEvidenceTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Receipt, error) {
	r, e := receiptTx(ctx, tx, a, id)
	if e != nil {
		return r, e
	}
	r.Void, e = receiptVoidTx(ctx, tx, a, r)
	return r, e
}
