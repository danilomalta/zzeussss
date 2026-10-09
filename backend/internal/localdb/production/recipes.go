// Package production publishes immutable local recipe versions. It does not
// reserve or consume stock, execute orders, or apply incoming production events.
package production

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrInvalid  = errors.New("receita invalida")
	ErrConflict = errors.New("operacao, versao ou revisao conflitante")
	ErrNotFound = errors.New("versao de receita nao encontrada")
)

const MaxQuantity int64 = 9007199254740991
const MaxRevision int64 = 2147483647

type Ingredient struct {
	ProductID     string `json:"product_id"`
	Unit          string `json:"unit"`
	QuantityMilli int64  `json:"quantity_milli"`
}

type PublishInput struct {
	OperationID      string       `json:"operation_id"`
	RecipeID         string       `json:"recipe_id"`
	VersionID        string       `json:"version_id"`
	ExpectedRevision int64        `json:"expected_revision"`
	Name             string       `json:"name"`
	OutputProductID  string       `json:"output_product_id"`
	OutputUnit       string       `json:"output_unit"`
	YieldMilli       int64        `json:"yield_milli"`
	Ingredients      []Ingredient `json:"ingredients"`
}

type Result struct {
	RecipeID  string `json:"recipe_id"`
	VersionID string `json:"version_id"`
	Revision  int64  `json:"revision"`
	Repeated  bool   `json:"repeated"`
}

