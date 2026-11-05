package retail

import (
	"context"
	"database/sql"
	"math/big"
	"titansystem-backend/internal/localdb/register"
)

type RefundInput struct {
	OperationID           string `json:"operation_id"`
	ItemID                string `json:"item_id"`
	QuantityMilli         int64  `json:"quantity_milli"`
	ExpectedQuantityMilli int64  `json:"expected_quantity_milli"`
	ExpectedCents         int64  `json:"expected_cents"`
	SessionID             string `json:"session_id"`
	Reason                string `json:"reason"`
}

func (s Service) RefundSale(ctx context.Context, sale string, in RefundInput) error {
	if !ID(sale) || !ID(in.ItemID) || !ID(in.SessionID) || in.QuantityMilli < 1 || in.QuantityMilli > Max || in.ExpectedQuantityMilli < 0 || in.ExpectedCents < 1 || in.ExpectedCents > Max || len(in.Reason) > 500 || in.Reason == "" {
		return ErrInvalid
	}
	return s.Run(ctx, in.OperationID, "financial_refund", sale, in, func(tx *sql.Tx) error {
		c, e := ConfigTx(ctx, tx, s.Actor.TenantID)
		if e != nil {
			return e
		}
		if !c.RefundEnabled {
			return ErrDisabled
		}
		if e = RoleTx(ctx, tx, s.Actor, c.RefundRoles); e != nil {
			return e
		}
		var quantity, total, paid, priorQty, priorCents int64
		var status string
		e = tx.QueryRowContext(ctx, `SELECT i.quantity_milli,i.total_cents,v.status FROM sale_items i JOIN sales v ON v.tenant_id=i.tenant_id AND v.id=i.sale_id WHERE i.tenant_id=? AND v.store_id=? AND v.device_id=? AND i.sale_id=? AND i.id=?`, s.Actor.TenantID, s.Actor.StoreID, s.Device.DeviceID, sale, in.ItemID).Scan(&quantity, &total, &status)
		if e != nil {
			return ErrInvalid
		}
		if status != "committed" {
			return ErrConflict
		}
		var incompatible int
		if e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM customer_returns WHERE tenant_id=? AND sale_id=?)+(SELECT count(*) FROM receivable_credit_sales WHERE tenant_id=? AND sale_id=?)+(SELECT count(*) FROM sale_tax_bindings WHERE tenant_id=? AND sale_id=?)`, s.Actor.TenantID, sale, s.Actor.TenantID, sale, s.Actor.TenantID, sale).Scan(&incompatible); e != nil {
			return e
		}
		if incompatible > 0 {
			return ErrDisabled
		}
		if e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(amount_cents),0) FROM sale_payments WHERE tenant_id=? AND sale_id=? AND method='cash' AND status='confirmed'`, s.Actor.TenantID, sale).Scan(&paid); e != nil {
			return e
		}
		if e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_milli),0),COALESCE(sum(amount_cents),0) FROM retail_sale_refunds WHERE tenant_id=? AND sale_id=? AND item_id=?`, s.Actor.TenantID, sale, in.ItemID).Scan(&priorQty, &priorCents); e != nil {
			return e
		}
		if priorQty != in.ExpectedQuantityMilli || in.QuantityMilli > quantity-priorQty {
			return ErrConflict
		}
		// Cumulative integer allocation makes the last fraction consume the exact remainder.
		n := new(big.Int).Mul(big.NewInt(total), big.NewInt(priorQty+in.QuantityMilli))
		n.Div(n, big.NewInt(quantity))
		if !n.IsInt64() {
			return ErrInvalid
		}
		amount := n.Int64() - priorCents
		if amount != in.ExpectedCents || amount < 1 {
			return ErrConflict
		}
		var priorAll int64
		if e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(amount_cents),0) FROM retail_sale_refunds WHERE tenant_id=? AND sale_id=?`, s.Actor.TenantID, sale).Scan(&priorAll); e != nil {
			return e
		}
		if amount > paid-priorAll {
			return ErrConflict
		}
		movement, e := register.MoveTxWithContract(ctx, tx, s.Contracts, s.Actor, s.Device, register.MovementInput{OperationID: "rf-" + Hash([]string{s.Actor.TenantID, in.OperationID}), SessionID: in.SessionID, Kind: "withdrawal", AmountCents: amount, Reason: "Sale financial compensation " + sale})
		if e != nil {
			return e
		}
		if movement.Repeated {
			return ErrConflict
		}
		result, e := tx.ExecContext(ctx, `INSERT INTO retail_sale_refunds VALUES(?,?,?,?,?,?,?,?)`, s.Actor.TenantID, s.Actor.StoreID, sale, in.ItemID, in.OperationID, in.QuantityMilli, amount, movement.MovementID)
		if e != nil {
			return e
		}
		written, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if written != 1 {
			return ErrConflict
		}
		return nil
	})
}
