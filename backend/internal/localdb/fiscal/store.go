package fiscal

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

type Profile struct {
	Input     ProfileInput `json:"input"`
	Revision  int          `json:"revision"`
	CreatedAt string       `json:"created_at"`
	Repeated  bool         `json:"repeated"`
	Readiness Readiness    `json:"readiness"`
}

// Configure appends a revision. Only a currently active owner can configure
// fiscal identity; ManageCash or a client-supplied role is not sufficient.
// The profile is always a draft, including when environment=production.
func Configure(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in ProfileInput, token string) (Profile, error) {
	var out Profile
	if !in.valid() {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return out, err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.ViewAccounting, modules.Fiscal); err != nil {
		return out, err
	}
	var role string
	if err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
		return out, err
	}
	if role != "owner" {
		return out, identity.ErrDenied
	}

	body, _ := json.Marshal(in)
	var old, actor, store string
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,revision,created_at FROM fiscal_profiles WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&old, &actor, &store, &out.Revision, &out.CreatedAt)
	if err == nil {
		if old != string(body) || actor != a.IdentityID || store != a.StoreID {
			return Profile{}, ErrConflict
		}
		out.Input = in
		out.Repeated = true
		out.Readiness = Unavailable()
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if in.RequireTaxPricing == nil {
		prior, err := profileTx(ctx, tx, a, in.ProfileID, 0)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return out, err
		}
		if err == nil && prior.Input.RequireTaxPricing != nil && *prior.Input.RequireTaxPricing {
			return out, ErrConflict
		}
	}
	var revision int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(max(revision),0) FROM fiscal_profiles WHERE tenant_id=? AND store_id=? AND profile_id=?`, a.TenantID, a.StoreID, in.ProfileID).Scan(&revision); err != nil {
		return out, err
	}
	if revision != in.ExpectedRevision {
		return out, ErrConflict
	}
	out = Profile{Input: in, Revision: revision + 1, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Readiness: Unavailable()}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_profiles(tenant_id,store_id,profile_id,revision,device_id,actor_id,operation_id,request_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.ProfileID, out.Revision, d.DeviceID, a.IdentityID, in.OperationID, string(body), out.CreatedAt)
	if err != nil {
		return Profile{}, err
	}
	return out, tx.Commit()
}

func profileTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, revision int) (Profile, error) {
	var out Profile
	var body string
	err := tx.QueryRowContext(ctx, `SELECT request_json,revision,created_at FROM fiscal_profiles WHERE tenant_id=? AND store_id=? AND profile_id=? AND (revision=? OR ?=0) ORDER BY revision DESC LIMIT 1`, a.TenantID, a.StoreID, id, revision, revision).Scan(&body, &out.Revision, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal([]byte(body), &out.Input); err != nil {
		return Profile{}, ErrIntegrity
	}
	canonical, _ := json.Marshal(out.Input)
	if string(canonical) != body || !out.Input.valid() || out.Input.ProfileID != id || out.Input.ExpectedRevision+1 != out.Revision {
		return Profile{}, ErrIntegrity
	}
	out.Readiness = Unavailable()
	return out, nil
}

func ReadProfile(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, revision int) (Profile, error) {
	if !idPattern.MatchString(id) || revision < 0 {
		return Profile{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return Profile{}, err
	}
	out, err := profileTx(ctx, tx, a, id, revision)
	if err != nil {
		return Profile{}, err
	}
	return out, tx.Commit()
}

type IntentInput struct {
	OperationID     string `json:"operation_id"`
	SaleID          string `json:"sale_id"`
	ProfileID       string `json:"profile_id"`
	ProfileRevision int    `json:"profile_revision"`
}
type SaleItem struct {
	Tax            *SaleTaxBinding `json:"tax,omitempty"`
	ID             string          `json:"id"`
	ProductID      string          `json:"product_id"`
	QuantityMilli  int64           `json:"quantity_milli"`
	UnitPriceCents int64           `json:"unit_price_cents"`
	TotalCents     int64           `json:"total_cents"`
}
type Payment struct {
	ID          string `json:"id"`
	Method      string `json:"method"`
	Status      string `json:"status"`
	AmountCents int64  `json:"amount_cents"`
}
type Snapshot struct {
	Version         int          `json:"version"`
	Profile         ProfileInput `json:"profile"`
	ProfileRevision int          `json:"profile_revision"`
	SaleID          string       `json:"sale_id"`
	TotalCents      int64        `json:"total_cents"`
	CommittedAt     string       `json:"committed_at"`
	Items           []SaleItem   `json:"items"`
	Payments        []Payment    `json:"payments"`
}
type Intent struct {
	ID             string      `json:"id"`
	Input          IntentInput `json:"input"`
	DeviceID       string      `json:"device_id"`
	ActorID        string      `json:"actor_id"`
	Status         string      `json:"status"`
	Snapshot       Snapshot    `json:"snapshot"`
	SnapshotSHA256 string      `json:"snapshot_sha256"`
	CreatedAt      string      `json:"created_at"`
	Repeated       bool        `json:"repeated"`
	Readiness      Readiness   `json:"readiness"`
}

// Prepare freezes server-side source data. It neither assigns a fiscal number
// nor signs/transmits XML. Missing contracts block writes, including replay.
func Prepare(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in IntentInput, token string) (Intent, error) {
	var out Intent
	if !idPattern.MatchString(in.OperationID) || !idPattern.MatchString(in.SaleID) || !idPattern.MatchString(in.ProfileID) || in.ProfileRevision < 1 {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return out, err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.Sell, modules.Fiscal); err != nil {
		return out, err
	}
	body, _ := json.Marshal(in)
	var id, old, store, actor string
	err = tx.QueryRowContext(ctx, `SELECT id,request_json,store_id,actor_id FROM fiscal_intents WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&id, &old, &store, &actor)
	if err == nil {
		if old != string(body) || store != a.StoreID || actor != a.IdentityID {
			return out, ErrConflict
		}
		out, err = intentTx(ctx, tx, a, id)
		if err != nil {
			return Intent{}, err
		}
		out.Repeated = true
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	p, err := profileTx(ctx, tx, a, in.ProfileID, 0)
	if err != nil {
		return out, err
	}
	if p.Revision != in.ProfileRevision {
		return out, ErrConflict
	}
	var used int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_intents WHERE tenant_id=? AND store_id=? AND sale_id=? AND model=? AND environment=?`, a.TenantID, a.StoreID, in.SaleID, p.Input.Model, p.Input.Environment).Scan(&used)
	if err != nil {
		return out, err
	}
	if used != 0 {
		return out, ErrConflict
	}
	snap := Snapshot{Version: 1, Profile: p.Input, ProfileRevision: p.Revision, SaleID: in.SaleID, Items: []SaleItem{}, Payments: []Payment{}}
	var state string
	err = tx.QueryRowContext(ctx, `SELECT status,total_cents,committed_at FROM sales WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.SaleID).Scan(&state, &snap.TotalCents, &snap.CommittedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if state != "committed" {
		return out, ErrConflict
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,product_id,quantity_milli,unit_price_cents,total_cents FROM sale_items WHERE tenant_id=? AND sale_id=? ORDER BY id`, a.TenantID, in.SaleID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item SaleItem
		if err = rows.Scan(&item.ID, &item.ProductID, &item.QuantityMilli, &item.UnitPriceCents, &item.TotalCents); err != nil {
			rows.Close()
			return out, err
		}
		snap.Items = append(snap.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(snap.Items) == 0 {
		return out, ErrIntegrity
	}
	bindings, err := StoredSaleTaxesTx(ctx, tx, a, d, in.SaleID)
	if err != nil {
		return out, err
	}
	for i := range snap.Items {
		if binding, ok := bindings[snap.Items[i].ID]; ok {
			if binding.ProfileID != in.ProfileID || binding.ProfileRevision != in.ProfileRevision {
				return out, ErrConflict
			}
			snap.Items[i].Tax = &binding
		}
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,method,status,amount_cents FROM sale_payments WHERE tenant_id=? AND sale_id=? ORDER BY id`, a.TenantID, in.SaleID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var pay Payment
		if err = rows.Scan(&pay.ID, &pay.Method, &pay.Status, &pay.AmountCents); err != nil {
			rows.Close()
			return out, err
		}
		snap.Payments = append(snap.Payments, pay)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(snap.Payments) == 0 {
		return out, ErrIntegrity
	}
	snapshot, _ := json.Marshal(snap)
	sum := sha256.Sum256(snapshot)
	id, err = localdb.NewID()
	if err != nil {
		return out, err
	}
	out = Intent{ID: id, Input: in, DeviceID: d.DeviceID, ActorID: a.IdentityID, Status: "blocked_configuration", Snapshot: snap, SnapshotSHA256: hex.EncodeToString(sum[:]), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Readiness: Unavailable()}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_intents(tenant_id,store_id,id,device_id,actor_id,operation_id,sale_id,profile_id,profile_revision,model,environment,request_json,snapshot_json,snapshot_sha256,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, out.ID, d.DeviceID, a.IdentityID, in.OperationID, in.SaleID, in.ProfileID, p.Revision, p.Input.Model, p.Input.Environment, string(body), string(snapshot), out.SnapshotSHA256, out.Status, out.CreatedAt)
	if err != nil {
		return Intent{}, err
	}
	sessionDigest := sha256.Sum256([]byte(token))
	if _, err = tx.ExecContext(ctx, `INSERT INTO fiscal_intent_session_bindings VALUES(?,?,?,?)`, a.TenantID, a.StoreID, out.ID, sessionDigest[:]); err != nil {
		return Intent{}, err
	}
	return out, tx.Commit()
}

func intentTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Intent, error) {
	var out Intent
	var body, snapshot, model, environment, sale, profile string
	var revision int
	err := tx.QueryRowContext(ctx, `SELECT id,request_json,snapshot_json,snapshot_sha256,status,created_at,actor_id,device_id,model,environment,sale_id,profile_id,profile_revision FROM fiscal_intents WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.ID, &body, &snapshot, &out.SnapshotSHA256, &out.Status, &out.CreatedAt, &out.ActorID, &out.DeviceID, &model, &environment, &sale, &profile, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256([]byte(snapshot))
	if hex.EncodeToString(sum[:]) != out.SnapshotSHA256 || out.Status != "blocked_configuration" {
		return Intent{}, ErrIntegrity
	}
	if json.Unmarshal([]byte(body), &out.Input) != nil || json.Unmarshal([]byte(snapshot), &out.Snapshot) != nil {
		return Intent{}, ErrIntegrity
	}
	canonical, _ := json.Marshal(out.Input)
	frozen, _ := json.Marshal(out.Snapshot)
	if string(canonical) != body || string(frozen) != snapshot || out.Snapshot.Version != 1 || out.Input.SaleID != sale || out.Snapshot.SaleID != sale || out.Input.ProfileID != profile || out.Snapshot.Profile.ProfileID != profile || out.Input.ProfileRevision != revision || out.Snapshot.ProfileRevision != revision || out.Snapshot.Profile.Model != model || out.Snapshot.Profile.Environment != environment || !out.Snapshot.Profile.valid() {
		return Intent{}, ErrIntegrity
	}
	out.Readiness = Unavailable()
	return out, nil
}

// Accounting readers can recover this store's records; sellers only their own
// device/actor attempts. Reading remains possible after license expiry.
func readPermission(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) (bool, error) {
	err := identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, identity.ErrDenied) {
		return false, err
	}
	return false, identity.CanOperateTx(ctx, tx, a, d, identity.Sell)
}

func ReadIntent(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, operation bool) (Intent, error) {
	if !idPattern.MatchString(id) {
		return Intent{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Intent{}, err
	}
	defer tx.Rollback()
	all, err := readPermission(ctx, tx, a, d)
	if err != nil {
		return Intent{}, err
	}
	if operation {
		var found string
		err = tx.QueryRowContext(ctx, `SELECT id FROM fiscal_intents WHERE tenant_id=? AND store_id=? AND device_id=? AND actor_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id).Scan(&found)
		if errors.Is(err, sql.ErrNoRows) {
			return Intent{}, ErrNotFound
		}
		if err != nil {
			return Intent{}, err
		}
		id = found
	}
	out, err := intentTx(ctx, tx, a, id)
	if err != nil {
		return Intent{}, err
	}
	if !all && (out.ActorID != a.IdentityID || out.DeviceID != d.DeviceID) {
		return Intent{}, ErrNotFound
	}
	return out, tx.Commit()
}

