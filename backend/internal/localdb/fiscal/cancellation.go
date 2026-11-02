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

// HomologationCancellationRecorder does NOT refund money, change the original
// sale or release stock/numbers. Only an explicit owner/manager decision may
// reserve one cancellation110111/sequence1; an uncertain event is never retried.
type HomologationCancellationRecorder struct {
	db                                                                                       *sql.DB
	license                                                                                  *entitlementstore.Store
	actor                                                                                    identity.Scope
	device                                                                                   identity.DeviceContext
	intentID, queryID, operationID, endpoint, approval, token, scope, key, authorizationHash string
}

var _ fiscalcore.CancellationRecorder = (*HomologationCancellationRecorder)(nil)

func NewHomologationCancellationRecorder(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, intentID, queryID, operationID, endpoint, approval, token string) (*HomologationCancellationRecorder, error) {
	u, err := url.Parse(endpoint)
	if db == nil || license == nil || !idPattern.MatchString(intentID) || !idPattern.MatchString(queryID) || !idPattern.MatchString(operationID) || strings.TrimSpace(approval) == "" || len(approval) > 128 || err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || len(endpoint) > 4096 || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".gov.br") {
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
	proof, hash, err := cancellationAuthorizationTx(ctx, tx, a, d, intentID, queryID, token, false)
	if err != nil {
		return nil, err
	}
	if operationID == proof.OriginalOperationID || operationID == queryID {
		return nil, ErrConflict
	}
	encoded, _ := json.Marshal([]string{a.TenantID, a.StoreID, intentID, queryID, operationID, hash, approval})
	sum := sha256.Sum256(encoded)
	r := &HomologationCancellationRecorder{db: db, license: license, actor: a, device: d, intentID: intentID, queryID: queryID, operationID: operationID, endpoint: endpoint, approval: approval, token: token, scope: "fiscal-cancel:" + hex.EncodeToString(sum[:]), key: proof.Key, authorizationHash: hash}
	return r, tx.Commit()
}
func (r *HomologationCancellationRecorder) Scope() string { return r.scope }
func (r *HomologationCancellationRecorder) Key() string   { return r.key }

func cancellationAuthorizationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, intentID, queryID, token string, latest bool) (fiscalcore.ConsultationEvidence, string, error) {
	var empty fiscalcore.ConsultationEvidence
	original, _, err := originalTransmissionTx(ctx, tx, a, intentID)
	if err != nil {
		return empty, "", err
	}
	var raw string
	if err = tx.QueryRowContext(ctx, `SELECT request_json FROM fiscal_query_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND query_id=?`, a.TenantID, a.StoreID, intentID, queryID).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return empty, "", ErrNotFound
	} else if err != nil {
		return empty, "", err
	}
	var req fiscalcore.RecordedRequest
	if fiscalcore.DecodeInput([]byte(raw), &req) != nil {
		return empty, "", ErrIntegrity
	}
	encoded, _ := json.Marshal([]string{a.TenantID, a.StoreID, intentID, queryID, original.RequestSHA256})
	sum := sha256.Sum256(encoded)
	reader := &HomologationQueryRecorder{actor: a, device: d, intentID: intentID, queryID: queryID, endpoint: req.URL, token: token, scope: "fiscal-query:" + hex.EncodeToString(sum[:]), key: original.Key, originalHash: original.RequestSHA256}
	request, result, err := reader.readTx(ctx, tx)
	if err != nil {
		return empty, "", err
	}
	proof, err := fiscalcore.ReconcileHomologationNFe(original, request, result)
	if err != nil || proof.Disposition != "authorized_indicated_homologation" || proof.Protocol == nil || len(proof.ProcessedXML) == 0 {
		return empty, "", ErrConflict
	}
	if latest {
		var newest string
		if err = tx.QueryRowContext(ctx, `SELECT query_id FROM fiscal_query_requests WHERE tenant_id=? AND store_id=? AND intent_id=? ORDER BY rowid DESC LIMIT 1`, a.TenantID, a.StoreID, intentID).Scan(&newest); err != nil {
			return empty, "", err
		}
		if newest != queryID {
			return empty, "", ErrConflict
		}
	}
	return proof, result.SHA256, nil
}
func (r *HomologationCancellationRecorder) sendPermissionTx(ctx context.Context, tx *sql.Tx) (fiscalcore.ConsultationEvidence, error) {
	var empty fiscalcore.ConsultationEvidence
	if err := queryReadPermissionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return empty, err
	}
	if err := r.license.RequireTx(ctx, tx, r.actor, r.device, identity.CancelSale, modules.Fiscal); err != nil {
		return empty, err
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_restore_barriers WHERE tenant_id=?`, r.actor.TenantID).Scan(&n); err != nil {
		return empty, err
	}
	if n != 0 {
		return empty, ErrRestored
	}
	proof, hash, err := cancellationAuthorizationTx(ctx, tx, r.actor, r.device, r.intentID, r.queryID, r.token, true)
	if err != nil {
		return empty, err
	}
	if hash != r.authorizationHash || proof.Key != r.key {
		return empty, ErrIntegrity
	}
	return proof, nil
}
func (r *HomologationCancellationRecorder) validateInput(in fiscalcore.Cancellation, proof fiscalcore.ConsultationEvidence) error {
	registered, err := time.Parse(time.RFC3339, proof.Protocol.ReceivedAt)
	if err != nil || in.Environment != 2 || in.Key != r.key || in.CNPJ != r.key[6:20] || in.Protocol != proof.Protocol.Number || in.Time.IsZero() || in.Time.Before(registered) || in.Time.After(time.Now().Add(5*time.Minute)) {
		return ErrInvalid
	}
	if _, err := fiscalcore.BuildCancellation(in); err != nil {
		return err
	}
	return nil
}
func (r *HomologationCancellationRecorder) ValidateCancellation(ctx context.Context, in fiscalcore.Cancellation) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	proof, err := r.sendPermissionTx(ctx, tx)
	if err != nil {
		return err
	}
	if err = r.validateInput(in, proof); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *HomologationCancellationRecorder) Reserve(ctx context.Context, in fiscalcore.RecordedRequest) error {
	original, err := fiscalcore.OriginalCancellationFromRequest(in)
	if err != nil {
		return ErrInvalid
	}
	if in.OperationID != r.operationID || in.Scope != r.scope || in.URL != r.endpoint || in.Key != r.key {
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
	proof, err := r.sendPermissionTx(ctx, tx)
	if err != nil {
		return err
	}
	if err = r.validateInput(original.Input, proof); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_cancellation_requests WHERE tenant_id=? AND store_id=? AND (intent_id=? OR operation_id=?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.operationID).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fiscalcore.ErrRecorded
	}
	raw, _ := json.Marshal(in)
	session := sha256.Sum256([]byte(r.token))
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_cancellation_requests VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.queryID, r.operationID, r.device.DeviceID, r.actor.IdentityID, session[:], r.authorizationHash, r.approval, string(raw), in.RequestSHA256, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *HomologationCancellationRecorder) BeforeSend(ctx context.Context, operationID string) error {
	if operationID != r.operationID {
		return ErrConflict
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = r.sendPermissionTx(ctx, tx); err != nil {
		return err
	}
	if _, _, err = r.readTx(ctx, tx); err != nil {
		return err
	}
	var actor, device string
	var original []byte
	if err = tx.QueryRowContext(ctx, `SELECT actor_id,device_id,session_sha256 FROM fiscal_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&actor, &device, &original); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(r.token))
	if actor != r.actor.IdentityID || device != r.device.DeviceID || subtle.ConstantTimeCompare(original, sum[:]) != 1 {
		return localauth.ErrDenied
	}
	return tx.Commit()
}
func (r *HomologationCancellationRecorder) Complete(ctx context.Context, in fiscalcore.ExchangeResult) error {
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
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_cancellation_results WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&previous)
	if err == nil {
		if previous != string(raw) {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_cancellation_results VALUES(?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *HomologationCancellationRecorder) readTx(ctx context.Context, tx *sql.Tx) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	var req fiscalcore.RecordedRequest
	result := fiscalcore.ExchangeResult{OperationID: r.operationID, State: "uncertain"}
	var raw, hash, query, auth, approval, operation string
	err := tx.QueryRowContext(ctx, `SELECT request_json,request_sha256,query_id,authorization_sha256,approval_reference,operation_id FROM fiscal_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&raw, &hash, &query, &auth, &approval, &operation)
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
	original, err := fiscalcore.OriginalCancellationFromRequest(req)
	if err != nil || string(canonical) != raw || hash != req.RequestSHA256 || query != r.queryID || auth != r.authorizationHash || approval != r.approval || operation != r.operationID || req.OperationID != r.operationID || req.Scope != r.scope || req.URL != r.endpoint || req.Key != r.key || original.Input.Key != r.key {
		return req, result, ErrIntegrity
	}
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_cancellation_results WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&raw)
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
func (r *HomologationCancellationRecorder) Read(ctx context.Context) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	defer tx.Rollback()
	if err = queryReadPermissionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	// Recovery is keyed by the original intent, never the caller's new ID.
	// Private send/callback paths above still require their exact reservation.
	var query, operation, authorization, approval, raw string
	err = tx.QueryRowContext(ctx, `SELECT query_id,operation_id,authorization_sha256,approval_reference,request_json FROM fiscal_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&query, &operation, &authorization, &approval, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, ErrNotFound
	}
	if err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	var stored fiscalcore.RecordedRequest
	if fiscalcore.DecodeInput([]byte(raw), &stored) != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, ErrIntegrity
	}
	proof, originalHash, err := cancellationAuthorizationTx(ctx, tx, r.actor, r.device, r.intentID, query, r.token, false)
	if err != nil || originalHash != authorization || proof.Key != r.key {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, ErrIntegrity
	}
	encoded, _ := json.Marshal([]string{r.actor.TenantID, r.actor.StoreID, r.intentID, query, operation, authorization, approval})
	sum := sha256.Sum256(encoded)
	view := *r
	view.queryID = query
	view.operationID = operation
	view.authorizationHash = authorization
	view.approval = approval
	view.endpoint = stored.URL
	view.scope = "fiscal-cancel:" + hex.EncodeToString(sum[:])
	req, result, err := view.readTx(ctx, tx)
	if err != nil {
		return req, result, err
	}
	archived, err := fiscalcore.OriginalCancellationFromRequest(req)
	if err != nil || archived.Input.Protocol != proof.Protocol.Number {
		return req, result, ErrIntegrity
	}
	return req, result, tx.Commit()
}
