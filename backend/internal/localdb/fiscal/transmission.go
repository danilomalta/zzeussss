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
	"strconv"
	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

var ErrRestored = errors.New("restored fiscal installation requires reconciliation before transmission")

// HomologationRecorder connects the technical TLS client's write-ahead journal
// to the SAME SQLite database as the sale/profile/intent. It does not classify
// taxes, allocate numbers, authorize a document or make the HTTP API emit.
// It is intentionally unavailable for production. Only a trusted host may
// provision endpoint and access key; no renderer endpoint exposes this type.
// A readable original request is permanently a no-POST barrier, not a retry.
type HomologationRecorder struct {
	db                                    *sql.DB
	license                               *entitlementstore.Store
	actor                                 identity.Scope
	device                                identity.DeviceContext
	intentID, endpoint, key, token, scope string
}

var _ fiscalcore.Recorder = (*HomologationRecorder)(nil)

func NewHomologationRecorder(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, intentID, endpoint, key, token string) (*HomologationRecorder, error) {
	u, err := url.Parse(endpoint)
	if db == nil || license == nil || !idPattern.MatchString(intentID) || !fiscalcore.ValidAccessKey(key) || err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || len(endpoint) > 4096 || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".gov.br") {
		return nil, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return nil, err
	}
	if _, err = readPermission(ctx, tx, a, d); err != nil {
		return nil, err
	}
	intent, err := intentTx(ctx, tx, a, intentID)
	if err != nil {
		return nil, err
	}
	if intent.ActorID != a.IdentityID || intent.DeviceID != d.DeviceID {
		return nil, ErrNotFound
	}
	p := intent.Snapshot.Profile
	if p.Environment != "homologation" || key[:2] != stateCodes[p.State] || key[6:20] != p.CNPJ || key[20:22] != p.Model || key[22:25] != formatSeries(p.Series) {
		return nil, ErrInvalid
	}
	identityBytes, _ := json.Marshal([]string{a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, intentID})
	digest := sha256.Sum256(identityBytes)
	r := &HomologationRecorder{db: db, license: license, actor: a, device: d, intentID: intentID, endpoint: endpoint, key: key, token: token, scope: "fiscal-homolog:" + hex.EncodeToString(digest[:])}
	return r, tx.Commit()
}
func formatSeries(series int) string {
	return strings.Repeat("0", 3-len(strconv.Itoa(series))) + strconv.Itoa(series)
}
func (r *HomologationRecorder) Scope() string { return r.scope }

func (r *HomologationRecorder) Reserve(ctx context.Context, in fiscalcore.RecordedRequest) error {
	digest := sha256.Sum256(in.Body)
	if !idPattern.MatchString(in.OperationID) || in.Scope != r.scope || in.URL != r.endpoint || in.Key != r.key || in.Environment != "2" || in.Issuer != r.key[6:20] || len(in.Body) == 0 || len(in.Body) > 2*fiscalcore.MaxXML || in.RequestSHA256 != hex.EncodeToString(digest[:]) {
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
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_transmission_requests WHERE tenant_id=? AND ((store_id=? AND intent_id=?) OR (device_id=? AND operation_id=?) OR (substr(access_key,7,14)=? AND substr(access_key,21,2)=? AND substr(access_key,23,3)=? AND substr(access_key,26,9)=?))`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.device.DeviceID, in.OperationID, r.key[6:20], r.key[20:22], r.key[22:25], r.key[25:34]).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return fiscalcore.ErrRecorded
	}
	body, _ := json.Marshal(in)
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_transmission_requests(tenant_id,store_id,intent_id,device_id,actor_id,operation_id,access_key,request_json,request_sha256,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.device.DeviceID, r.actor.IdentityID, in.OperationID, r.key, string(body), in.RequestSHA256, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Complete deliberately rechecks no human session: a POST already sent may
// finish after logout/cancellation. Persist its proof, without new authority.
// An identical callback is idempotent; a different result cannot overwrite it.
func (r *HomologationRecorder) Complete(ctx context.Context, in fiscalcore.ExchangeResult) error {
	if !idPattern.MatchString(in.OperationID) || len(in.Body) > 2*fiscalcore.MaxXML || (in.HTTPStatus != 0 && (in.HTTPStatus < 100 || in.HTTPStatus > 599)) {
		return ErrInvalid
	}
	switch in.State {
	case "uncertain":
		if len(in.Body) != 0 || in.SHA256 != "" {
			return ErrInvalid
		}
	case "response_received":
		sum := sha256.Sum256(in.Body)
		if in.HTTPStatus == 0 || in.SHA256 != hex.EncodeToString(sum[:]) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	req, _, err := r.readTx(ctx, tx)
	if err != nil {
		return err
	}
	if in.OperationID != req.OperationID {
		return ErrConflict
	}
	body, _ := json.Marshal(in)
	var old string
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_transmission_results WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&old)
	if err == nil {
		if old != string(body) {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_transmission_results VALUES(?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, string(body), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *HomologationRecorder) readTx(ctx context.Context, tx *sql.Tx) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	var req fiscalcore.RecordedRequest
	var result fiscalcore.ExchangeResult
	var body, op, digest, actor, device, accessKey string
	err := tx.QueryRowContext(ctx, `SELECT request_json,operation_id,request_sha256,actor_id,device_id,access_key FROM fiscal_transmission_requests WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&body, &op, &digest, &actor, &device, &accessKey)
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
	if string(canonical) != body || !idPattern.MatchString(op) || len(req.Body) == 0 || len(req.Body) > 2*fiscalcore.MaxXML || req.Issuer != r.key[6:20] || accessKey != req.Key || req.OperationID != op || req.Scope != r.scope || req.URL != r.endpoint || req.Key != r.key || req.Environment != "2" || digest != req.RequestSHA256 || digest != hex.EncodeToString(sum[:]) || actor != r.actor.IdentityID || device != r.device.DeviceID {
		return req, result, ErrIntegrity
	}
	result = fiscalcore.ExchangeResult{OperationID: op, State: "uncertain"}
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_transmission_results WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&body)
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
	sum = sha256.Sum256(result.Body)
	if string(canonical) != body || len(result.Body) > 2*fiscalcore.MaxXML || (result.HTTPStatus != 0 && (result.HTTPStatus < 100 || result.HTTPStatus > 599)) || result.OperationID != op || (result.State != "uncertain" && result.State != "response_received") || (result.State == "response_received" && (result.HTTPStatus < 100 || result.HTTPStatus > 599 || result.SHA256 != hex.EncodeToString(sum[:]))) || (result.State == "uncertain" && (len(result.Body) != 0 || result.SHA256 != "")) {
		return req, result, ErrIntegrity
	}
	return req, result, nil
}