type Page struct {
	Items   []Intent `json:"items"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	HasMore bool     `json:"has_more"`
}

func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, saleID string, limit, offset int) (Page, error) {
	out := Page{Items: []Intent{}, Limit: limit, Offset: offset}
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000 || (saleID != "" && !idPattern.MatchString(saleID)) {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	all, err := readPermission(ctx, tx, a, d)
	if err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM fiscal_intents WHERE tenant_id=? AND store_id=? AND (?='' OR sale_id=?) AND (? OR (actor_id=? AND device_id=?)) ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, saleID, saleID, all, a.IdentityID, d.DeviceID, limit+1, offset)
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(ids) > limit {
		out.HasMore = true
		ids = ids[:limit]
	}
	for _, id := range ids {
		v, e := intentTx(ctx, tx, a, id)
		if e != nil {
			return Page{}, e
		}
		out.Items = append(out.Items, v)
	}
	return out, tx.Commit()
}

// The middleware check can precede a wait for SQLite. Resolve the exact token
// again inside the mutation transaction, so a logout/revocation cannot leave a
// previously validated actor capable of writing or replaying an old request.
func fiscalSessionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, token string) error {
	session, err := localauth.ResolveTx(ctx, tx, token)
	if err != nil {
		return err
	}
	if session.Actor != a || session.Device != d {
		return localauth.ErrDenied
	}
	return nil
}
