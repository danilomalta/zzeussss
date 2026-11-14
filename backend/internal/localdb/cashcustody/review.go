package cashcustody

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitewrite"
)

// Review is a custody investigation, not a cash adjustment or accounting loss.
// Recounts append compensating evidence; original bags and vaults never change.
type ReviewInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Action           string `json:"action"`
	CountedCents     int64  `json:"counted_cents"`
	Reason           string `json:"reason"`
}
type ReviewEvent struct {
	OperationID  string `json:"operation_id"`
	Revision     int64  `json:"revision"`
	ActorID      string `json:"actor_id"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	CountedCents int64  `json:"counted_cents"`
	DeltaCents   int64  `json:"delta_cents"`
	CreatedAt    string `json:"created_at"`
}
type Review struct {
	BagID                string        `json:"bag_id"`
	Revision             int64         `json:"revision"`
	State                string        `json:"state"`
	OriginalCountedCents int64         `json:"original_counted_cents"`
	DeclaredCents        int64         `json:"declared_cents"`
	CountedCents         int64         `json:"counted_cents"`
	DifferenceCents      int64         `json:"difference_cents"`
	History              []ReviewEvent `json:"history"`
}
type ReviewResult struct {
	OperationID string `json:"operation_id"`
	Repeated    bool   `json:"repeated"`
	Review      Review `json:"review"`
}

func reviewTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string, stop int64) (Review, error) {
	b, err := ReadTx(ctx, tx, a, d, id)
	if err != nil {
		return Review{}, err
	}
	if b.State != "received" || b.DifferenceCents == nil || *b.DifferenceCents == 0 {
		return Review{}, ErrConflict
	}
	v := Review{BagID: id, State: "pending", OriginalCountedCents: *b.CountedCents, DeclaredCents: *b.DeclaredCents, CountedCents: *b.CountedCents, History: []ReviewEvent{}}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id,revision,actor_id,action,reason,counted_cents,delta_cents,created_at FROM cash_bag_reviews WHERE tenant_id=? AND store_id=? AND device_id=? AND bag_id=? AND (?=0 OR revision<=?) ORDER BY revision`, a.TenantID, a.StoreID, d.DeviceID, id, stop, stop)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var e ReviewEvent
		if err = rows.Scan(&e.OperationID, &e.Revision, &e.ActorID, &e.Action, &e.Reason, &e.CountedCents, &e.DeltaCents, &e.CreatedAt); err != nil {
			return v, err
		}
		if e.Revision != v.Revision+1 || v.State == "resolved" || e.CountedCents-v.CountedCents != e.DeltaCents {
			return v, ErrInconsistent
		}
		v.State = "investigating"
		if e.Action == "resolve" {
			v.State = "resolved"
		}
		v.Revision = e.Revision
		v.CountedCents = e.CountedCents
		v.History = append(v.History, e)
	}
	v.DifferenceCents = v.CountedCents - v.DeclaredCents
	return v, rows.Err()
}
func ReadReview(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Review, error) {
	if db == nil || !pathID(id) {
		return Review{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback()
	v, err := reviewTx(ctx, tx, a, d, id, 0)
	if err != nil {
		return v, err
	}
	return v, tx.Commit()
}
func WriteReview(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in ReviewInput) (ReviewResult, error) {
	if db == nil || contracts == nil || !pathID(id) || !pathID(in.OperationID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= maxSafe || !validReason(in.Reason) || in.CountedCents < 0 || in.CountedCents > maxSafe || (in.Action != "investigate" && in.Action != "recount" && in.Action != "resolve") {
		return ReviewResult{}, ErrInvalid
	}
	var out ReviewResult
	err := sqlitewrite.Run(ctx, db, ErrConflict, func(tx *sql.Tx) error {
		if err := contracts.RequireTx(ctx, tx, a, d, identity.ManageCash, modules.POS); err != nil {
			return err
		}
		var role string
		if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
			return identity.ErrDenied
		}
		if role != "owner" {
			return identity.ErrDenied
		}
		fingerprint := hash(struct {
			Bag   string
			Input ReviewInput
		}{id, in})
		var saved, actor, bag string
		var rev int64
		err := tx.QueryRowContext(ctx, `SELECT request_hash,actor_id,bag_id,revision FROM cash_bag_reviews WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID).Scan(&saved, &actor, &bag, &rev)
		if err == nil {
			if saved != fingerprint || actor != a.IdentityID || bag != id {
				return ErrConflict
			}
			v, err := reviewTx(ctx, tx, a, d, id, rev)
			if err != nil {
				return err
			}
			out = ReviewResult{in.OperationID, true, v}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		v, err := reviewTx(ctx, tx, a, d, id, 0)
		if err != nil {
			return err
		}
		if v.State == "resolved" || v.Revision != in.ExpectedRevision {
			return ErrConflict
		}
		if in.Action != "recount" && in.CountedCents != v.CountedCents {
			return ErrConflict
		}
		if in.Action == "resolve" && v.State != "investigating" {
			return ErrConflict
		}
		e := ReviewEvent{in.OperationID, v.Revision + 1, a.IdentityID, in.Action, in.Reason, in.CountedCents, in.CountedCents - v.CountedCents, time.Now().UTC().Format(time.RFC3339Nano)}
		if err = checked(ctx, tx, `INSERT INTO cash_bag_reviews(tenant_id,store_id,device_id,bag_id,revision,operation_id,actor_id,action,reason,counted_cents,delta_cents,request_hash,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, id, e.Revision, e.OperationID, e.ActorID, e.Action, e.Reason, e.CountedCents, e.DeltaCents, fingerprint, e.CreatedAt); err != nil {
			return err
		}
		payload, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if err = checked(ctx, tx, `INSERT INTO cash_bag_review_events VALUES(?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, id, e.Revision, string(payload)); err != nil {
			return err
		}
		v, err = reviewTx(ctx, tx, a, d, id, 0)
		if err != nil {
			return err
		}
		out = ReviewResult{in.OperationID, false, v}
		return nil
	})
	if err != nil {
		return ReviewResult{}, err
	}
	return out, nil
}
