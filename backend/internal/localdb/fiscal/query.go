package fiscal

import (
	"bytes"
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

// HomologationQueryRecorder is a distinct write-ahead journal for consSitNFe.
// An authorized accounting reader may query an archived attempt in a NEW
// session, including after Restore; this never resumes its issuance authority.
// Only the trusted host selects the endpoint; no renderer route exposes it.
type HomologationQueryRecorder struct {
	db                                                           *sql.DB
	license                                                      *entitlementstore.Store
	actor                                                        identity.Scope
	device                                                       identity.DeviceContext
	intentID, queryID, endpoint, token, scope, key, originalHash string
}

var _ fiscalcore.Recorder = (*HomologationQueryRecorder)(nil)
var _ fiscalcore.SendGuard = (*HomologationQueryRecorder)(nil)

func NewHomologationQueryRecorder(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, intentID, queryID, endpoint, token string) (*HomologationQueryRecorder, error) {
	u, err := url.Parse(endpoint)
	if db == nil || license == nil || !idPattern.MatchString(intentID) || !idPattern.MatchString(queryID) || err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || len(endpoint) > 4096 || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".gov.br") {
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
	original, _, err := originalTransmissionTx(ctx, tx, a, intentID)
	if err != nil {
		return nil, err
	}
	if original.OperationID == queryID {
		return nil, ErrConflict
	}
	// Query scope remains stable for current authorized readers after logout.
	// Session/actor/device authority for sending is bound separately at Reserve.
	identityBytes, _ := json.Marshal([]string{a.TenantID, a.StoreID, intentID, queryID, original.RequestSHA256})
	digest := sha256.Sum256(identityBytes)
	out := &HomologationQueryRecorder{db: db, license: license, actor: a, device: d, intentID: intentID, queryID: queryID, endpoint: endpoint, token: token, scope: "fiscal-query:" + hex.EncodeToString(digest[:]), key: original.Key, originalHash: original.RequestSHA256}
	return out, tx.Commit()
}
func (r *HomologationQueryRecorder) Scope() string { return r.scope }
func (r *HomologationQueryRecorder) Key() string   { return r.key }

func queryReadPermissionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, token string) error {
	if err := fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return err
	}
	return identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting)
}

