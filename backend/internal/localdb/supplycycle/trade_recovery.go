package supplycycle

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
)

var ErrTradeRecovery = errors.New("company trade requires owner review after restoration")

type TradeRecovery struct {
	Generation            string `json:"generation"`
	Required              bool   `json:"required"`
	ApprovedBy            string `json:"approved_by"`
	VerificationReference string `json:"verification_reference"`
}
type TradeRecoveryInput struct {
	OperationID           string `json:"operation_id"`
	Generation            string `json:"generation"`
	VerificationReference string `json:"verification_reference"`
}

func tradeRecoveryTx(ctx context.Context, tx *sql.Tx, a identity.Scope) (TradeRecovery, error) {
	var r TradeRecovery
	e := tx.QueryRowContext(ctx, `SELECT generation,required,approved_by,verification_reference FROM company_trade_recovery WHERE tenant_id=? AND store_id=?`, a.TenantID, a.StoreID).Scan(&r.Generation, &r.Required, &r.ApprovedBy, &r.VerificationReference)
	if errors.Is(e, sql.ErrNoRows) {
		return r, nil
	}
	return r, e
}
func tradeReadyTx(ctx context.Context, tx *sql.Tx, a identity.Scope) error {
	r, e := tradeRecoveryTx(ctx, tx, a)
	if e != nil {
		return e
	}
	if r.Required {
		return ErrTradeRecovery
	}
	return nil
}
func ReadTradeRecovery(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (TradeRecovery, error) {
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return TradeRecovery{}, e
	}
	defer tx.Rollback()
	r, e := tradeRecoveryTx(ctx, tx, a)
	if e != nil {
		return r, e
	}
	return r, tx.Commit()
}

// Recovery never discovers remote revocations by itself. Only an explicit owner
// decision, after reviewing all retained fingerprints through the business
// relationship's authenticated channel, can unblock this restored installation.
func ReviewRestoredTrade(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in TradeRecoveryInput) (TradeResult, error) {
	if !key(in.OperationID) || !key(in.Generation) || !text(in.VerificationReference, 255, false) {
		return TradeResult{}, purchases.ErrInvalid
	}
	tx, e := begin(ctx, db, c, a, d, false)
	if e != nil {
		return TradeResult{}, e
	}
	defer tx.Rollback()
	if e = tradeOwner(ctx, tx, a); e != nil {
		return TradeResult{}, e
	}
	state, e := tradeRecoveryTx(ctx, tx, a)
	if e != nil {
		return TradeResult{}, e
	}
	if state.Generation != in.Generation {
		return TradeResult{}, purchases.ErrConflict
	}
	body := request(in.Generation, "review_restored_trade", in)
	if r, ok, e := tradePrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
		return r, e
	}
	if !state.Required {
		return TradeResult{}, purchases.ErrConflict
	}
	e = one(ctx, tx, `UPDATE company_trade_recovery SET required=0,approved_by=?,verification_reference=?,updated_at=? WHERE tenant_id=? AND store_id=? AND generation=? AND required=1`, a.IdentityID, in.VerificationReference, time.Now().UTC().Format(time.RFC3339Nano), a.TenantID, a.StoreID, in.Generation)
	if e != nil {
		return TradeResult{}, e
	}
	state.Required = false
	state.ApprovedBy = a.IdentityID
	state.VerificationReference = in.VerificationReference
	return tradeFinish(ctx, tx, a, d, body, TradeResult{OperationID: in.OperationID, Recovery: &state})
}