type Version struct {
	PublishInput
	Revision  int64  `json:"revision"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
}

func validID(s string) bool {
	return len(s) > 0 && len(s) <= 128 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}

func validUnit(s string) bool {
	switch s {
	case "unit", "kg", "g", "liter", "ml", "meter":
		return true
	}
	return false
}

func normalize(in PublishInput) (PublishInput, string, error) {
	in.Name = strings.TrimSpace(in.Name)
	if !validID(in.OperationID) || !validID(in.RecipeID) || !validID(in.VersionID) || !validID(in.OutputProductID) ||
		in.ExpectedRevision < 0 || in.ExpectedRevision >= MaxRevision || len(in.Name) == 0 || len(in.Name) > 255 ||
		!validUnit(in.OutputUnit) || in.YieldMilli < 1 || in.YieldMilli > MaxQuantity || len(in.Ingredients) < 1 || len(in.Ingredients) > 100 {
		return PublishInput{}, "", ErrInvalid
	}
	in.Ingredients = append([]Ingredient(nil), in.Ingredients...)
	sort.Slice(in.Ingredients, func(i, j int) bool { return in.Ingredients[i].ProductID < in.Ingredients[j].ProductID })
	for i, item := range in.Ingredients {
		if !validID(item.ProductID) || item.ProductID == in.OutputProductID || !validUnit(item.Unit) ||
			item.QuantityMilli < 1 || item.QuantityMilli > MaxQuantity || (i > 0 && in.Ingredients[i-1].ProductID == item.ProductID) {
			return PublishInput{}, "", ErrInvalid
		}
	}
	body, err := json.Marshal(in)
	return in, string(body), err
}

// Require exactly one row even when a trigger suppresses a write.
func one(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	r, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

// Publish compares the current revision, then records the version, ingredients,
// audit and pending outbox event in the same authorization transaction.
func Publish(ctx context.Context, db *sql.DB, license *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, input PublishInput) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponivel")
	}
	in, body, err := normalize(input)
	if err != nil {
		return Result{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	if err = license.RequireTx(ctx, tx, actor, device, identity.ManageProduction, modules.Production); err != nil {
		return Result{}, err
	}
	var previousBody, previousActor, previousStore, previousRecipe, previousVersion string
	var revision int64
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,recipe_id,version_id,revision FROM production_recipe_versions WHERE tenant_id=? AND device_id=? AND operation_id=?`, actor.TenantID, device.DeviceID, in.OperationID).Scan(&previousBody, &previousActor, &previousStore, &previousRecipe, &previousVersion, &revision)
	if err == nil {
		if body != previousBody || previousActor != actor.IdentityID || previousStore != actor.StoreID {
			return Result{}, ErrConflict
		}
		err = tx.Commit()
		return Result{previousRecipe, previousVersion, revision, true}, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM production_recipe_versions WHERE tenant_id=? AND store_id=? AND version_id=?`, actor.TenantID, actor.StoreID, in.VersionID).Scan(&exists); err != nil {
		return Result{}, err
	}
	if exists != 0 {
		return Result{}, ErrConflict
	}
	revision = 0
	err = tx.QueryRowContext(ctx, `SELECT revision FROM production_recipes WHERE tenant_id=? AND store_id=? AND id=?`, actor.TenantID, actor.StoreID, in.RecipeID).Scan(&revision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	if revision != in.ExpectedRevision {
		return Result{}, ErrConflict
	}
	// Unit snapshots must match the catalog at publication. Conversions belong
	// to P03; no inferred density or silent kg/g conversion is performed here.
	checkUnit := func(id, unit string) error {
		var actual string
		e := tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, actor.TenantID, id).Scan(&actual)
		if errors.Is(e, sql.ErrNoRows) {
			return ErrInvalid
		}
		if e != nil {
			return e
		}
		if actual != unit {
			return ErrInvalid
		}
		return nil
	}
	if err = checkUnit(in.OutputProductID, in.OutputUnit); err != nil {
		return Result{}, err
	}
	for _, item := range in.Ingredients {
		if err = checkUnit(item.ProductID, item.Unit); err != nil {
			return Result{}, err
		}
	}
	revision++
	if in.ExpectedRevision == 0 {
		err = one(ctx, tx, `INSERT INTO production_recipes VALUES(?,?,?,?)`, actor.TenantID, actor.StoreID, in.RecipeID, revision)
	} else {
		err = one(ctx, tx, `UPDATE production_recipes SET revision=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=?`, revision, actor.TenantID, actor.StoreID, in.RecipeID, in.ExpectedRevision)
	}
	if err != nil {
		return Result{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = one(ctx, tx, `INSERT INTO production_recipe_versions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, in.RecipeID, revision, in.VersionID, in.Name, in.OutputProductID, in.OutputUnit, in.YieldMilli, device.DeviceID, in.OperationID, actor.IdentityID, body, now); err != nil {
		return Result{}, err
	}
	for _, item := range in.Ingredients {
		if err = one(ctx, tx, `INSERT INTO production_recipe_ingredients VALUES(?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, in.VersionID, item.ProductID, item.Unit, item.QuantityMilli); err != nil {
			return Result{}, err
		}
	}
	if err = one(ctx, tx, `INSERT INTO production_recipe_audit VALUES(?,?,?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, actor.IdentityID, in.RecipeID, in.VersionID, revision, now); err != nil {
		return Result{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return Result{}, err
	}
	payload, err := json.Marshal(Version{in, revision, now, actor.IdentityID})
	if err != nil {
		return Result{}, err
	}
	if err = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, eventID, actor.TenantID, actor.StoreID, device.DeviceID, in.OperationID, in.VersionID, "production.recipe.published", 1, string(payload), now); err != nil {
		return Result{}, err
	}
	err = tx.Commit()
	return Result{in.RecipeID, in.VersionID, revision, false}, err
}

// Reads remain available after contract expiry, but require current human and
// device authorization. Historical units and quantities come from the snapshot.
func readTx(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext) (*sql.Tx, error) {
	if db == nil {
		return nil, errors.New("banco local indisponivel")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.ManageProduction); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func scanVersion(row interface{ Scan(...any) error }) (Version, error) {
	var v Version
	var body string
	err := row.Scan(&body, &v.Revision, &v.CreatedAt, &v.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	if err != nil {
		return Version{}, err
	}
	if err = json.Unmarshal([]byte(body), &v.PublishInput); err != nil {
		return Version{}, err
	}
	if _, _, err = normalize(v.PublishInput); err != nil || v.Revision < 1 || v.Revision > MaxRevision || v.ExpectedRevision != v.Revision-1 {
		return Version{}, ErrConflict
	}
	return v, nil
}

func Get(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, versionID string) (Version, error) {
	if !validID(versionID) {
		return Version{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, actor, device)
	if err != nil {
		return Version{}, err
	}
	defer tx.Rollback()
	v, err := scanVersion(tx.QueryRowContext(ctx, `SELECT request_json,revision,created_at,actor_id FROM production_recipe_versions WHERE tenant_id=? AND store_id=? AND version_id=?`, actor.TenantID, actor.StoreID, versionID))
	if err != nil {
		return Version{}, err
	}
	if v.VersionID != versionID {
		return Version{}, ErrConflict
	}
	return v, tx.Commit()
}

// List returns all published versions, including historical ones, ordered by
// recipe and revision. An optional recipe filter uses the same scope.
func List(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, recipeID string, limit, offset int) ([]Version, error) {
	if (recipeID != "" && !validID(recipeID)) || limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalid
	}
	tx, err := readTx(ctx, db, actor, device)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT request_json,revision,created_at,actor_id FROM production_recipe_versions WHERE tenant_id=? AND store_id=? AND (?='' OR recipe_id=?) ORDER BY recipe_id,revision LIMIT ? OFFSET ?`, actor.TenantID, actor.StoreID, recipeID, recipeID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Version{}
	for rows.Next() {
		v, e := scanVersion(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	return items, tx.Commit()
}