// Read requires the current exact session and current actor/device permission.
// It returns original bytes/IDs, never initiates HTTP and never calls Reserve.
func (r *HomologationRecorder) Read(ctx context.Context) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	defer tx.Rollback()
	if err = fiscalSessionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	if _, err = readPermission(ctx, tx, r.actor, r.device); err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	req, result, err := r.readTx(ctx, tx)
	if err != nil {
		return req, result, err
	}
	return req, result, tx.Commit()
}

// BeforeSend is invoked by fiscalcore.Client after durable reservation and
// immediately before HTTP I/O. It narrows the reservation-to-send auth window;
// it cannot revoke a request already handed to the network retroactively.
func (r *HomologationRecorder) BeforeSend(ctx context.Context, operationID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = r.sendPermissionTx(ctx, tx); err != nil {
		return err
	}
	req, _, err := r.readTx(ctx, tx)
	if err != nil {
		return err
	}
	if req.OperationID != operationID {
		return ErrConflict
	}
	return tx.Commit()
}
func (r *HomologationRecorder) sendPermissionTx(ctx context.Context, tx *sql.Tx) error {
	var err error
	if err = fiscalSessionTx(ctx, tx, r.actor, r.device, r.token); err != nil {
		return err
	}
	sessionDigest := sha256.Sum256([]byte(r.token))
	var originalSession []byte
	if err = tx.QueryRowContext(ctx, `SELECT session_sha256 FROM fiscal_intent_session_bindings WHERE tenant_id=? AND store_id=? AND intent_id=?`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&originalSession); errors.Is(err, sql.ErrNoRows) {
		return localauth.ErrDenied
	} else if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(sessionDigest[:], originalSession) != 1 {
		return localauth.ErrDenied
	}
	if err = r.license.RequireTx(ctx, tx, r.actor, r.device, identity.Sell, modules.Fiscal); err != nil {
		return err
	}
	intent, err := intentTx(ctx, tx, r.actor, r.intentID)
	if err != nil {
		return err
	}
	if intent.ActorID != r.actor.IdentityID || intent.DeviceID != r.device.DeviceID || r.key[6:20] != intent.Snapshot.Profile.CNPJ || intent.Snapshot.Profile.Environment != "homologation" {
		return ErrInvalid
	}
	p, err := profileTx(ctx, tx, r.actor, intent.Input.ProfileID, 0)
	if err != nil {
		return err
	}
	if p.Revision != intent.Input.ProfileRevision {
		return ErrConflict
	}
	var n int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_restore_barriers WHERE tenant_id=?`, r.actor.TenantID).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return ErrRestored
	}
	return nil
}
