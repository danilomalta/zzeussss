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
	r, e := receiptTx(ctx, tx, a, id)
	if e != nil {
		return r, e
	}
	r.Void, e = receiptVoidTx(ctx, tx, a, r)
	return r, e
}
