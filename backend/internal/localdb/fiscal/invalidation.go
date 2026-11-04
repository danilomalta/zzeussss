package fiscal

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

type HomologationInvalidationRecorder struct {
	db                                                            *sql.DB
	license                                                       *entitlementstore.Store
	actor                                                         identity.Scope
	device                                                        identity.DeviceContext
	profileID, operationID, endpoint, approval, token, scope, key string
	profileRevision                                               int
	input                                                         fiscalcore.Invalidation
}

var _ fiscalcore.InvalidationRecorder = (*HomologationInvalidationRecorder)(nil)

func NewHomologationInvalidationRecorder(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, profileID string, revision int, operationID, endpoint, approval, token string, in fiscalcore.Invalidation) (*HomologationInvalidationRecorder, error) {
	id, err := fiscalcore.InvalidationID(in)
	if err != nil {
		return nil, ErrInvalid
	}
	u, e := url.Parse(endpoint)
	if db == nil || license == nil || !idPattern.MatchString(profileID) || revision < 1 || !idPattern.MatchString(operationID) || !idPattern.MatchString(approval) || e != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || len(endpoint) > 4096 || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".gov.br") {
		return nil, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = queryReadPermissionTx(ctx, tx, a, d, token); err != nil {
		return nil, err
	}
	profile, err := profileTx(ctx, tx, a, profileID, revision)
	if err != nil {
		return nil, err
	}
	p := profile.Input
	if p.Environment != "homologation" || p.CNPJ != in.CNPJ || p.Model != in.Model || p.Series != in.Series || p.State != in.UF {
		return nil, ErrInvalid
	}
	encoded, _ := json.Marshal([]any{a.TenantID, a.StoreID, profileID, revision, operationID, approval, in})
	sum := sha256.Sum256(encoded)
	out := &HomologationInvalidationRecorder{db: db, license: license, actor: a, device: d, profileID: profileID, profileRevision: revision, operationID: operationID, endpoint: endpoint, approval: approval, token: token, scope: "fiscal-inut:" + hex.EncodeToString(sum[:]), key: id, input: in}
	return out, tx.Commit()
}
func (r *HomologationInvalidationRecorder) Scope() string { return r.scope }
func (r *HomologationInvalidationRecorder) ID() string    { return r.key }
func (r *HomologationInvalidationRecorder) sendPermissionTx(ctx context.Context, tx *sql.Tx) error {
	if r.input.Year > time.Now().Year() {
		return ErrInvalid
	}
	if err := queryReadPermissionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return err
	}
	if err := r.license.RequireTx(ctx, tx, r.actor, r.device, identity.ViewAccounting, modules.Fiscal); err != nil {
		return err
	}
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, r.actor.TenantID, r.actor.IdentityID).Scan(&role); err != nil {
		return err
	}
	if role != "owner" {
		return identity.ErrDenied
	}
	if err := numberBarrierTx(ctx, tx, r.actor.TenantID); err != nil {
		return err
	}
	profile, err := profileTx(ctx, tx, r.actor, r.profileID, 0)
	if err != nil {
		return err
	}
	if profile.Revision != r.profileRevision {
		return ErrConflict
	}
	var tenant, store string
	var first int64
	err = tx.QueryRowContext(ctx, `SELECT tenant_id,store_id,initial_number FROM fiscal_number_series WHERE issuer=? AND model=? AND series=?`, r.input.CNPJ, r.input.Model, r.input.Series).Scan(&tenant, &store, &first)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if tenant != r.actor.TenantID || store != r.actor.StoreID {
		return identity.ErrDenied
	}
	if r.input.First < first {
		return ErrConflict
	}
	return nil
}
func (r *HomologationInvalidationRecorder) availableTx(ctx context.Context, tx *sql.Tx) error {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM fiscal_invalidation_requests WHERE issuer=? AND model=? AND series=? AND first_number<=? AND last_number>=? AND NOT(tenant_id=? AND store_id=? AND operation_id=?))+(SELECT count(*) FROM fiscal_number_allocations WHERE issuer=? AND model=? AND series=? AND number BETWEEN ? AND ?)+(SELECT count(*) FROM fiscal_transmission_requests WHERE substr(access_key,7,14)=? AND substr(access_key,21,2)=? AND substr(access_key,23,3)=? AND CAST(substr(access_key,26,9) AS INTEGER) BETWEEN ? AND ?)`, r.input.CNPJ, r.input.Model, r.input.Series, r.input.Last, r.input.First, r.actor.TenantID, r.actor.StoreID, r.operationID, r.input.CNPJ, r.input.Model, r.input.Series, r.input.First, r.input.Last, r.input.CNPJ, r.input.Model, formatSeries(r.input.Series), r.input.First, r.input.Last).Scan(&count)
	if err != nil {
		return err
	}
	if count != 0 {
		return ErrConflict
	}
	return nil
}
func (r *HomologationInvalidationRecorder) ValidateInvalidation(ctx context.Context, in fiscalcore.Invalidation) error {
	if in != r.input {
		return ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = r.sendPermissionTx(ctx, tx); err != nil {
		return err
	}
	if err = r.availableTx(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *HomologationInvalidationRecorder) Reserve(ctx context.Context, in fiscalcore.RecordedRequest) error {
	original, err := fiscalcore.OriginalInvalidationFromRequest(in)
	if err != nil || original.Input != r.input || in.OperationID != r.operationID || in.Scope != r.scope || in.URL != r.endpoint || in.Key != r.key {
		return ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = numberLockTx(ctx, tx); err != nil {
		return err
	}
	if err = r.sendPermissionTx(ctx, tx); err != nil {
		return err
	}
	var scope string
	err = tx.QueryRowContext(ctx, `SELECT request_json FROM fiscal_invalidation_requests WHERE tenant_id=? AND store_id=? AND operation_id=?`, r.actor.TenantID, r.actor.StoreID, r.operationID).Scan(&scope)
	if err == nil {
		var old fiscalcore.RecordedRequest
		if fiscalcore.DecodeInput([]byte(scope), &old) != nil || old.Scope != r.scope {
			return ErrConflict
		}
		return fiscalcore.ErrRecorded
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err = r.availableTx(ctx, tx); err != nil {
		return err
	}
	raw, _ := json.Marshal(in)
	session := sha256.Sum256([]byte(r.token))
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_invalidation_requests VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.operationID, r.input.CNPJ, r.input.Model, r.input.Series, r.input.Year, r.input.First, r.input.Last, r.profileID, r.profileRevision, r.actor.IdentityID, r.device.DeviceID, session[:], r.approval, string(raw), in.RequestSHA256, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *HomologationInvalidationRecorder) BeforeSend(ctx context.Context, operationID string) error {
	if operationID != r.operationID {
		return ErrConflict
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = r.sendPermissionTx(ctx, tx); err != nil {
		return err
	}
	if err = r.availableTx(ctx, tx); err != nil {
		return err
	}
	if _, _, err = r.readTx(ctx, tx); err != nil {
		return err
	}
	var actor, device string
	var original []byte
	if err = tx.QueryRowContext(ctx, `SELECT actor_id,device_id,session_sha256 FROM fiscal_invalidation_requests WHERE tenant_id=? AND store_id=? AND operation_id=?`, r.actor.TenantID, r.actor.StoreID, r.operationID).Scan(&actor, &device, &original); err != nil {
		return err
	}
	current := sha256.Sum256([]byte(r.token))
	if actor != r.actor.IdentityID || device != r.device.DeviceID || subtle.ConstantTimeCompare(original, current[:]) != 1 {
		return localauth.ErrDenied
	}
	return tx.Commit()
}
func (r *HomologationInvalidationRecorder) Complete(ctx context.Context, in fiscalcore.ExchangeResult) error {
	if in.OperationID != r.operationID || !validQueryResult(in) {
		return ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = numberLockTx(ctx, tx); err != nil {
		return err
	}
	if _, _, err = r.readTx(ctx, tx); err != nil {
		return err
	}
	raw, _ := json.Marshal(in)
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_invalidation_results WHERE tenant_id=? AND store_id=? AND operation_id=?`, r.actor.TenantID, r.actor.StoreID, r.operationID).Scan(&previous)
	if err == nil {
		if previous != string(raw) {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_invalidation_results VALUES(?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.operationID, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *HomologationInvalidationRecorder) readTx(ctx context.Context, tx *sql.Tx) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	var req fiscalcore.RecordedRequest
	result := fiscalcore.ExchangeResult{OperationID: r.operationID, State: "uncertain"}
	var raw, hash, issuer, model, profile, approval string
	var series, year, revision int
	var first, last int64
	err := tx.QueryRowContext(ctx, `SELECT request_json,request_sha256,issuer,model,series,fiscal_year,first_number,last_number,profile_id,profile_revision,approval_reference FROM fiscal_invalidation_requests WHERE tenant_id=? AND store_id=? AND operation_id=?`, r.actor.TenantID, r.actor.StoreID, r.operationID).Scan(&raw, &hash, &issuer, &model, &series, &year, &first, &last, &profile, &revision, &approval)
	if errors.Is(err, sql.ErrNoRows) {
		return req, result, ErrNotFound
	}
	if err != nil {
		return req, result, err
	}
	if fiscalcore.DecodeInput([]byte(raw), &req) != nil {
		return req, result, ErrIntegrity
	}
	canonical, _ := json.Marshal(req)
	original, err := fiscalcore.OriginalInvalidationFromRequest(req)
	if err != nil || string(canonical) != raw || hash != req.RequestSHA256 || req.OperationID != r.operationID || req.Scope != r.scope || req.URL != r.endpoint || req.Key != r.key || original.Input != r.input || issuer != r.input.CNPJ || model != r.input.Model || series != r.input.Series || year != r.input.Year || first != r.input.First || last != r.input.Last || profile != r.profileID || revision != r.profileRevision || approval != r.approval {
		return req, result, ErrIntegrity
	}
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_invalidation_results WHERE tenant_id=? AND store_id=? AND operation_id=?`, r.actor.TenantID, r.actor.StoreID, r.operationID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return req, result, nil
	}
	if err != nil {
		return req, result, err
	}
	if fiscalcore.DecodeInput([]byte(raw), &result) != nil {
		return req, result, ErrIntegrity
	}
	canonical, _ = json.Marshal(result)
	if string(canonical) != raw || result.OperationID != r.operationID || !validQueryResult(result) {
		return req, result, ErrIntegrity
	}
	return req, result, nil
}
func (r *HomologationInvalidationRecorder) Read(ctx context.Context) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	defer tx.Rollback()
	if err = queryReadPermissionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	req, result, err := r.readTx(ctx, tx)
	if err != nil {
		return req, result, err
	}
	return req, result, tx.Commit()
}