// This reads an immutable transmission without impersonating its original
// human session. Caller must already have current accounting read permission.
func originalTransmissionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, intentID string) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	var req fiscalcore.RecordedRequest
	var body, actor, device, key string
	err := tx.QueryRowContext(ctx, `SELECT request_json,actor_id,device_id,access_key FROM fiscal_transmission_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, a.TenantID, a.StoreID, intentID).Scan(&body, &actor, &device, &key)
	if errors.Is(err, sql.ErrNoRows) {
		return req, fiscalcore.ExchangeResult{}, ErrNotFound
	}
	if err != nil {
		return req, fiscalcore.ExchangeResult{}, err
	}
	if fiscalcore.DecodeInput([]byte(body), &req) != nil {
		return req, fiscalcore.ExchangeResult{}, ErrIntegrity
	}
	if !fiscalcore.ValidAccessKey(key) {
		return req, fiscalcore.ExchangeResult{}, ErrIntegrity
	}
	identityBytes, _ := json.Marshal([]string{a.TenantID, a.StoreID, device, actor, intentID})
	hash := sha256.Sum256(identityBytes)
	originalScope := identity.Scope{TenantID: a.TenantID, StoreID: a.StoreID, IdentityID: actor}
	originalDevice := identity.DeviceContext{TenantID: a.TenantID, StoreID: a.StoreID, DeviceID: device}
	reader := &HomologationRecorder{actor: originalScope, device: originalDevice, intentID: intentID, endpoint: req.URL, key: key, scope: "fiscal-homolog:" + hex.EncodeToString(hash[:])}
	original, result, err := reader.readTx(ctx, tx)
	if err != nil {
		return original, result, err
	}
	invoice, err := fiscalcore.OriginalNFeFromRequest(original)
	if err != nil {
		return original, result, ErrIntegrity
	}
	intent, err := intentTx(ctx, tx, a, intentID)
	if err != nil {
		return original, result, err
	}
	p := intent.Snapshot.Profile
	if p.Environment != "homologation" || invoice.Key[:2] != stateCodes[p.State] || invoice.Issuer != p.CNPJ || invoice.Key[20:22] != p.Model || invoice.Key[22:25] != formatSeries(p.Series) {
		return original, result, ErrIntegrity
	}
	return original, result, nil
}

func (r *HomologationQueryRecorder) sendPermissionTx(ctx context.Context, tx *sql.Tx) error {
	if err := queryReadPermissionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return err
	}
	if err := r.license.RequireTx(ctx, tx, r.actor, r.device, identity.ViewAccounting, modules.Fiscal); err != nil {
		return err
	}
	// Restore barrier deliberately does not prevent status LOOKUPS. It still
	// blocks original issuance and numbering, and this method never removes it.
	original, _, err := originalTransmissionTx(ctx, tx, r.actor, r.intentID)
	if err != nil {
		return err
	}
	if original.Key != r.key || original.RequestSHA256 != r.originalHash {
		return ErrIntegrity
	}
	return nil
}

func (r *HomologationQueryRecorder) Reserve(ctx context.Context, in fiscalcore.RecordedRequest) error {
	expected, err := fiscalcore.BuildNFeConsultation(r.key, "2")
	if err != nil {
		return ErrIntegrity
	}
	hash := sha256.Sum256(in.Body)
	if in.OperationID != r.queryID || in.Scope != r.scope || in.URL != r.endpoint || in.Key != r.key || in.Environment != "2" || in.Issuer != r.key[6:20] || !bytes.Equal(in.Body, expected) || in.RequestSHA256 != hex.EncodeToString(hash[:]) {
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
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_query_requests WHERE tenant_id=? AND store_id=? AND query_id=?`, r.actor.TenantID, r.actor.StoreID, r.queryID).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fiscalcore.ErrRecorded
	}
	body, _ := json.Marshal(in)
	sessionHash := sha256.Sum256([]byte(r.token))
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_query_requests VALUES(?,?,?,?,?,?,?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.queryID, r.intentID, r.device.DeviceID, r.actor.IdentityID, sessionHash[:], r.originalHash, string(body), in.RequestSHA256, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *HomologationQueryRecorder) BeforeSend(ctx context.Context, operationID string) error {
	if operationID != r.queryID {
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
	if _, _, err = r.readTx(ctx, tx); err != nil {
		return err
	}
	var actor, device string
	var originalSession []byte
	if err = tx.QueryRowContext(ctx, `SELECT actor_id,device_id,session_sha256 FROM fiscal_query_requests WHERE tenant_id=? AND store_id=? AND query_id=?`, r.actor.TenantID, r.actor.StoreID, r.queryID).Scan(&actor, &device, &originalSession); err != nil {
		return err
	}
	currentHash := sha256.Sum256([]byte(r.token))
	if actor != r.actor.IdentityID || device != r.device.DeviceID || subtle.ConstantTimeCompare(originalSession, currentHash[:]) != 1 {
		return localauth.ErrDenied
	}
	return tx.Commit()
}

// Complete preserves in-flight evidence even if its human session ended.
func (r *HomologationQueryRecorder) Complete(ctx context.Context, in fiscalcore.ExchangeResult) error {
	if in.OperationID != r.queryID || !validQueryResult(in) {
		return ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err = r.readTx(ctx, tx); err != nil {
		return err
	}
	body, _ := json.Marshal(in)
	var old string
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_query_results WHERE tenant_id=? AND store_id=? AND query_id=?`, r.actor.TenantID, r.actor.StoreID, r.queryID).Scan(&old)
	if err == nil {
		if old != string(body) {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_query_results VALUES(?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.queryID, string(body), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func validQueryResult(in fiscalcore.ExchangeResult) bool {
	if len(in.Body) > 2*fiscalcore.MaxXML || (in.HTTPStatus != 0 && (in.HTTPStatus < 100 || in.HTTPStatus > 599)) {
		return false
	}
	if in.State == "uncertain" {
		return len(in.Body) == 0 && in.SHA256 == ""
	}
	if in.State != "response_received" || in.HTTPStatus == 0 {
		return false
	}
	hash := sha256.Sum256(in.Body)
	return in.SHA256 == hex.EncodeToString(hash[:])
}
func (r *HomologationQueryRecorder) readTx(ctx context.Context, tx *sql.Tx) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	var req fiscalcore.RecordedRequest
	result := fiscalcore.ExchangeResult{OperationID: r.queryID, State: "uncertain"}
	var body, hash, intentID, originalHash string
	err := tx.QueryRowContext(ctx, `SELECT request_json,request_sha256,intent_id,original_request_sha256 FROM fiscal_query_requests WHERE tenant_id=? AND store_id=? AND query_id=?`, r.actor.TenantID, r.actor.StoreID, r.queryID).Scan(&body, &hash, &intentID, &originalHash)
	if errors.Is(err, sql.ErrNoRows) {
		return req, result, ErrNotFound
	}
	if err != nil {
		return req, result, err
	}
	if fiscalcore.DecodeInput([]byte(body), &req) != nil {
		return req, result, ErrIntegrity
	}
	canonical, _ := json.Marshal(req)
	sum := sha256.Sum256(req.Body)
	expected, err := fiscalcore.BuildNFeConsultation(r.key, "2")
	if err != nil || string(canonical) != body || hash != req.RequestSHA256 || hash != hex.EncodeToString(sum[:]) || intentID != r.intentID || originalHash != r.originalHash || req.OperationID != r.queryID || req.Scope != r.scope || req.URL != r.endpoint || req.Key != r.key || req.Issuer != r.key[6:20] || req.Environment != "2" || !bytes.Equal(req.Body, expected) {
		return req, result, ErrIntegrity
	}
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_query_results WHERE tenant_id=? AND store_id=? AND query_id=?`, r.actor.TenantID, r.actor.StoreID, r.queryID).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return req, result, nil
	}
	if err != nil {
		return req, result, err
	}
	if fiscalcore.DecodeInput([]byte(body), &result) != nil {
		return req, result, ErrIntegrity
	}
	canonical, _ = json.Marshal(result)
	if string(canonical) != body || result.OperationID != r.queryID || !validQueryResult(result) {
		return req, result, ErrIntegrity
	}
	return req, result, nil
}

func (r *HomologationQueryRecorder) Read(ctx context.Context) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
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

func (r *HomologationQueryRecorder) Reconcile(ctx context.Context) (fiscalcore.ConsultationEvidence, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.ConsultationEvidence{}, err
	}
	defer tx.Rollback()
	if err = queryReadPermissionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return fiscalcore.ConsultationEvidence{}, err
	}
	original, _, err := originalTransmissionTx(ctx, tx, r.actor, r.intentID)
	if err != nil {
		return fiscalcore.ConsultationEvidence{}, err
	}
	req, result, err := r.readTx(ctx, tx)
	if err != nil {
		return fiscalcore.ConsultationEvidence{}, err
	}
	out, err := fiscalcore.ReconcileHomologationNFe(original, req, result)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
