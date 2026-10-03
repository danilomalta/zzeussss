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

var ErrNoPolicy = errors.New("política de reposição não cadastrada")

type SuggestInput struct {
	OperationID string `json:"operation_id"`
	ProductID   string `json:"product_id"`
}

type SuggestResult struct {
	SuggestionID     string
	ObservedMilli    int64
	RecommendedMilli int64
	PolicyRevision   int64
	Needed           bool
	Repeated         bool
}

// Suggest observa os locais shelf/backroom/receiving da mesma loja.
// Não envia pedido, não marca pagamento e não altera estoque.
func Suggest(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in SuggestInput) (SuggestResult, error) {
	return suggest(ctx, db, actor, device, in, func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStock)
	})
}

func suggest(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in SuggestInput, authorize replenishmentAuthorization) (SuggestResult, error) {
	if db == nil {
		return SuggestResult{}, errors.New("banco local indisponível")
	}
	in.OperationID = strings.TrimSpace(in.OperationID)
	in.ProductID = strings.TrimSpace(in.ProductID)
	if in.OperationID == "" || in.ProductID == "" {
		return SuggestResult{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SuggestResult{}, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, actor, device); err != nil {
		return SuggestResult{}, err
	}
	var prior SuggestResult
	var product, store, by, status string
	err = tx.QueryRowContext(ctx, `SELECT id,product_id,store_id,actor_identity_id,observed_milli,recommended_milli,policy_revision,status
		FROM restock_suggestions WHERE tenant_id=? AND device_id=? AND operation_id=?`, actor.TenantID, device.DeviceID, in.OperationID).
		Scan(&prior.SuggestionID, &product, &store, &by, &prior.ObservedMilli, &prior.RecommendedMilli, &prior.PolicyRevision, &status)
	if err == nil {
		if product != in.ProductID || store != actor.StoreID || by != actor.IdentityID {
			return SuggestResult{}, ErrConflict
		}
		prior.Needed = status != "not_needed"
		prior.Repeated = true
		if err = tx.Commit(); err != nil {
			return SuggestResult{}, err
		}
		return prior, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return SuggestResult{}, err
	}
	var minimum, target, revision int64
	err = tx.QueryRowContext(ctx, `SELECT minimum_milli,target_milli,revision FROM restock_policies
		WHERE tenant_id=? AND store_id=? AND product_id=?`, actor.TenantID, actor.StoreID, in.ProductID).Scan(&minimum, &target, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return SuggestResult{}, ErrNoPolicy
	}
	if err != nil {
		return SuggestResult{}, err
	}
	var observed int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(m.quantity_milli),0) FROM stock_movements m
		JOIN stock_locations l ON l.tenant_id=m.tenant_id AND l.store_id=m.store_id AND l.id=m.location_id
		WHERE m.tenant_id=? AND m.store_id=? AND m.product_id=? AND l.kind IN ('shelf','backroom','receiving')`,
		actor.TenantID, actor.StoreID, in.ProductID).Scan(&observed)
	if err != nil {
		return SuggestResult{}, err
	}
	if observed < 0 || observed > MaxExact || target > MaxExact || revision > MaxExact {
		return SuggestResult{}, ErrInvalid
	}
	status = "not_needed"
	var recommended int64
	if observed < minimum {
		status = "suggested"
		recommended = target - observed
	}
	id, err := localdb.NewID()
	if err != nil {
		return SuggestResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = checkedExec(ctx, tx, `INSERT INTO restock_suggestions VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		actor.TenantID, actor.StoreID, device.DeviceID, id, in.OperationID, actor.IdentityID, in.ProductID, observed, minimum, target, revision, recommended, status, now)
	if err != nil {
		return SuggestResult{}, err
	}
	if status == "suggested" {
		payload, err := json.Marshal(struct {
			TenantID         string `json:"tenant_id"`
			StoreID          string `json:"store_id"`
			DeviceID         string `json:"device_id"`
			ActorID          string `json:"actor_id"`
			SuggestionID     string `json:"suggestion_id"`
			ProductID        string `json:"product_id"`
			ObservedMilli    int64  `json:"observed_milli"`
			RecommendedMilli int64  `json:"recommended_milli"`
			PolicyRevision   int64  `json:"policy_revision"`
		}{actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, id, in.ProductID, observed, recommended, revision})
		if err != nil {
			return SuggestResult{}, err
		}
		eventID, err := localdb.NewID()
		if err != nil {
			return SuggestResult{}, err
		}
		_, err = checkedExec(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
			VALUES (?,?,?,?,?,?,'restock.suggested',1,?,?)`, eventID, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, id, string(payload), now)
		if err != nil {
			return SuggestResult{}, fmt.Errorf("enfileirar sugestão: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return SuggestResult{}, err
	}
	return SuggestResult{SuggestionID: id, ObservedMilli: observed, RecommendedMilli: recommended, PolicyRevision: revision, Needed: status == "suggested"}, nil
}
