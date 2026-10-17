package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type RecipeStateInput struct {
	OperationID      string `json:"operation_id"`
	RecipeID         string `json:"recipe_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
}
type RecipeState struct {
	RecipeID string `json:"recipe_id"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
	Repeated bool   `json:"repeated"`
}

func recipeStateTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (RecipeState, error) {
	out := RecipeState{RecipeID: id, Status: "active"}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM production_recipes WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&n); err != nil {
		return out, err
	}
	if n != 1 {
		return out, ErrNotFound
	}
	err := tx.QueryRowContext(ctx, `SELECT status,revision FROM production_recipe_states WHERE tenant_id=? AND store_id=? AND recipe_id=?`, a.TenantID, a.StoreID, id).Scan(&out.Status, &out.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	return out, err
}
func GetRecipeState(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (RecipeState, error) {
	if !validID(id) {
		return RecipeState{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return RecipeState{}, err
	}
	defer tx.Rollback()
	out, err := recipeStateTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func ChangeRecipeState(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in RecipeStateInput) (RecipeState, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if !validID(in.OperationID) || !validID(in.RecipeID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= MaxRevision || (in.Status != "active" && in.Status != "inactive") || len(in.Reason) == 0 || len(in.Reason) > 255 {
		return RecipeState{}, ErrInvalid
	}
	if db == nil {
		return RecipeState{}, errors.New("banco indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return RecipeState{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, a, d, identity.ManageProduction, modules.Production); err != nil {
		return RecipeState{}, err
	}
	b, err := json.Marshal(in)
	if err != nil {
		return RecipeState{}, err
	}
	var saved, actor, store, status string
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,after_status,revision FROM production_recipe_state_events WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&saved, &actor, &store, &status, &revision)
	if err == nil {
		if saved != string(b) || actor != a.IdentityID || store != a.StoreID {
			return RecipeState{}, ErrConflict
		}
		return RecipeState{in.RecipeID, status, revision, true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return RecipeState{}, err
	}
	prior, err := recipeStateTx(ctx, tx, a, in.RecipeID)
	if err != nil {
		return RecipeState{}, err
	}
	if prior.Revision != in.ExpectedRevision || prior.Status == in.Status {
		return RecipeState{}, ErrConflict
	}
	revision = prior.Revision + 1
	if prior.Revision == 0 {
		err = one(ctx, tx, `INSERT INTO production_recipe_states VALUES(?,?,?,?,?)`, a.TenantID, a.StoreID, in.RecipeID, in.Status, revision)
	} else {
		err = one(ctx, tx, `UPDATE production_recipe_states SET status=?,revision=? WHERE tenant_id=? AND store_id=? AND recipe_id=? AND revision=?`, in.Status, revision, a.TenantID, a.StoreID, in.RecipeID, prior.Revision)
	}
	if err != nil {
		return RecipeState{}, err
	}
	err = one(ctx, tx, `INSERT INTO production_recipe_state_events VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, a.IdentityID, in.RecipeID, string(b), prior.Status, in.Status, revision, in.Reason, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return RecipeState{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return RecipeState{}, err
	}
	if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'production.recipe.state.changed',1,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, in.RecipeID, string(b), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return RecipeState{}, err
	}
	return RecipeState{in.RecipeID, in.Status, revision, false}, tx.Commit()
}
