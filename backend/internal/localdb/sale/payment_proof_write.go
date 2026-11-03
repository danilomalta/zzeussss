package sale

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func replayProof(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id, h string) (ProofResult, bool, error) {
	var who, saved string
	err := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash FROM checkout_proof_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &saved)
	if errors.Is(err, sql.ErrNoRows) {
		return ProofResult{}, false, nil
	}
	if err != nil {
		return ProofResult{}, false, err
	}
	if who != a.IdentityID {
		return ProofResult{}, false, ErrProofNotFound
	}
	if saved != h {
		return ProofResult{}, false, ErrProofConflict
	}
	r, _, err := proofOperation(ctx, tx, a, d, id)
	if err != nil {
		return r, false, err
	}
	r.Repeated = true
	return r, true, nil
}
func finishProof(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, p PaymentProof, e ProofEvent, h string) (ProofResult, error) {
	p.History = append(p.History, e)
	if err := validPaymentProof(p); err != nil {
		return ProofResult{}, err
	}
	r := ProofResult{OperationID: e.OperationID, Proof: p}
	body, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	if err = proofChecked(ctx, tx, `INSERT INTO checkout_proof_operations(tenant_id,store_id,device_id,operation_id,proof_id,actor_id,action,from_state,to_state,reason,request_hash,result_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, e.OperationID, p.ID, a.IdentityID, e.Action, e.FromState, e.ToState, e.Reason, h, string(body), e.CreatedAt); err != nil {
		return ProofResult{}, err
	}
	if err = proofChecked(ctx, tx, `INSERT INTO checkout_proof_events(tenant_id,store_id,device_id,operation_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,1,?,?)`, a.TenantID, a.StoreID, d.DeviceID, e.OperationID, "checkout.proof."+e.Action, string(body), e.CreatedAt); err != nil {
		return ProofResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return ProofResult{}, err
	}
	return r, nil
}

// SubmitPaymentProof records a manual reference. It never confirms a provider
// payment, reserves stock, accepts credit or completes a sale.
func SubmitPaymentProof(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, checkoutID, partID string, in SubmitProofInput) (ProofResult, error) {
	if db == nil || !planID(checkoutID) || !planID(partID) || !proofKey(in.ProofID) || !proofKey(in.OperationID) || !proofKey(in.Provider) || len(in.Provider) > 80 || !proofKey(in.Terminal) || len(in.Terminal) > 80 || !proofText(in.Reference, 160) || len(in.OccurredAt) > 64 {
		return ProofResult{}, ErrProofInvalid
	}
	occurred, err := time.Parse(time.RFC3339Nano, in.OccurredAt)
	if err != nil {
		return ProofResult{}, ErrProofInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ProofResult{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, a, d, identity.Sell, modules.POS); err != nil {
		return ProofResult{}, err
	}
	h := planHash(struct {
		CheckoutID, PartID string
		Input              SubmitProofInput
	}{checkoutID, partID, in})
	if r, ok, err := replayProof(ctx, tx, a, d, in.OperationID, h); ok || err != nil {
		return r, err
	}
	plan, err := readPlanTx(ctx, tx, a, d, checkoutID)
	if err != nil {
		return ProofResult{}, err
	}
	if plan.State != "prepared" {
		return ProofResult{}, ErrProofConflict
	}
	var part PlanPart
	found := false
	for _, v := range plan.Parts {
		if v.PartID == partID {
			part = v
			found = true
		}
	}
	if !found {
		return ProofResult{}, ErrProofNotFound
	}
	if part.Method != "debit" && part.Method != "credit" && part.Method != "pix" {
		return ProofResult{}, ErrProofInvalid
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	p := PaymentProof{ID: in.ProofID, CheckoutID: checkoutID, PartID: partID, SubmitterID: a.IdentityID, Method: part.Method, AmountCents: part.AmountCents, Provider: in.Provider, Terminal: in.Terminal, Reference: in.Reference, OccurredAt: occurred.UTC().Format(time.RFC3339Nano), State: "submitted", Effect: "evidence_only", CreatedAt: now, UpdatedAt: now, LastOperationID: in.OperationID, History: []ProofEvent{}}
	if err = proofChecked(ctx, tx, `INSERT INTO checkout_payment_proofs(tenant_id,store_id,device_id,id,checkout_id,part_id,submitter_id,method,amount_cents,provider,terminal,reference,occurred_at,state,created_at,updated_at,last_operation_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'submitted',?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, p.ID, p.CheckoutID, p.PartID, p.SubmitterID, p.Method, p.AmountCents, p.Provider, p.Terminal, p.Reference, p.OccurredAt, now, now, p.LastOperationID); err != nil {
		return ProofResult{}, err
	}
	return finishProof(ctx, tx, a, d, p, ProofEvent{OperationID: in.OperationID, ActorID: a.IdentityID, Action: "submit", ToState: "submitted", CreatedAt: now}, h)
}
func ReviewPaymentProof(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, action string, in ReviewProofInput) (ProofResult, error) {
	if db == nil || !proofKey(id) || !proofKey(in.OperationID) || !proofText(in.Reason, 500) {
		return ProofResult{}, ErrProofInvalid
	}
	permission := identity.ManageCash
	switch action {
	case "withdraw":
		permission = identity.Sell
	case "accept", "reject", "revoke":
	default:
		return ProofResult{}, ErrProofInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ProofResult{}, err
	}
	defer tx.Rollback()
	if err = contracts.RequireTx(ctx, tx, a, d, permission, modules.POS); err != nil {
		return ProofResult{}, err
	}
	h := planHash(struct {
		ID, Action string
		Input      ReviewProofInput
	}{id, action, in})
	if r, ok, err := replayProof(ctx, tx, a, d, in.OperationID, h); ok || err != nil {
		return r, err
	}
	p, err := loadPaymentProof(ctx, tx, a, d, id)
	if err != nil {
		return ProofResult{}, err
	}
	if action == "withdraw" {
		if p.SubmitterID != a.IdentityID {
			return ProofResult{}, ErrProofNotFound
		}
	} else if p.SubmitterID == a.IdentityID {
		return ProofResult{}, identity.ErrDenied
	}
	before := p.State
	after := ""
	switch action {
	case "accept", "reject", "withdraw":
		if before != "submitted" {
			return ProofResult{}, ErrProofConflict
		}
		switch action {
		case "accept":
			after = "accepted"
		case "reject":
			after = "rejected"
		case "withdraw":
			after = "withdrawn"
		}
	case "revoke":
		if before != "accepted" {
			return ProofResult{}, ErrProofConflict
		}
		after = "revoked"
	}
	if action != "withdraw" {
		p.ReviewerID = &a.IdentityID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	p.State = after
	p.UpdatedAt = now
	p.LastOperationID = in.OperationID
	if err = proofChecked(ctx, tx, `UPDATE checkout_payment_proofs SET state=?,reviewer_id=?,updated_at=?,last_operation_id=? WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND state=?`, after, p.ReviewerID, now, in.OperationID, a.TenantID, a.StoreID, d.DeviceID, id, before); err != nil {
		return ProofResult{}, err
	}
	return finishProof(ctx, tx, a, d, p, ProofEvent{in.OperationID, a.IdentityID, action, before, after, in.Reason, now}, h)
}
