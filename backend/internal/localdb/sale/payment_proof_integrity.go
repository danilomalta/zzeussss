package sale

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"titansystem-backend/internal/localdb/identity"
)

func validPaymentProof(p PaymentProof) error {
	if p.Effect != "evidence_only" || p.ProviderConfirmed || p.AmountCents < 1 || p.AmountCents > checkoutMaxSafe || len(p.History) < 1 || len(p.History) > 3 {
		return ErrProofIntegrity
	}
	if p.Method != "debit" && p.Method != "credit" && p.Method != "pix" {
		return ErrProofIntegrity
	}
	state := ""
	reviewer := ""
	for i, e := range p.History {
		if e.FromState != state {
			return ErrProofIntegrity
		}
		ok := false
		switch e.Action {
		case "submit":
			ok = i == 0 && state == "" && e.ToState == "submitted" && e.ActorID == p.SubmitterID && e.Reason == "" && e.CreatedAt == p.CreatedAt
		case "withdraw":
			ok = state == "submitted" && e.ToState == "withdrawn" && e.ActorID == p.SubmitterID && proofText(e.Reason, 500)
		case "accept", "reject":
			ok = state == "submitted" && e.ActorID != p.SubmitterID && proofText(e.Reason, 500) && ((e.Action == "accept" && e.ToState == "accepted") || (e.Action == "reject" && e.ToState == "rejected"))
			reviewer = e.ActorID
		case "revoke":
			ok = state == "accepted" && e.ToState == "revoked" && e.ActorID != p.SubmitterID && proofText(e.Reason, 500)
			reviewer = e.ActorID
		}
		if !ok {
			return ErrProofIntegrity
		}
		state = e.ToState
	}
	last := p.History[len(p.History)-1]
	if p.State != state || p.UpdatedAt != last.CreatedAt || p.LastOperationID != last.OperationID {
		return ErrProofIntegrity
	}
	if reviewer == "" {
		if p.ReviewerID != nil {
			return ErrProofIntegrity
		}
	} else if p.ReviewerID == nil || *p.ReviewerID != reviewer {
		return ErrProofIntegrity
	}
	return nil
}
func proofOperation(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (ProofResult, string, error) {
	var r ProofResult
	var body, proofID, actor, action, from, to, reason, at, eventBody, eventType string
	err := tx.QueryRowContext(ctx, `SELECT o.result_json,o.proof_id,o.actor_id,o.action,o.from_state,o.to_state,o.reason,o.created_at,e.payload_json,e.event_type FROM checkout_proof_operations o LEFT JOIN checkout_proof_events e ON e.tenant_id=o.tenant_id AND e.store_id=o.store_id AND e.device_id=o.device_id AND e.operation_id=o.operation_id WHERE o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&body, &proofID, &actor, &action, &from, &to, &reason, &at, &eventBody, &eventType)
	if errors.Is(err, sql.ErrNoRows) {
		return r, "", ErrProofNotFound
	}
	if err != nil {
		return r, "", ErrProofIntegrity
	}
	if json.Unmarshal([]byte(body), &r) != nil || r.OperationID != id || r.Repeated || r.Proof.ID != proofID || validPaymentProof(r.Proof) != nil || eventBody != body || eventType != "checkout.proof."+action {
		return r, "", ErrProofIntegrity
	}
	if r.Proof.History[len(r.Proof.History)-1] != (ProofEvent{id, actor, action, from, to, reason, at}) {
		return r, "", ErrProofIntegrity
	}
	return r, actor, nil
}
func proofPlanTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Plan, string, error) {
	var submitter string
	err := tx.QueryRowContext(ctx, `SELECT actor_identity_id FROM checkout_plans WHERE tenant_id=? AND store_id=? AND device_id=? AND checkout_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&submitter)
	if errors.Is(err, sql.ErrNoRows) {
		return Plan{}, "", ErrProofNotFound
	}
	if err != nil {
		return Plan{}, "", err
	}
	original := a
	original.IdentityID = submitter
	p, err := readPlanTx(ctx, tx, original, d, id)
	return p, submitter, err
}
func loadPaymentProof(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (PaymentProof, error) {
	p := PaymentProof{Effect: "evidence_only", History: []ProofEvent{}}
	err := tx.QueryRowContext(ctx, `SELECT id,checkout_id,part_id,submitter_id,method,amount_cents,provider,terminal,reference,occurred_at,state,reviewer_id,created_at,updated_at,last_operation_id FROM checkout_payment_proofs WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&p.ID, &p.CheckoutID, &p.PartID, &p.SubmitterID, &p.Method, &p.AmountCents, &p.Provider, &p.Terminal, &p.Reference, &p.OccurredAt, &p.State, &p.ReviewerID, &p.CreatedAt, &p.UpdatedAt, &p.LastOperationID)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrProofNotFound
	}
	if err != nil {
		return p, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id,actor_id,action,from_state,to_state,reason,created_at FROM checkout_proof_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND proof_id=? ORDER BY CASE action WHEN 'submit' THEN 0 WHEN 'revoke' THEN 2 ELSE 1 END`, a.TenantID, a.StoreID, d.DeviceID, id)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var e ProofEvent
		if err = rows.Scan(&e.OperationID, &e.ActorID, &e.Action, &e.FromState, &e.ToState, &e.Reason, &e.CreatedAt); err != nil {
			rows.Close()
			return p, err
		}
		p.History = append(p.History, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, err
	}
	if err = validPaymentProof(p); err != nil {
		return p, err
	}
	for i, e := range p.History {
		r, _, err := proofOperation(ctx, tx, a, d, e.OperationID)
		if err != nil {
			return p, ErrProofIntegrity
		}
		expected := p
		expected.State = e.ToState
		expected.History = p.History[:i+1]
		expected.UpdatedAt = e.CreatedAt
		expected.LastOperationID = e.OperationID
		expected.ReviewerID = nil
		if e.Action == "accept" || e.Action == "reject" || e.Action == "revoke" {
			actor := e.ActorID
			expected.ReviewerID = &actor
		}
		if !reflect.DeepEqual(r.Proof, expected) {
			return p, ErrProofIntegrity
		}
	}
	plan, who, err := proofPlanTx(ctx, tx, a, d, p.CheckoutID)
	if err != nil {
		return p, err
	}
	if who != p.SubmitterID || plan.State != "prepared" {
		return p, ErrProofIntegrity
	}
	found := false
	for _, part := range plan.Parts {
		if part.PartID == p.PartID && part.Method == p.Method && part.AmountCents == p.AmountCents {
			found = true
		}
	}
	if !found {
		return p, ErrProofIntegrity
	}
	return p, nil
}
