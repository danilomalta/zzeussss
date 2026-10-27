package sale

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type FinalizeCashPlanInput struct {
	OperationID   string `json:"operation_id"`
	ReceivedCents int64  `json:"received_cents"`
}

type PlanCompletion struct {
	SaleID        string `json:"sale_id"`
	OperationID   string `json:"operation_id"`
	SaleStatus    string `json:"sale_status"`
	ReceivedCents int64  `json:"received_cents"`
	ChangeCents   int64  `json:"change_cents"`
	CompletedAt   string `json:"completed_at"`
}

func samePlanPrices(a, b []QuoteItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Item != b[i].Item || a[i].Unit != b[i].Unit || a[i].UnitPriceCents != b[i].UnitPriceCents || a[i].TotalCents != b[i].TotalCents {
			return false
		}
	}
	return true
}

func planItems(plan Plan) []Item {
	items := make([]Item, len(plan.Items))
	for i, p := range plan.Items {
		items[i] = p.Item
	}
	return items
}

func cashOnly(plan Plan) bool {
	if len(plan.Parts) == 0 {
		return false
	}
	for _, p := range plan.Parts {
		if p.Method != "cash" {
			return false
		}
	}
	return true
}

func readPlanCompletionTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, plan *Plan) error {
	var result PlanCompletion
	err := tx.QueryRowContext(ctx, `SELECT f.sale_id,f.operation_id,s.status,f.received_cents,f.change_cents,f.completed_at
		FROM checkout_plan_sales f JOIN sales s ON s.tenant_id=f.tenant_id AND s.id=f.sale_id
		WHERE f.tenant_id=? AND f.store_id=? AND f.device_id=? AND f.checkout_id=?`, actor.TenantID, actor.StoreID, device.DeviceID, plan.CheckoutID).Scan(&result.SaleID, &result.OperationID, &result.SaleStatus, &result.ReceivedCents, &result.ChangeCents, &result.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		// This is an observed cash readiness hint, never an authorization grant
		// or price/stock reservation. Finalization checks the contract again.
		if plan.State == "prepared" && cashOnly(*plan) && checkShiftTx(ctx, tx, actor, device, plan.CashSessionID) == nil {
			items, total, e := priceItemsTx(ctx, tx, actor, planItems(*plan))
			plan.CanFinalize = e == nil && total == plan.TotalCents && samePlanPrices(plan.Items, items)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if plan.Cancellation != nil || !cashOnly(*plan) || result.SaleID != plan.CheckoutID || result.ReceivedCents < plan.TotalCents || result.ReceivedCents-plan.TotalCents != result.ChangeCents {
		return ErrConflict
	}
	for i, p := range plan.Parts {
		var method, status string
		var amount int64
		if err = tx.QueryRowContext(ctx, `SELECT method,status,amount_cents FROM sale_payments WHERE tenant_id=? AND sale_id=? AND id=?`, actor.TenantID, result.SaleID, p.PartID).Scan(&method, &status, &amount); err != nil {
			return err
		}
		if method != "cash" || amount != p.AmountCents || (result.SaleStatus == "committed" && status != "confirmed") || (result.SaleStatus == "cancelled" && status != "reversed") {
			return ErrConflict
		}
		plan.Parts[i].State = status
	}
	plan.State = "completed"
	plan.Completion = &result
	plan.CanFinalize = false
	return nil
}

// FinalizeCashPlan atomically creates the existing cash sale, stock consumption,
// net cash movement, stable payment IDs, outbox and tender/change audit link.
// It accepts only cash allocations; no electronic approval can be supplied.
func FinalizeCashPlan(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, id string, in FinalizeCashPlanInput) (Plan, error) {
	if db == nil || !planID(id) || !planID(in.OperationID) || in.ReceivedCents <= 0 || in.ReceivedCents > checkoutMaxSafe {
		return Plan{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Plan{}, err
	}
	defer tx.Rollback()
	if err = authorizeLegacy(ctx, tx, actor, device); err != nil {
		return Plan{}, err
	}
	plan, err := readPlanTx(ctx, tx, actor, device, id)
	if err != nil {
		return Plan{}, err
	}
	hash := planHash(in)
	if plan.Completion != nil {
		var originalHash string
		if err = tx.QueryRowContext(ctx, `SELECT request_hash FROM checkout_plan_sales WHERE tenant_id=? AND checkout_id=?`, actor.TenantID, id).Scan(&originalHash); err != nil {
			return Plan{}, err
		}
		if originalHash != hash || plan.Completion.OperationID != in.OperationID {
			return Plan{}, ErrConflict
		}
		plan.Repeated = true
		if err = tx.Commit(); err != nil {
			return Plan{}, err
		}
		return plan, nil
	}
	if plan.State != "prepared" {
		return Plan{}, ErrConflict
	}
	if !cashOnly(plan) {
		return Plan{}, ErrPaymentUnverified
	}
	if in.ReceivedCents < plan.TotalCents {
		return Plan{}, ErrInvalid
	}
	if contracts == nil {
		return Plan{}, ErrPaymentUnverified
	}
	var used int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM checkout_plan_operations WHERE tenant_id=? AND device_id=? AND operation_id=? AND NOT(checkout_id=? AND action='created')`, actor.TenantID, device.DeviceID, in.OperationID, id).Scan(&used); err != nil {
		return Plan{}, err
	}
	if used != 0 {
		return Plan{}, ErrConflict
	}
	authorize := func(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) error {
		return contracts.RequireTx(ctx, tx, a, d, identity.Sell, modules.POS)
	}
	if err = authorize(ctx, tx, actor, device); err != nil {
		return Plan{}, err
	}
	if err = checkShiftTx(ctx, tx, actor, device, plan.CashSessionID); err != nil {
		return Plan{}, err
	}
	items, total, err := priceItemsTx(ctx, tx, actor, planItems(plan))
	if err != nil {
		return Plan{}, err
	}
	if total != plan.TotalCents || !samePlanPrices(plan.Items, items) {
		return Plan{}, ErrConflict
	}
	saleInput := Input{OperationID: in.OperationID, SaleID: plan.CheckoutID, CashSessionID: plan.CashSessionID, Items: planItems(plan)}
	ids := make([]string, len(plan.Parts))
	for i, p := range plan.Parts {
		saleInput.Payments = append(saleInput.Payments, Payment{Method: "cash", AmountCents: p.AmountCents})
		ids[i] = p.PartID
	}
	if _, err = completeTx(ctx, tx, actor, device, saleInput, authorize, ids); err != nil {
		return Plan{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	write, err := tx.ExecContext(ctx, `INSERT INTO checkout_plan_sales VALUES (?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, id, in.OperationID, plan.CheckoutID, in.ReceivedCents, in.ReceivedCents-plan.TotalCents, hash, now)
	if err != nil {
		return Plan{}, err
	}
	count, err := write.RowsAffected()
	if err != nil {
		return Plan{}, err
	}
	if count != 1 {
		return Plan{}, ErrConflict
	}
	plan, err = readPlanTx(ctx, tx, actor, device, id)
	if err != nil {
		return Plan{}, err
	}
	if plan.Completion == nil {
		return Plan{}, ErrConflict
	}
	if err = tx.Commit(); err != nil {
		return Plan{}, err
	}
	return plan, nil
}
