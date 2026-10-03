package replenishment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrStale           = errors.New("sugestão desatualizada, calcule novamente")
	ErrPendingApproval = errors.New("já existe aprovação pendente para este produto")
	ErrNotFound        = errors.New("sugestão não encontrada")
)

type ReviewInput struct {
	OperationID  string `json:"operation_id"`
	SuggestionID string `json:"suggestion_id"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
}

type ReviewResult struct {
	SuggestionID string
	Decision     string
	Repeated     bool
}

// Review só permite decisão de gerente/dono. A aprovação confere política e
// saldo atual local antes de alterar o estado e registrar evento auditável.
func Review(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in ReviewInput) (ReviewResult, error) {
	return review(ctx, db, actor, device, in, func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return identity.CanOperateTx(ctx, tx, actor, device, identity.ManageReplenishment)
	})
}

func review(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in ReviewInput, authorize replenishmentAuthorization) (ReviewResult, error) {
	if db == nil {
		return ReviewResult{}, errors.New("banco local indisponível")
	}
	in.OperationID = strings.TrimSpace(in.OperationID)
	in.SuggestionID = strings.TrimSpace(in.SuggestionID)
	in.Reason = strings.TrimSpace(in.Reason)
	if in.OperationID == "" || in.SuggestionID == "" || (in.Decision != "approved" && in.Decision != "rejected") || in.Reason == "" || len(in.Reason) > 255 {
		return ReviewResult{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ReviewResult{}, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, actor, device); err != nil {
		return ReviewResult{}, err
	}
	var priorSuggestion, priorDecision, priorReason, priorReviewer, priorStore string
	err = tx.QueryRowContext(ctx, `SELECT suggestion_id,decision,reason,reviewer_identity_id,store_id FROM restock_reviews
		WHERE tenant_id=? AND device_id=? AND operation_id=?`, actor.TenantID, device.DeviceID, in.OperationID).
		Scan(&priorSuggestion, &priorDecision, &priorReason, &priorReviewer, &priorStore)
	if err == nil {
		if priorSuggestion != in.SuggestionID || priorDecision != in.Decision || priorReason != in.Reason || priorReviewer != actor.IdentityID || priorStore != actor.StoreID {
			return ReviewResult{}, ErrConflict
		}
		if err = tx.Commit(); err != nil {
			return ReviewResult{}, err
		}
		return ReviewResult{SuggestionID: in.SuggestionID, Decision: in.Decision, Repeated: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ReviewResult{}, err
	}
	var store, product, status string
	var observed, revision, recommended int64
	err = tx.QueryRowContext(ctx, `SELECT store_id,product_id,status,observed_milli,policy_revision,recommended_milli
		FROM restock_suggestions WHERE tenant_id=? AND id=?`, actor.TenantID, in.SuggestionID).
		Scan(&store, &product, &status, &observed, &revision, &recommended)
	if errors.Is(err, sql.ErrNoRows) {
		return ReviewResult{}, ErrNotFound
	}
	if err != nil {
		return ReviewResult{}, err
	}
	if store != actor.StoreID {
		return ReviewResult{}, identity.ErrDenied
	}
	if observed < 0 || observed > MaxExact || recommended < 0 || recommended > MaxExact || revision < 1 || revision > MaxExact {
		return ReviewResult{}, ErrInvalid
	}
	if status != "suggested" {
		return ReviewResult{}, ErrConflict
	}
	if in.Decision == "approved" {
		var currentRevision int64
		err = tx.QueryRowContext(ctx, `SELECT revision FROM restock_policies WHERE tenant_id=? AND store_id=? AND product_id=?`,
			actor.TenantID, actor.StoreID, product).Scan(&currentRevision)
		if errors.Is(err, sql.ErrNoRows) {
			return ReviewResult{}, ErrStale
		}
		if err != nil {
			return ReviewResult{}, err
		}
		var current int64
		err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(m.quantity_milli),0) FROM stock_movements m
			JOIN stock_locations l ON l.tenant_id=m.tenant_id AND l.store_id=m.store_id AND l.id=m.location_id
			WHERE m.tenant_id=? AND m.store_id=? AND m.product_id=? AND l.kind IN ('shelf','backroom','receiving')`,
			actor.TenantID, actor.StoreID, product).Scan(&current)
		if err != nil {
			return ReviewResult{}, err
		}
		if currentRevision != revision || current != observed {
			return ReviewResult{}, ErrStale
		}
		var existing int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM restock_suggestions
			WHERE tenant_id=? AND store_id=? AND product_id=? AND status='approved'`,
			actor.TenantID, actor.StoreID, product).Scan(&existing); err != nil {
			return ReviewResult{}, err
		}
		if existing > 0 {
			return ReviewResult{}, ErrPendingApproval
		}
	}
	result, err := checkedExec(ctx, tx, `UPDATE restock_suggestions SET status=? WHERE tenant_id=? AND store_id=? AND id=? AND status='suggested'`,
		in.Decision, actor.TenantID, actor.StoreID, in.SuggestionID)
	if err != nil {
		return ReviewResult{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ReviewResult{}, err
	}
	if changed != 1 {
		return ReviewResult{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = checkedExec(ctx, tx, `INSERT INTO restock_reviews VALUES (?,?,?,?,?,?,?,?,?)`,
		actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.SuggestionID, actor.IdentityID, in.Decision, in.Reason, now)
	if err != nil {
		return ReviewResult{}, err
	}
	payload, err := json.Marshal(struct {
		TenantID         string `json:"tenant_id"`
		StoreID          string `json:"store_id"`
		DeviceID         string `json:"device_id"`
		ActorID          string `json:"actor_id"`
		ProductID        string `json:"product_id"`
		RecommendedMilli int64  `json:"recommended_milli"`
		ReviewInput
	}{actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, product, recommended, in})
	if err != nil {
		return ReviewResult{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return ReviewResult{}, err
	}
	_, err = checkedExec(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
		VALUES (?,?,?,?,?,?,'restock.reviewed',1,?,?)`, eventID, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.SuggestionID, string(payload), now)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("enfileirar decisão: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return ReviewResult{}, err
	}
	return ReviewResult{SuggestionID: in.SuggestionID, Decision: in.Decision}, nil
}
