package sale

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
)

const checkoutMaxSafe int64 = 9007199254740991

// PlanPartInput is an allocation, never an instruction to charge or grant credit.
type PlanPartInput struct {
	PartID      string `json:"part_id"`
	Method      string `json:"method"`
	AmountCents int64  `json:"amount_cents"`
}

type PlanInput struct {
	TaxQuoteOperations []string        `json:"tax_quote_operations,omitempty"`
	OperationID        string          `json:"operation_id"`
	CheckoutID         string          `json:"checkout_id"`
	CashSessionID      string          `json:"cash_session_id"`
	ExpectedTotalCents int64           `json:"expected_total_cents"`
	Items              []Item          `json:"items"`
	Parts              []PlanPartInput `json:"parts"`
}

type PlanPart struct {
	PlanPartInput
	State             string `json:"state"`
	ProviderConfirmed bool   `json:"provider_confirmed"`
}

type PlanCancellation struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
	CancelledAt string `json:"cancelled_at"`
}

type Plan struct {
	Fiscal        *fiscal.CheckoutStatus `json:"fiscal,omitempty"`
	CheckoutID    string                 `json:"checkout_id"`
	OperationID   string                 `json:"operation_id"`
	CashSessionID string                 `json:"cash_session_id"`
	State         string                 `json:"state"`
	TotalCents    int64                  `json:"total_cents"`
	Items         []QuoteItem            `json:"items"`
	Parts         []PlanPart             `json:"parts"`
	CreatedAt     string                 `json:"created_at"`
	Repeated      bool                   `json:"repeated"`
	CanFinalize   bool                   `json:"can_finalize"`
	Cancellation  *PlanCancellation      `json:"cancellation,omitempty"`
	Completion    *PlanCompletion        `json:"completion,omitempty"`
}

type CancelPlanInput struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
}

func planID(s string) bool {
	return s != "" && len(s) <= 128 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}

func validPlan(in PlanInput) bool {
	if !planID(in.OperationID) || !planID(in.CheckoutID) || !planID(in.CashSessionID) ||
		!validQuote(QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items}) ||
		in.ExpectedTotalCents <= 0 || in.ExpectedTotalCents > checkoutMaxSafe || len(in.Parts) == 0 || len(in.Parts) > 20 {
		return false
	}
	seen := map[string]bool{}
	var total int64
	for _, p := range in.Parts {
		if !planID(p.PartID) || seen[p.PartID] || p.AmountCents <= 0 || p.AmountCents > checkoutMaxSafe-total {
			return false
		}
		switch p.Method {
		case "cash", "debit", "credit", "pix", "creditAccount":
		default:
			return false
		}
		seen[p.PartID] = true
		total += p.AmountCents
	}
	return total == in.ExpectedTotalCents
}

func planHash(in any) string {
	body, _ := json.Marshal(in) // All callers pass typed structs with finite integer values.
	return fmt.Sprintf("%x", sha256.Sum256(body))
}

