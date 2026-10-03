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
	ErrInvalid  = errors.New("regra de reposição inválida")
	ErrConflict = errors.New("operação de reposição reutilizada com outros dados")
)

type PolicyInput struct {
	OperationID  string `json:"operation_id"`
	ProductID    string `json:"product_id"`
	MinimumMilli int64  `json:"minimum_milli"`
	TargetMilli  int64  `json:"target_milli"`
}

type PolicyResult struct {
	Revision int64
	Repeated bool
}

// SetPolicy configura limite e alvo sob autorização gerencial. A mudança e
// seu evento local são atômicos. A API ainda deve provar a sessão do gerente.
func SetPolicy(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in PolicyInput) (PolicyResult, error) {
	return setPolicy(ctx, db, actor, device, in, func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return identity.CanOperateTx(ctx, tx, actor, device, identity.ManageReplenishment)
	})
}

func setPolicy(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in PolicyInput, authorize replenishmentAuthorization) (PolicyResult, error) {
	if db == nil {
		return PolicyResult{}, errors.New("banco local indisponível")
	}
	in.OperationID = strings.TrimSpace(in.OperationID)
	in.ProductID = strings.TrimSpace(in.ProductID)
	if in.OperationID == "" || in.ProductID == "" || in.MinimumMilli < 0 || in.TargetMilli <= in.MinimumMilli {
		return PolicyResult{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return PolicyResult{}, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, actor, device); err != nil {
		return PolicyResult{}, err
	}
	var store, product, by string
	var minimum, target, revision int64
	err = tx.QueryRowContext(ctx, `SELECT store_id,product_id,actor_identity_id,minimum_milli,target_milli,revision
		FROM restock_policy_changes WHERE tenant_id=? AND device_id=? AND operation_id=?`, actor.TenantID, device.DeviceID, in.OperationID).
		Scan(&store, &product, &by, &minimum, &target, &revision)
	if err == nil {
		if store != actor.StoreID || product != in.ProductID || by != actor.IdentityID || minimum != in.MinimumMilli || target != in.TargetMilli {
			return PolicyResult{}, ErrConflict
		}
		if err = tx.Commit(); err != nil {
			return PolicyResult{}, err
		}
		return PolicyResult{Revision: revision, Repeated: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return PolicyResult{}, err
	}
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM products WHERE tenant_id=? AND id=?`, actor.TenantID, in.ProductID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return PolicyResult{}, ErrInvalid
	}
	if err != nil {
		return PolicyResult{}, err
	}
	err = tx.QueryRowContext(ctx, `SELECT revision FROM restock_policies WHERE tenant_id=? AND store_id=? AND product_id=?`, actor.TenantID, actor.StoreID, in.ProductID).Scan(&revision)
	if !errors.Is(err, sql.ErrNoRows) && err != nil {
		return PolicyResult{}, err
	}
	if revision >= MaxExact {
		return PolicyResult{}, ErrInvalid
	}
	revision++
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = checkedExec(ctx, tx, `INSERT INTO restock_policies VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(tenant_id,store_id,product_id) DO UPDATE SET minimum_milli=excluded.minimum_milli,
		target_milli=excluded.target_milli,revision=excluded.revision,changed_by=excluded.changed_by,changed_at=excluded.changed_at`,
		actor.TenantID, actor.StoreID, in.ProductID, in.MinimumMilli, in.TargetMilli, revision, actor.IdentityID, now)
	if err != nil {
		return PolicyResult{}, err
	}
	_, err = checkedExec(ctx, tx, `INSERT INTO restock_policy_changes VALUES (?,?,?,?,?,?,?,?,?,?)`,
		actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, actor.IdentityID, in.ProductID, in.MinimumMilli, in.TargetMilli, revision, now)
	if err != nil {
		return PolicyResult{}, err
	}
	payload, err := json.Marshal(struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		DeviceID string `json:"device_id"`
		ActorID  string `json:"actor_id"`
		PolicyInput
		Revision int64 `json:"revision"`
	}{actor.TenantID, actor.StoreID, device.DeviceID, actor.IdentityID, in, revision})
	if err != nil {
		return PolicyResult{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return PolicyResult{}, err
	}
	_, err = checkedExec(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
		VALUES (?,?,?,?,?,?,'restock.policy_changed',1,?,?)`, eventID, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.ProductID, string(payload), now)
	if err != nil {
		return PolicyResult{}, fmt.Errorf("enfileirar política: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return PolicyResult{}, err
	}
	return PolicyResult{Revision: revision}, nil
}
