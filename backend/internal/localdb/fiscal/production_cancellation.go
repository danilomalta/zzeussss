package fiscal

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

// ProductionCancellationRecorder does NOT refund money, change the original
// sale or release stock/numbers. Only an explicit owner/manager decision may
// reserve one cancellation110111/sequence1; an uncertain event is never retried.
type ProductionCancellationRecorder struct {
	binding                                                                                  fiscalcore.ProductionBinding
	db                                                                                       *sql.DB
	license                                                                                  *entitlementstore.Store
	actor                                                                                    identity.Scope
	device                                                                                   identity.DeviceContext
	intentID, queryID, operationID, endpoint, approval, token, scope, key, authorizationHash string
}

var _ fiscalcore.CancellationRecorder = (*ProductionCancellationRecorder)(nil)

func NewProductionCancellationRecorder(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, intentID, queryID, operationID, approval, token string, binding fiscalcore.ProductionBinding) (*ProductionCancellationRecorder, error) {
	endpoint := binding.CancellationURL
	if db == nil || license == nil || !binding.Valid(time.Now()) || !fiscalcore.ProductionEndpoint(endpoint) || !idPattern.MatchString(intentID) || !idPattern.MatchString(queryID) || !idPattern.MatchString(operationID) || len(approval) < 8 || len(approval) > 128 || strings.TrimSpace(approval) != approval || strings.ContainsAny(approval, "\r\n\t\x00") {
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
	if _, err = productionIntentTx(ctx, tx, a, d, intentID, token, false, binding); err != nil {
		return nil, err
	}
	proof, hash, err := productionCancellationAuthorizationTx(ctx, tx, a, d, intentID, queryID, token, false)
	if err != nil {
		return nil, err
	}
	if operationID == proof.OriginalOperationID || operationID == queryID {
		return nil, ErrConflict
	}
	encoded, _ := json.Marshal([]string{a.TenantID, a.StoreID, intentID, queryID, operationID, hash, approval})
	sum := sha256.Sum256(encoded)
	r := &ProductionCancellationRecorder{binding: binding, db: db, license: license, actor: a, device: d, intentID: intentID, queryID: queryID, operationID: operationID, endpoint: endpoint, approval: approval, token: token, scope: "fiscal-production-cancel:" + hex.EncodeToString(sum[:]), key: proof.Key, authorizationHash: hash}
	return r, tx.Commit()
}
func (r *ProductionCancellationRecorder) Scope() string { return r.scope }
func (r *ProductionCancellationRecorder) Key() string   { return r.key }

func productionCancellationAuthorizationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, intentID, queryID, token string, latest bool) (fiscalcore.ConsultationEvidence, string, error) {
	var empty fiscalcore.ConsultationEvidence
	if _, err := productionIntentTx(ctx, tx, a, d, intentID, token, false, fiscalcore.ProductionBinding{}); err != nil {
		return empty, "", err
	}
	var rev int
	var operation, device string
	if err := tx.QueryRowContext(ctx, `SELECT revision,operation_id,device_id FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND kind='issue' ORDER BY revision DESC LIMIT 1`, a.TenantID, a.StoreID, intentID).Scan(&rev, &operation, &device); err != nil {
		return empty, "", ErrConflict
	}
	original, _, err := productionRecordTx(ctx, tx, a, device, operation)
	if err != nil {
		return empty, "", err
	}
	var qdev string
	if err = tx.QueryRowContext(ctx, `SELECT device_id FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=? AND kind='query' AND operation_id=?`, a.TenantID, a.StoreID, intentID, rev, queryID).Scan(&qdev); err != nil {
		return empty, "", ErrConflict
	}
	if latest {
		var op string
		if err = tx.QueryRowContext(ctx, `SELECT operation_id FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=? AND kind='query' ORDER BY rowid DESC LIMIT 1`, a.TenantID, a.StoreID, intentID, rev).Scan(&op); err != nil || op != queryID {
			return empty, "", ErrConflict
		}
	}
	query, result, err := productionRecordTx(ctx, tx, a, qdev, queryID)
	if err != nil {
		return empty, "", err
	}
	proof, err := fiscalcore.ReconcileProductionNFe(original, query, result)
	if err != nil || proof.Disposition != "authorized_indicated_production" || proof.Protocol == nil {
		return empty, "", ErrConflict
	}
	return proof, result.SHA256, nil
}
func (r *ProductionCancellationRecorder) sendPermissionTx(ctx context.Context, tx *sql.Tx) (fiscalcore.ConsultationEvidence, error) {
	var empty fiscalcore.ConsultationEvidence
	if !r.binding.Valid(time.Now()) || r.binding.TenantID != r.actor.TenantID || r.binding.StoreID != r.actor.StoreID || r.binding.DeviceID != r.device.DeviceID || r.binding.CancellationURL != r.endpoint || r.key[6:20] != r.binding.Issuer || r.key[20:22] != r.binding.Model {
		return empty, ErrInvalid
	}
	intent, err := productionIntentTx(ctx, tx, r.actor, r.device, r.intentID, r.token, false, r.binding)
	if err != nil {
		return empty, err
	}
	if intent.Snapshot.Profile.State != r.binding.UF || intent.Snapshot.Profile.Series != r.binding.Series || intent.Snapshot.Profile.Model != r.binding.Model {
		return empty, ErrInvalid
	}
	if err := queryReadPermissionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return empty, err
	}
	if err := r.license.RequireTx(ctx, tx, r.actor, r.device, identity.CancelSale, modules.Fiscal); err != nil {
		return empty, err
	}
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, r.actor.TenantID, r.actor.IdentityID).Scan(&role); err != nil {
		return empty, err
	}
	if role != "owner" && role != "manager" {
		return empty, identity.ErrDenied
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_restore_barriers WHERE tenant_id=?`, r.actor.TenantID).Scan(&n); err != nil {
		return empty, err
	}
	if n != 0 {
		return empty, ErrRestored
	}
	proof, hash, err := productionCancellationAuthorizationTx(ctx, tx, r.actor, r.device, r.intentID, r.queryID, r.token, true)
	if err != nil {
		return empty, err
	}
	if hash != r.authorizationHash || proof.Key != r.key {
		return empty, ErrIntegrity
	}
	return proof, nil
}
func (r *ProductionCancellationRecorder) validateInput(in fiscalcore.Cancellation, proof fiscalcore.ConsultationEvidence) error {
	registered, err := time.Parse(time.RFC3339, proof.Protocol.ReceivedAt)
	if err != nil || in.Environment != 1 || in.Key != r.key || in.CNPJ != r.key[6:20] || in.Protocol != proof.Protocol.Number || in.Time.IsZero() || in.Time.Before(registered) || in.Time.After(time.Now().Add(5*time.Minute)) {
		return ErrInvalid
	}
	if _, err := fiscalcore.BuildCancellation(in); err != nil {
		return err
	}
	return nil
}
func (r *ProductionCancellationRecorder) ValidateCancellation(ctx context.Context, in fiscalcore.Cancellation) error {
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
func (r *ProductionCancellationRecorder) Reserve(ctx context.Context, in fiscalcore.RecordedRequest) error {
	original, err := fiscalcore.OriginalProductionCancellation(in)
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
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_production_cancellation_requests WHERE tenant_id=? AND store_id=? AND (intent_id=? OR operation_id=?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.operationID).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fiscalcore.ErrRecorded
	}
	raw, _ := json.Marshal(in)
	session := sha256.Sum256([]byte(r.token))
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_production_cancellation_requests VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.queryID, r.operationID, r.device.DeviceID, r.actor.IdentityID, session[:], r.authorizationHash, r.approval, string(raw), in.RequestSHA256, string(mustProductionJSON(r.binding)), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *ProductionCancellationRecorder) BeforeSend(ctx context.Context, operationID string) error {
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
	if err = tx.QueryRowContext(ctx, `SELECT actor_id,device_id,session_sha256 FROM fiscal_production_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&actor, &device, &original); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(r.token))
	if actor != r.actor.IdentityID || device != r.device.DeviceID || subtle.ConstantTimeCompare(original, sum[:]) != 1 {
		return localauth.ErrDenied
	}
	return tx.Commit()
}
func (r *ProductionCancellationRecorder) Complete(ctx context.Context, in fiscalcore.ExchangeResult) error {
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
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_production_cancellation_results WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&previous)
	if err == nil {
		if previous != string(raw) {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_production_cancellation_results VALUES(?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *ProductionCancellationRecorder) readTx(ctx context.Context, tx *sql.Tx) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	var req fiscalcore.RecordedRequest
	result := fiscalcore.ExchangeResult{OperationID: r.operationID, State: "uncertain"}
	var raw, hash, query, auth, approval, operation, configuration, created string
	err := tx.QueryRowContext(ctx, `SELECT request_json,request_sha256,query_id,authorization_sha256,approval_reference,operation_id,config_json,created_at FROM fiscal_production_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&raw, &hash, &query, &auth, &approval, &operation, &configuration, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return req, result, ErrNotFound
	}
	if err != nil {
		return req, result, err
	}
	if fiscalcore.DecodeInput([]byte(raw), &req) != nil {
		return req, result, ErrIntegrity
	}
	var binding fiscalcore.ProductionBinding
	at, e := time.Parse(time.RFC3339Nano, created)
	if e != nil || fiscalcore.DecodeInput([]byte(configuration), &binding) != nil || !binding.Valid(at) || binding.TenantID != r.actor.TenantID || binding.StoreID != r.actor.StoreID || binding.CancellationURL != req.URL || binding.Issuer != req.Issuer {
		return req, result, ErrIntegrity
	}
	canonical, _ := json.Marshal(req)
	original, err := fiscalcore.OriginalProductionCancellation(req)
	if err != nil || fiscalcore.MatchProductionCancellation(original.Input, original.SignedXML, binding.CertificateSHA256) != nil || string(canonical) != raw || hash != req.RequestSHA256 || query != r.queryID || auth != r.authorizationHash || approval != r.approval || operation != r.operationID || req.OperationID != r.operationID || req.Scope != r.scope || req.URL != r.endpoint || req.Key != r.key || original.Input.Key != r.key {
		return req, result, ErrIntegrity
	}
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_production_cancellation_results WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&raw)
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
func (r *ProductionCancellationRecorder) Read(ctx context.Context) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	defer tx.Rollback()
	if err = queryReadPermissionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	if _, err = productionIntentTx(ctx, tx, r.actor, r.device, r.intentID, r.token, false, fiscalcore.ProductionBinding{}); err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	// Recovery is keyed by the original intent, never the caller's new ID.
	// Private send/callback paths above still require their exact reservation.
	var query, operation, authorization, approval, raw string
	err = tx.QueryRowContext(ctx, `SELECT query_id,operation_id,authorization_sha256,approval_reference,request_json FROM fiscal_production_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&query, &operation, &authorization, &approval, &raw)
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
	proof, originalHash, err := productionCancellationAuthorizationTx(ctx, tx, r.actor, r.device, r.intentID, query, r.token, false)
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
	view.scope = "fiscal-production-cancel:" + hex.EncodeToString(sum[:])
	req, result, err := view.readTx(ctx, tx)
	if err != nil {
		return req, result, err
	}
	archived, err := fiscalcore.OriginalProductionCancellation(req)
	if err != nil || archived.Input.Protocol != proof.Protocol.Number {
		return req, result, ErrIntegrity
	}
	return req, result, tx.Commit()
}

func (r *ProductionCancellationRecorder) ValidateProductionCancellation(ctx context.Context, in fiscalcore.Cancellation, fingerprint, endpoint string) error {
	if fingerprint != r.binding.CertificateSHA256 || endpoint != r.binding.CancellationURL {
		return ErrInvalid
	}
	return r.ValidateCancellation(ctx, in)
}

// ReadProductionCancellation reconstructs immutable evidence without a live key.
func ReadProductionCancellation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id, token string) (fiscalcore.CancellationEvidence, error) {
	r := &ProductionCancellationRecorder{db: db, actor: a, device: d, intentID: id, token: token}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	if _, err = productionIntentTx(ctx, tx, a, d, id, token, false, fiscalcore.ProductionBinding{}); err != nil {
		tx.Rollback()
		return fiscalcore.CancellationEvidence{}, err
	}
	if err = tx.Commit(); err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT request_json FROM fiscal_production_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, a.TenantID, a.StoreID, id).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return fiscalcore.CancellationEvidence{}, ErrNotFound
	} else if err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	var req fiscalcore.RecordedRequest
	if fiscalcore.DecodeInput([]byte(raw), &req) != nil {
		return fiscalcore.CancellationEvidence{}, ErrIntegrity
	}
	r.key = req.Key
	original, result, err := r.Read(ctx)
	if err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	out, err := fiscalcore.ReconcileProductionCancellation(original, result)
	if out.State != "cancellation_indicated_production" {
		// Query evidence is independent: preserve the uncertain original exchange and
		// identify exactly which newer query supplied the event registration.
		if recovered, e := recoverProductionCancellationQuery(ctx, db, a, d, id, token, original); e == nil {
			recovered.Exchange = result
			return recovered, nil
		}
	}

	if err != nil && out.Exchange.OperationID != "" {
		out.State = "uncertain"
		return out, nil
	}
	return out, err
}