// CreatePlanWithContract persists an immutable, fully allocated quote snapshot.
// No money is collected, stock reserved or sale committed, including for cash.
// Replays return the original snapshot even after prices, stock or shift change.
func CreatePlanWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in PlanInput, token ...string) (Plan, error) {
	if db == nil || contracts == nil || !validPlan(in) {
		return Plan{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Plan{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, actor, device, identity.Sell, modules.POS); err != nil {
		return Plan{}, err
	}
	if len(in.TaxQuoteOperations) > 0 {
		if len(token) != 1 {
			return Plan{}, ErrInvalid
		}
		if err = fiscal.ValidateSaleSessionTx(ctx, tx, actor, device, token[0]); err != nil {
			return Plan{}, err
		}
		if err = contracts.RequireTx(ctx, tx, actor, device, identity.Sell, modules.Fiscal); err != nil {
			return Plan{}, err
		}
	}
	hash := planHash(in)
	var priorCheckout, action, priorHash string
	err = tx.QueryRowContext(ctx, `SELECT checkout_id,action,request_hash FROM checkout_plan_operations WHERE tenant_id=? AND device_id=? AND operation_id=?`, actor.TenantID, device.DeviceID, in.OperationID).Scan(&priorCheckout, &action, &priorHash)
	if err == nil {
		if priorCheckout != in.CheckoutID || action != "created" || priorHash != hash {
			return Plan{}, ErrConflict
		}
		result, e := readPlanTx(ctx, tx, actor, device, in.CheckoutID)
		if e != nil {
			return Plan{}, e
		}
		result.Repeated = true
		if e = tx.Commit(); e != nil {
			return Plan{}, e
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Plan{}, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM checkout_plans WHERE tenant_id=? AND checkout_id=?`, actor.TenantID, in.CheckoutID).Scan(&count); err != nil {
		return Plan{}, err
	}
	if count != 0 {
		return Plan{}, ErrConflict
	}
	if err = checkShiftTx(ctx, tx, actor, device, in.CashSessionID); err != nil {
		return Plan{}, err
	}
	items, total, err := priceItemsTx(ctx, tx, actor, in.Items)
	if err != nil {
		return Plan{}, err
	}
	bindings, total, err := priceTaxesTx(ctx, tx, actor, device, in.TaxQuoteOperations, items, total)
	if err != nil {
		return Plan{}, err
	}
	for i, b := range bindings {
		base := items[i].TotalCents
		items[i].MerchandiseCents = &base
		items[i].Tax = b
		items[i].TotalCents += b.AdditionalCents
	}
	if total != in.ExpectedTotalCents {
		return Plan{}, ErrConflict
	}
	for _, i := range items {
		if i.QuantityMilli > checkoutMaxSafe || i.UnitPriceCents > checkoutMaxSafe || i.AvailableMilli > checkoutMaxSafe {
			return Plan{}, ErrInvalid
		}
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return Plan{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO checkout_plans VALUES (?,?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, in.CheckoutID, actor.IdentityID, in.CashSessionID, in.OperationID, hash, total, string(encoded), now)
	if err != nil {
		return Plan{}, err
	}
	for i, p := range in.Parts {
		if _, err = tx.ExecContext(ctx, `INSERT INTO checkout_plan_parts VALUES (?,?,?,?,?,?)`, actor.TenantID, in.CheckoutID, p.PartID, i, p.Method, p.AmountCents); err != nil {
			return Plan{}, err
		}
	}
	if err = planEventTx(ctx, tx, actor, device, in.CheckoutID, in.OperationID, "created", hash, in, now); err != nil {
		return Plan{}, err
	}
	result, err := readPlanTx(ctx, tx, actor, device, in.CheckoutID)
	if err != nil {
		return Plan{}, err
	}
	if err = tx.Commit(); err != nil {
		return Plan{}, err
	}
	return result, nil
}

// ReadPlan requires the original human, company, store and device. Recovery of
// durable history does not require a currently licensed POS module or open shift.
func ReadPlan(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, id string) (Plan, error) {
	if db == nil || !planID(id) {
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
	result, err := readPlanTx(ctx, tx, actor, device, id)
	if err != nil {
		return Plan{}, err
	}
	if err = tx.Commit(); err != nil {
		return Plan{}, err
	}
	return result, nil
}

func readPlanTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, id string) (Plan, error) {
	result := Plan{State: "prepared", Parts: make([]PlanPart, 0), Items: make([]QuoteItem, 0)}
	var items string
	err := tx.QueryRowContext(ctx, `SELECT checkout_id,operation_id,cash_session_id,total_cents,items_json,created_at FROM checkout_plans WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_identity_id=? AND checkout_id=?`, actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, id).Scan(&result.CheckoutID, &result.OperationID, &result.CashSessionID, &result.TotalCents, &items, &result.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	if err != nil {
		return Plan{}, err
	}
	if err = json.Unmarshal([]byte(items), &result.Items); err != nil {
		return Plan{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT part_id,method,amount_cents FROM checkout_plan_parts WHERE tenant_id=? AND checkout_id=? ORDER BY position`, actor.TenantID, id)
	if err != nil {
		return Plan{}, err
	}
	for rows.Next() {
		p := PlanPart{State: "not_requested"}
		if err = rows.Scan(&p.PartID, &p.Method, &p.AmountCents); err != nil {
			rows.Close()
			return Plan{}, err
		}
		result.Parts = append(result.Parts, p)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return Plan{}, err
	}
	if closeErr != nil {
		return Plan{}, closeErr
	}
	cancel := &PlanCancellation{}
	err = tx.QueryRowContext(ctx, `SELECT operation_id,reason,cancelled_at FROM checkout_plan_cancellations WHERE tenant_id=? AND checkout_id=?`, actor.TenantID, id).Scan(&cancel.OperationID, &cancel.Reason, &cancel.CancelledAt)
	if err == nil {
		result.State = "cancelled"
		result.Cancellation = cancel
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Plan{}, err
	}
	if err = readPlanCompletionTx(ctx, tx, actor, device, &result); err != nil {
		return Plan{}, err
	}
	return result, nil
}

// CancelPlan cancels only the unsubmitted allocation implemented in this release.
// It never claims that an external charge was cancelled, refunded or reversed.
func CancelPlan(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, id string, in CancelPlanInput) (Plan, error) {
	if db == nil || !planID(id) || !planID(in.OperationID) || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 512 {
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
	result, err := readPlanTx(ctx, tx, actor, device, id)
	if err != nil {
		return Plan{}, err
	}
	hash := planHash(in)
	var priorCheckout, action, priorHash string
	err = tx.QueryRowContext(ctx, `SELECT checkout_id,action,request_hash FROM checkout_plan_operations WHERE tenant_id=? AND device_id=? AND operation_id=?`, actor.TenantID, device.DeviceID, in.OperationID).Scan(&priorCheckout, &action, &priorHash)
	if err == nil {
		if priorCheckout != id || action != "cancelled" || priorHash != hash {
			return Plan{}, ErrConflict
		}
		result.Repeated = true
		if err = tx.Commit(); err != nil {
			return Plan{}, err
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Plan{}, err
	}
	if result.State != "prepared" {
		return Plan{}, ErrConflict
	}
	// Documentary withdrawal/rejection does not prove that an external charge
	// was absent or refunded. Keep the allocation until external resolution.
	var proofs int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM checkout_payment_proofs WHERE tenant_id=? AND checkout_id=?`, actor.TenantID, id).Scan(&proofs); err != nil {
		return Plan{}, err
	}
	if proofs != 0 {
		return Plan{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO checkout_plan_cancellations VALUES (?,?,?,?,?,?)`, actor.TenantID, id, in.OperationID, actor.IdentityID, in.Reason, now)
	if err != nil {
		return Plan{}, err
	}
	if err = planEventTx(ctx, tx, actor, device, id, in.OperationID, "cancelled", hash, in, now); err != nil {
		return Plan{}, err
	}
	result, err = readPlanTx(ctx, tx, actor, device, id)
	if err != nil {
		return Plan{}, err
	}
	if err = tx.Commit(); err != nil {
		return Plan{}, err
	}
	return result, nil
}

func planEventTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, id, op, action, hash string, input any, now string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO checkout_plan_operations VALUES (?,?,?,?,?,?,?)`, actor.TenantID, device.DeviceID, op, id, action, hash, now); err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		ActorID string `json:"actor_id"`
		Input   any    `json:"input"`
	}{actor.IdentityID, input})
	if err != nil {
		return err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES (?,?,?,?,?,?,?,1,?,?)`, eventID, actor.TenantID, actor.StoreID, device.DeviceID, op, id, "checkout.plan."+action, string(payload), now)
	return err
}
