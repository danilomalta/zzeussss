package production

import (
	"context"
	"database/sql"

	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitewrite"
)

// writeProduction holds the writer before reading session or business state.
// HTTP always supplies its admitted token; omission preserves trusted in-process
// callers, which still pass the existing contract and permission checks.
func writeProduction[T any](ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, token []string, work func(*sql.Tx) (T, error)) (T, error) {
	var out T
	if len(token) > 1 {
		return out, ErrInvalid
	}
	err := sqlitewrite.Run(ctx, db, ErrConflict, func(tx *sql.Tx) error {
		if len(token) == 1 {
			if e := fiscal.ValidateSaleSessionTx(ctx, tx, a, d, token[0]); e != nil {
				return e
			}
		}
		var e error
		out, e = work(tx)
		return e
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}
