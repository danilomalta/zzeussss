package sale

import (
	"context"
	"database/sql"
	"errors"
	"titansystem-backend/internal/localdb/identity"
)

func proofReadTx(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, managerOnly bool) (*sql.Tx, bool, error) {
	if db == nil {
		return nil, false, ErrProofInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash)
	manager := err == nil
	if err != nil && !errors.Is(err, identity.ErrDenied) {
		tx.Rollback()
		return nil, false, err
	}
	if !manager {
		if managerOnly {
			tx.Rollback()
			return nil, false, identity.ErrDenied
		}
		if err = identity.CanOperateTx(ctx, tx, a, d, identity.Sell); err != nil {
			tx.Rollback()
			return nil, false, err
		}
	}
	return tx, manager, nil
}
func ReadPaymentProof(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (PaymentProof, error) {
	if !proofKey(id) {
		return PaymentProof{}, ErrProofInvalid
	}
	tx, manager, err := proofReadTx(ctx, db, a, d, false)
	if err != nil {
		return PaymentProof{}, err
	}
	defer tx.Rollback()
	var submitter string
	err = tx.QueryRowContext(ctx, `SELECT submitter_id FROM checkout_payment_proofs WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&submitter)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentProof{}, ErrProofNotFound
	}
	if err != nil {
		return PaymentProof{}, err
	}
	if !manager && submitter != a.IdentityID {
		return PaymentProof{}, ErrProofNotFound
	}
	p, err := loadPaymentProof(ctx, tx, a, d, id)
	if err != nil {
		return p, err
	}
	if err = tx.Commit(); err != nil {
		return PaymentProof{}, err
	}
	return p, nil
}
func PaymentProofReceipt(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (ProofResult, error) {
	if !proofKey(id) {
		return ProofResult{}, ErrProofInvalid
	}
	tx, _, err := proofReadTx(ctx, db, a, d, false)
	if err != nil {
		return ProofResult{}, err
	}
	defer tx.Rollback()
	var actor string
	err = tx.QueryRowContext(ctx, `SELECT actor_id FROM checkout_proof_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&actor)
	if errors.Is(err, sql.ErrNoRows) {
		return ProofResult{}, ErrProofNotFound
	}
	if err != nil {
		return ProofResult{}, err
	}
	if actor != a.IdentityID {
		return ProofResult{}, ErrProofNotFound
	}
	r, _, err := proofOperation(ctx, tx, a, d, id)
	if err != nil {
		return r, err
	}
	if err = tx.Commit(); err != nil {
		return ProofResult{}, err
	}
	return r, nil
}
func validProofState(s string) bool {
	switch s {
	case "all", "submitted", "accepted", "rejected", "withdrawn", "revoked":
		return true
	}
	return false
}
func ListPaymentProofs(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, state string, limit, offset int) (ProofPage, error) {
	if !validProofState(state) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return ProofPage{}, ErrProofInvalid
	}
	tx, _, err := proofReadTx(ctx, db, a, d, true)
	if err != nil {
		return ProofPage{}, err
	}
	defer tx.Rollback()
	p := ProofPage{Items: []PaymentProof{}, State: state, Limit: limit, Offset: offset}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM checkout_payment_proofs WHERE tenant_id=? AND store_id=? AND device_id=? AND (?='all' OR state=?)`, a.TenantID, a.StoreID, d.DeviceID, state, state).Scan(&p.TotalCount); err != nil {
		return p, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM checkout_payment_proofs WHERE tenant_id=? AND store_id=? AND device_id=? AND (?='all' OR state=?) ORDER BY rowid LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, d.DeviceID, state, state, limit, offset)
	if err != nil {
		return p, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return p, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, err
	}
	for _, id := range ids {
		proof, err := loadPaymentProof(ctx, tx, a, d, id)
		if err != nil {
			return ProofPage{}, err
		}
		p.Items = append(p.Items, proof)
	}
	p.HasMore = int64(offset+len(p.Items)) < p.TotalCount
	if err = tx.Commit(); err != nil {
		return ProofPage{}, err
	}
	return p, nil
}