func (r *ProductionCancellationRecorder) Input(ctx context.Context, reason string) (fiscalcore.Cancellation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.Cancellation{}, err
	}
	defer tx.Rollback()
	proof, err := r.sendPermissionTx(ctx, tx)
	if err != nil {
		return fiscalcore.Cancellation{}, err
	}
	zone, err := time.LoadLocation(r.binding.Timezone)
	if err != nil {
		return fiscalcore.Cancellation{}, ErrInvalid
	}
	in := fiscalcore.Cancellation{Key: proof.Key, CNPJ: proof.Key[6:20], Environment: 1, Time: time.Now().In(zone).Truncate(time.Second), Protocol: proof.Protocol.Number, Reason: reason}
	if err = r.validateInput(in, proof); err != nil {
		return in, err
	}
	return in, tx.Commit()
}

func CheckProductionCancellationReplay(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id, token, operation, query, reason, approval string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = productionIntentTx(ctx, tx, a, d, id, token, false, fiscalcore.ProductionBinding{}); err != nil {
		return err
	}
	var raw, oldQuery, oldApproval string
	if err = tx.QueryRowContext(ctx, `SELECT request_json,query_id,approval_reference FROM fiscal_production_cancellation_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, a.TenantID, a.StoreID, id).Scan(&raw, &oldQuery, &oldApproval); err != nil {
		return err
	}
	var req fiscalcore.RecordedRequest
	if fiscalcore.DecodeInput([]byte(raw), &req) != nil {
		return ErrIntegrity
	}
	original, err := fiscalcore.OriginalProductionCancellation(req)
	if err != nil {
		return ErrIntegrity
	}
	if req.OperationID != operation || oldQuery != query || oldApproval != approval || original.Input.Reason != reason {
		return ErrConflict
	}
	return tx.Commit()
}

func recoverProductionCancellationQuery(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id, token string, event fiscalcore.RecordedRequest) (fiscalcore.CancellationEvidence, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	defer tx.Rollback()
	if _, err = productionIntentTx(ctx, tx, a, d, id, token, false, fiscalcore.ProductionBinding{}); err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	var operation, device string
	var revision int
	if err = tx.QueryRowContext(ctx, `SELECT operation_id,device_id,revision FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND kind='issue' ORDER BY revision DESC LIMIT 1`, a.TenantID, a.StoreID, id).Scan(&operation, &device, &revision); err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	invoice, _, err := productionRecordTx(ctx, tx, a, device, operation)
	if err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT operation_id,device_id FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=? AND kind='query' ORDER BY rowid DESC LIMIT 1`, a.TenantID, a.StoreID, id, revision).Scan(&operation, &device); err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	query, result, err := productionRecordTx(ctx, tx, a, device, operation)
	if err != nil {
		return fiscalcore.CancellationEvidence{}, err
	}
	out, err := fiscalcore.ReconcileProductionCancellationQuery(event, invoice, query, result)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
