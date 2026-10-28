package fiscal

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type ProductionPreparation struct {
	Configuration       fiscalcore.ProductionBinding `json:"configuration"`
	Revision            int                          `json:"revision"`
	OperationID         string                       `json:"operation_id"`
	ConfigurationSHA256 string                       `json:"configuration_sha256"`
	ReviewSHA256        string                       `json:"review_sha256"`
	Invoice             fiscalcore.Invoice           `json:"invoice"`
	CreatedAt           string                       `json:"created_at"`
}
type ProductionStatus struct {
	Preparation      ProductionPreparation         `json:"preparation"`
	Approved         bool                          `json:"approved"`
	Evidence         fiscalcore.ProductionEvidence `json:"evidence"`
	EvidenceRevision int                           `json:"evidence_revision"`
}
type ProductionRecorder struct {
	db                           *sql.DB
	license                      *entitlementstore.Store
	actor                        identity.Scope
	device                       identity.DeviceContext
	intentID, token, kind, scope string
	revision                     int
	binding                      fiscalcore.ProductionBinding
}

func productionIntentTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id, token string, send bool, b fiscalcore.ProductionBinding) (Intent, error) {
	if err := fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return Intent{}, err
	}
	all, err := readPermission(ctx, tx, a, d)
	if err != nil {
		return Intent{}, err
	}
	intent, err := intentTx(ctx, tx, a, id)
	if err != nil {
		return Intent{}, err
	}
	if !all && (intent.ActorID != a.IdentityID || intent.DeviceID != d.DeviceID) {
		return Intent{}, ErrNotFound
	}
	if intent.Snapshot.Profile.Environment != "production" {
		return Intent{}, ErrInvalid
	}
	if send {
		if intent.ActorID != a.IdentityID || intent.DeviceID != d.DeviceID {
			return Intent{}, ErrNotFound
		}
		if !b.Valid(time.Now()) || b.TenantID != a.TenantID || b.StoreID != a.StoreID || b.DeviceID != d.DeviceID {
			return Intent{}, ErrInvalid
		}
		p, err := profileTx(ctx, tx, a, intent.Input.ProfileID, 0)
		if err != nil {
			return Intent{}, err
		}
		if p.Revision != intent.Input.ProfileRevision || b.ProfileID != p.Input.ProfileID || b.ProfileRevision != p.Revision || b.Issuer != p.Input.CNPJ || b.UF != p.Input.State || b.Model != p.Input.Model || b.Series != p.Input.Series {
			return Intent{}, ErrConflict
		}
		if err = numberBarrierTx(ctx, tx, a.TenantID); err != nil {
			return Intent{}, err
		}
	}
	return intent, nil
}

func productionPreparationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, revision int) (ProductionPreparation, error) {
	var out ProductionPreparation
	var raw, snapshot, configuration string
	err := tx.QueryRowContext(ctx, `SELECT p.revision,p.operation_id,p.config_sha256,p.config_json,p.invoice_json,p.invoice_sha256,p.created_at,i.snapshot_sha256 FROM fiscal_production_preparations p JOIN fiscal_intents i ON i.tenant_id=p.tenant_id AND i.store_id=p.store_id AND i.id=p.intent_id WHERE p.tenant_id=? AND p.store_id=? AND p.intent_id=? AND (p.revision=? OR ?=0) ORDER BY p.revision DESC LIMIT 1`, a.TenantID, a.StoreID, id, revision, revision).Scan(&out.Revision, &out.OperationID, &out.ConfigurationSHA256, &configuration, &raw, &out.ReviewSHA256, &out.CreatedAt, &snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if fiscalcore.DecodeInput([]byte(configuration), &out.Configuration) != nil || out.Configuration.SHA256() != out.ConfigurationSHA256 || out.Configuration.TenantID != a.TenantID || out.Configuration.StoreID != a.StoreID {
		return out, ErrIntegrity
	}
	if fiscalcore.DecodeInput([]byte(raw), &out.Invoice) != nil {
		return out, ErrIntegrity
	}
	canonical, _ := json.Marshal(out.Invoice)
	sum := sha256.Sum256([]byte(out.ConfigurationSHA256 + "\n" + snapshot + "\n" + raw))
	if string(canonical) != raw || out.ReviewSHA256 != hex.EncodeToString(sum[:]) || out.Invoice.Environment != 1 {
		return ProductionPreparation{}, ErrIntegrity
	}
	at, e := time.Parse(time.RFC3339Nano, out.CreatedAt)
	if e != nil || !out.Configuration.Valid(at) || out.Configuration.Issuer != out.Invoice.Key.CNPJ || out.Configuration.Model != out.Invoice.Key.Model || out.Configuration.Series != out.Invoice.Key.Series || out.Configuration.UF != out.Invoice.Key.UF {
		return out, ErrIntegrity
	}
	if _, _, err = fiscalcore.BuildInvoice(out.Invoice); err != nil {
		return ProductionPreparation{}, ErrIntegrity
	}
	return out, nil
}

func productionAllocationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, intent Intent, b fiscalcore.ProductionBinding) (fiscalcore.KeyInput, error) {
	var key string
	var number int64
	var issued string
	err := tx.QueryRowContext(ctx, `SELECT access_key,number,issued_at FROM fiscal_production_allocations WHERE tenant_id=? AND store_id=? AND intent_id=?`, a.TenantID, a.StoreID, intent.ID).Scan(&key, &number, &issued)
	if err == nil {
		t, e := time.Parse(time.RFC3339, issued)
		if e != nil || !fiscalcore.ValidAccessKey(key) {
			return fiscalcore.KeyInput{}, ErrIntegrity
		}
		in := fiscalcore.KeyInput{UF: b.UF, CNPJ: b.Issuer, Model: b.Model, Series: b.Series, Number: number, IssuedAt: t, Emission: 1, NumericCode: key[35:43]}
		expected, e := fiscalcore.AccessKey(in)
		if e != nil || expected != key {
			return fiscalcore.KeyInput{}, ErrIntegrity
		}
		return in, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fiscalcore.KeyInput{}, err
	}
	var tenant, store, profile, reference string
	var initial int64
	err = tx.QueryRowContext(ctx, `SELECT tenant_id,store_id,profile_id,initial_number,authority_reference FROM fiscal_production_series WHERE issuer=? AND model=? AND series=?`, b.Issuer, b.Model, b.Series).Scan(&tenant, &store, &profile, &initial, &reference)
	if errors.Is(err, sql.ErrNoRows) {
		var role string
		if e := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); e != nil {
			return fiscalcore.KeyInput{}, e
		}
		if role != "owner" {
			return fiscalcore.KeyInput{}, identity.ErrDenied
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_production_series VALUES(?,?,?,?,?,?,?,?,?,?)`, b.Issuer, b.Model, b.Series, a.TenantID, a.StoreID, b.ProfileID, b.InitialNumber, b.SoleAuthorityReference, a.IdentityID, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return fiscalcore.KeyInput{}, err
		}
	} else if err != nil {
		return fiscalcore.KeyInput{}, err
	} else if tenant != a.TenantID || store != a.StoreID || profile != b.ProfileID || initial != b.InitialNumber || reference != b.SoleAuthorityReference {
		return fiscalcore.KeyInput{}, ErrConflict
	}
	var last int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(max(number),0) FROM fiscal_production_allocations WHERE issuer=? AND model=? AND series=?`, b.Issuer, b.Model, b.Series).Scan(&last); err != nil {
		return fiscalcore.KeyInput{}, err
	}
	number = b.InitialNumber
	if last >= number {
		if last >= 999999999 {
			return fiscalcore.KeyInput{}, ErrNumberExhausted
		}
		number = last + 1
	}
	n, e := rand.Int(rand.Reader, big.NewInt(100000000))
	if e != nil {
		return fiscalcore.KeyInput{}, e
	}
	code := strconv.FormatInt(n.Int64(), 10)
	code = strings.Repeat("0", 8-len(code)) + code
	// Timezone is explicit in the issuer's public invoice input only after
	// numbering; use canonical UTC for the durable assignment and exact key.
	zone, e := time.LoadLocation(b.Timezone)
	if e != nil {
		return fiscalcore.KeyInput{}, ErrInvalid
	}
	in := fiscalcore.KeyInput{UF: b.UF, CNPJ: b.Issuer, Model: b.Model, Series: b.Series, Number: number, IssuedAt: time.Now().In(zone).Truncate(time.Second), Emission: 1, NumericCode: code}
	key, err = fiscalcore.AccessKey(in)
	if err != nil {
		return in, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_production_allocations VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, intent.ID, b.Issuer, b.Model, b.Series, number, key, in.IssuedAt.Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339Nano))
	return in, err
}

// Preparation is an explicit new review operation. Renewing a session never
// revives an old callback; a new revision/approval is required before any send.
// Any uncertain/authorized/denied/duplicate previous issue blocks correction.
func PrepareProduction(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, token, operationID string, expectedRevision int, in fiscalcore.Invoice, b fiscalcore.ProductionBinding) (ProductionPreparation, error) {
	var out ProductionPreparation
	if db == nil || license == nil || !idPattern.MatchString(operationID) || expectedRevision < 0 || in.Environment != 1 || in.Key != (fiscalcore.KeyInput{}) || !in.ContingencyAt.IsZero() || in.ContingencyReason != "" {
		return out, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = numberLockTx(ctx, tx); err != nil {
		return out, err
	}
	intent, err := productionIntentTx(ctx, tx, a, d, id, token, true, b)
	if err != nil {
		return out, err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.Sell, modules.Fiscal); err != nil {
		return out, err
	}
	old, err := productionPreparationTx(ctx, tx, a, id, 0)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return out, err
	}
	if err == nil && old.OperationID == operationID {
		candidate := in
		candidate.Key = old.Invoice.Key
		candidateRaw, _ := json.Marshal(candidate)
		oldRaw, _ := json.Marshal(old.Invoice)
		if old.Revision != expectedRevision+1 || old.ConfigurationSHA256 != b.SHA256() || !bytes.Equal(candidateRaw, oldRaw) {
			return out, ErrConflict
		}
		if err = productionSessionTx(ctx, tx, a, d, id, old.Revision, token); err != nil {
			return out, err
		}
		return old, tx.Commit()
	}
	if old.Revision != expectedRevision {
		return out, ErrConflict
	}
	if err = productionCorrectionAllowedTx(ctx, tx, a, id); err != nil {
		return out, err
	}
	in.Key, err = productionAllocationTx(ctx, tx, a, d, intent, b)
	if err != nil {
		return out, err
	}
	key, err := fiscalcore.AccessKey(in.Key)
	if err != nil {
		return out, err
	}
	if err = validateInvoiceSnapshot(in, intent.Snapshot, key, 1); err != nil {
		return out, err
	}
	if _, _, err = fiscalcore.BuildInvoice(in); err != nil {
		return out, err
	}
	raw, _ := json.Marshal(in)
	configuration := b.SHA256()
	sum := sha256.Sum256([]byte(configuration + "\n" + intent.SnapshotSHA256 + "\n" + string(raw)))
	session := sha256.Sum256([]byte(token))
	out = ProductionPreparation{Configuration: b, Revision: expectedRevision + 1, OperationID: operationID, ConfigurationSHA256: configuration, ReviewSHA256: hex.EncodeToString(sum[:]), Invoice: in, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_production_preparations VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, id, out.Revision, d.DeviceID, a.IdentityID, operationID, configuration, string(mustProductionJSON(b)), session[:], string(raw), out.ReviewSHA256, out.CreatedAt)
	if err != nil {
		return ProductionPreparation{}, err
	}
	return out, tx.Commit()
}

func productionSessionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string, revision int, token string) error {
	var digest []byte
	var actor, device string
	if err := tx.QueryRowContext(ctx, `SELECT session_sha256,actor_id,device_id FROM fiscal_production_preparations WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=?`, a.TenantID, a.StoreID, id, revision).Scan(&digest, &actor, &device); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(token))
	if actor != a.IdentityID || device != d.DeviceID || subtle.ConstantTimeCompare(sum[:], digest) != 1 {
		return identity.ErrDenied
	}
	return nil
}

func ApproveProduction(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, token, operationID string, revision int, reviewSHA, taxReference string, b fiscalcore.ProductionBinding) error {
	if db == nil || license == nil || !idPattern.MatchString(operationID) || len(taxReference) < 8 || len(taxReference) > 255 || strings.TrimSpace(taxReference) != taxReference || strings.ContainsAny(taxReference, "\r\n\t\x00") {
		return ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = numberLockTx(ctx, tx); err != nil {
		return err
	}
	intent, err := productionIntentTx(ctx, tx, a, d, id, token, false, b)
	if err != nil {
		return err
	}
	if !b.Valid(time.Now()) || b.TenantID != a.TenantID || b.StoreID != a.StoreID || b.DeviceID != d.DeviceID || intent.Input.ProfileID != b.ProfileID || intent.Input.ProfileRevision != b.ProfileRevision {
		return ErrConflict
	}
	if err = numberBarrierTx(ctx, tx, a.TenantID); err != nil {
		return err
	}
	if err = license.RequireTx(ctx, tx, a, d, identity.ViewAccounting, modules.Fiscal); err != nil {
		return err
	}
	var role string
	if err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
		return err
	}
	if role != "owner" && role != "accountant" {
		return identity.ErrDenied
	}
	prep, err := productionPreparationTx(ctx, tx, a, id, 0)
	if err != nil {
		return err
	}
	if prep.Revision != revision || prep.ReviewSHA256 != reviewSHA || prep.ConfigurationSHA256 != b.SHA256() {
		return ErrConflict
	}
	var oldSHA, ref, actor, device, op string
	err = tx.QueryRowContext(ctx, `SELECT invoice_sha256,tax_reference,actor_id,device_id,operation_id FROM fiscal_production_approvals WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=?`, a.TenantID, a.StoreID, id, revision).Scan(&oldSHA, &ref, &actor, &device, &op)
	if err == nil {
		if oldSHA != reviewSHA || ref != taxReference || actor != a.IdentityID || device != d.DeviceID || op != operationID {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_production_approvals VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, id, revision, d.DeviceID, a.IdentityID, operationID, taxReference, reviewSHA, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func NewProductionRecorder(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id, token, kind string, revision int, b fiscalcore.ProductionBinding) (*ProductionRecorder, error) {
	if db == nil || license == nil || revision < 1 || !idPattern.MatchString(id) || (kind != "issue" && kind != "query") {
		return nil, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = productionIntentTx(ctx, tx, a, d, id, token, false, b); err != nil {
		return nil, err
	}
	if _, err = productionPreparationTx(ctx, tx, a, id, revision); err != nil {
		return nil, err
	}
	identityBytes, _ := json.Marshal([]any{a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, id, revision, kind})
	sum := sha256.Sum256(identityBytes)
	r := &ProductionRecorder{db: db, license: license, actor: a, device: d, intentID: id, token: token, kind: kind, revision: revision, binding: b, scope: "fiscal-production:" + hex.EncodeToString(sum[:])}
	return r, tx.Commit()
}
func (r *ProductionRecorder) Scope() string { return r.scope }
func (r *ProductionRecorder) ValidateInvoice(ctx context.Context, in fiscalcore.Invoice) error {
	return r.ValidateProduction(ctx, in, r.binding.CertificateSHA256, r.binding.AuthorizationURL)
}
func (r *ProductionRecorder) ValidateProduction(ctx context.Context, in fiscalcore.Invoice, fingerprint, endpoint string) error {
	if r.kind != "issue" || fingerprint != r.binding.CertificateSHA256 || endpoint != r.binding.AuthorizationURL {
		return ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	prep, err := r.sendTx(ctx, tx)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(in)
	original, _ := json.Marshal(prep.Invoice)
	if !bytes.Equal(raw, original) {
		return ErrConflict
	}
	return tx.Commit()
}

func (r *ProductionRecorder) sendTx(ctx context.Context, tx *sql.Tx) (ProductionPreparation, error) {
	issue := r.kind == "issue"
	intent, err := productionIntentTx(ctx, tx, r.actor, r.device, r.intentID, r.token, issue, r.binding)
	if err != nil {
		return ProductionPreparation{}, err
	}
	permission := identity.ViewAccounting
	if issue {
		permission = identity.Sell
	}
	if err = r.license.RequireTx(ctx, tx, r.actor, r.device, permission, modules.Fiscal); err != nil {
		return ProductionPreparation{}, err
	}
	selected := r.revision
	if issue {
		selected = 0
	}
	prep, err := productionPreparationTx(ctx, tx, r.actor, r.intentID, selected)
	if err != nil {
		return prep, err
	}
	if issue {
		if prep.Revision != r.revision || prep.ConfigurationSHA256 != r.binding.SHA256() {
			return prep, ErrConflict
		}
		if err = productionSessionTx(ctx, tx, r.actor, r.device, r.intentID, r.revision, r.token); err != nil {
			return prep, err
		}
		var sha string
		if err = tx.QueryRowContext(ctx, `SELECT invoice_sha256 FROM fiscal_production_approvals WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=?`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.revision).Scan(&sha); err != nil {
			return prep, ErrConflict
		}
		if sha != prep.ReviewSHA256 {
			return prep, ErrIntegrity
		}
		if err = productionCorrectionAllowedBeforeRevisionTx(ctx, tx, r.actor, r.intentID, r.revision); err != nil {
			return prep, err
		}
	} else {
		if !r.binding.Valid(time.Now()) || r.binding.TenantID != r.actor.TenantID || r.binding.StoreID != r.actor.StoreID || r.binding.DeviceID != r.device.DeviceID || r.binding.Issuer != prep.Invoice.Key.CNPJ || r.binding.UF != prep.Invoice.Key.UF || r.binding.Model != prep.Invoice.Key.Model || r.binding.Series != prep.Invoice.Key.Series {
			return prep, ErrInvalid
		}
		var latest int
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(max(revision),0) FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND kind='issue'`, r.actor.TenantID, r.actor.StoreID, r.intentID).Scan(&latest); err != nil {
			return prep, err
		}
		if latest != r.revision {
			return prep, ErrConflict
		}
	}
	key, err := fiscalcore.AccessKey(prep.Invoice.Key)
	if err != nil {
		return prep, err
	}
	if err = validateInvoiceSnapshot(prep.Invoice, intent.Snapshot, key, 1); err != nil {
		return prep, err
	}
	return prep, nil
}

func (r *ProductionRecorder) Reserve(ctx context.Context, in fiscalcore.RecordedRequest) error {
	if !idPattern.MatchString(in.OperationID) || in.Scope != r.scope || in.Environment != "1" || in.Issuer != r.binding.Issuer || len(in.Body) > 2*fiscalcore.MaxXML {
		return ErrInvalid
	}
	sum := sha256.Sum256(in.Body)
	if in.RequestSHA256 != hex.EncodeToString(sum[:]) {
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
	prep, err := r.sendTx(ctx, tx)
	if err != nil {
		return err
	}
	key, _ := fiscalcore.AccessKey(prep.Invoice.Key)
	if key != in.Key {
		return ErrInvalid
	}
	if r.kind == "issue" {
		if in.URL != r.binding.AuthorizationURL {
			return ErrInvalid
		}
		original, e := fiscalcore.OriginalProductionNFeFromRequest(in)
		if e != nil {
			return e
		}
		// Rebuild the reviewed payload and compare the signed infNFe digest.
		// Certificate/QR supplements are outside that immutable payload.
		if e = fiscalcore.MatchProductionInvoice(prep.Invoice, original.SignedXML, r.binding.CertificateSHA256); e != nil {
			return e
		}
	} else {
		if in.URL != r.binding.ConsultationURL {
			return ErrInvalid
		}
		expected, e := fiscalcore.BuildNFeConsultation(key, "1")
		if e != nil || !bytes.Equal(in.Body, expected) {
			return ErrInvalid
		}
	}
	raw, _ := json.Marshal(in)
	var existing, existingIntent, existingKind string
	var existingRevision int
	err = tx.QueryRowContext(ctx, `SELECT request_json,intent_id,kind,revision FROM fiscal_production_requests WHERE tenant_id=? AND device_id=? AND operation_id=?`, r.actor.TenantID, r.device.DeviceID, in.OperationID).Scan(&existing, &existingIntent, &existingKind, &existingRevision)
	if err == nil {
		if existing != string(raw) || existingIntent != r.intentID || existingKind != r.kind || existingRevision != r.revision {
			return ErrConflict
		}
		return fiscalcore.ErrRecorded
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=? AND kind='issue' AND ?='issue'`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.revision, r.kind).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return fiscalcore.ErrRecorded
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_production_requests VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.revision, r.device.DeviceID, r.actor.IdentityID, in.OperationID, r.kind, string(mustProductionJSON(r.binding)), string(raw), in.RequestSHA256, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ProductionRecorder) BeforeSend(ctx context.Context, operationID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = r.sendTx(ctx, tx); err != nil {
		return err
	}
	req, _, err := productionRecordTx(ctx, tx, r.actor, r.device.DeviceID, operationID)
	if err != nil {
		return err
	}
	if req.Scope != r.scope {
		return ErrConflict
	}
	return tx.Commit()
}

func productionRecordTx(ctx context.Context, tx *sql.Tx, a identity.Scope, device, operation string) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	var req fiscalcore.RecordedRequest
	var result fiscalcore.ExchangeResult
	var raw, hash, store, configuration, created, kind, intentID, actor string
	var revision int
	err := tx.QueryRowContext(ctx, `SELECT request_json,request_sha256,store_id,config_json,created_at,kind,intent_id,revision,actor_id FROM fiscal_production_requests WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, device, operation).Scan(&raw, &hash, &store, &configuration, &created, &kind, &intentID, &revision, &actor)
	if errors.Is(err, sql.ErrNoRows) {
		return req, result, ErrNotFound
	}
	if err != nil {
		return req, result, err
	}
	if store != a.StoreID || fiscalcore.DecodeInput([]byte(raw), &req) != nil {
		return req, result, ErrIntegrity
	}
	canonical, _ := json.Marshal(req)
	sum := sha256.Sum256(req.Body)
	if string(canonical) != raw || req.Environment != "1" || req.OperationID != operation || len(req.Body) > 2*fiscalcore.MaxXML || req.RequestSHA256 != hash || hash != hex.EncodeToString(sum[:]) {
		return req, result, ErrIntegrity
	}
	var binding fiscalcore.ProductionBinding
	at, e := time.Parse(time.RFC3339Nano, created)
	if e != nil || fiscalcore.DecodeInput([]byte(configuration), &binding) != nil || !binding.Valid(at) || binding.TenantID != a.TenantID || binding.StoreID != a.StoreID || binding.DeviceID != device {
		return req, result, ErrIntegrity
	}
	if string(mustProductionJSON(binding)) != configuration || !fiscalcore.ValidAccessKey(req.Key) || req.Issuer != binding.Issuer || req.Key[:2] != stateCodes[binding.UF] || req.Key[6:20] != binding.Issuer || req.Key[20:22] != binding.Model || req.Key[22:25] != formatSeries(binding.Series) {
		return req, result, ErrIntegrity
	}
	scopeRaw, _ := json.Marshal([]any{a.TenantID, a.StoreID, device, actor, intentID, revision, kind})
	scopeSum := sha256.Sum256(scopeRaw)
	if req.Scope != "fiscal-production:"+hex.EncodeToString(scopeSum[:]) {
		return req, result, ErrIntegrity
	}
	if kind == "issue" {
		if req.URL != binding.AuthorizationURL {
			return req, result, ErrIntegrity
		}
		prep, e := productionPreparationTx(ctx, tx, a, intentID, revision)
		if e != nil || prep.ConfigurationSHA256 != binding.SHA256() {
			return req, result, ErrIntegrity
		}
		original, e := fiscalcore.OriginalProductionNFeFromRequest(req)
		if e != nil {
			return req, result, ErrIntegrity
		}
		if fiscalcore.MatchProductionInvoice(prep.Invoice, original.SignedXML, binding.CertificateSHA256) != nil {
			return req, result, ErrIntegrity
		}
	} else if kind == "query" {
		expected, e := fiscalcore.BuildNFeConsultation(req.Key, "1")
		if e != nil || !bytes.Equal(expected, req.Body) || req.URL != binding.ConsultationURL {
			return req, result, ErrIntegrity
		}
	} else {
		return req, result, ErrIntegrity
	}
	result = fiscalcore.ExchangeResult{OperationID: operation, State: "uncertain"}
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_production_results WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, device, operation).Scan(&raw)
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
	sum = sha256.Sum256(result.Body)
	if string(canonical) != raw || result.OperationID != operation || !validProductionResult(result) || result.State == "response_received" && result.SHA256 != hex.EncodeToString(sum[:]) {
		return req, result, ErrIntegrity
	}
	return req, result, nil
}
func validProductionResult(in fiscalcore.ExchangeResult) bool {
	if !idPattern.MatchString(in.OperationID) || len(in.Body) > 2*fiscalcore.MaxXML || in.HTTPStatus != 0 && (in.HTTPStatus < 100 || in.HTTPStatus > 599) {
		return false
	}
	if in.State == "uncertain" {
		return len(in.Body) == 0 && in.SHA256 == ""
	}
	sum := sha256.Sum256(in.Body)
	return in.State == "response_received" && in.HTTPStatus != 0 && in.SHA256 == hex.EncodeToString(sum[:])
}
func (r *ProductionRecorder) Complete(ctx context.Context, in fiscalcore.ExchangeResult) error {
	if !validProductionResult(in) {
		return ErrInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	req, _, err := productionRecordTx(ctx, tx, r.actor, r.device.DeviceID, in.OperationID)
	if err != nil {
		return err
	}
	if req.Scope != r.scope {
		return ErrConflict
	}
	raw, _ := json.Marshal(in)
	var old string
	err = tx.QueryRowContext(ctx, `SELECT result_json FROM fiscal_production_results WHERE tenant_id=? AND device_id=? AND operation_id=?`, r.actor.TenantID, r.device.DeviceID, in.OperationID).Scan(&old)
	if err == nil {
		if old != string(raw) {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_production_results VALUES(?,?,?,?,?)`, r.actor.TenantID, r.device.DeviceID, in.OperationID, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *ProductionRecorder) Read(ctx context.Context) (fiscalcore.RecordedRequest, fiscalcore.ExchangeResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	defer tx.Rollback()
	if _, err = productionIntentTx(ctx, tx, r.actor, r.device, r.intentID, r.token, false, r.binding); err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	var operation, device string
	if err = tx.QueryRowContext(ctx, `SELECT operation_id,device_id FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=? AND kind=? ORDER BY rowid DESC LIMIT 1`, r.actor.TenantID, r.actor.StoreID, r.intentID, r.revision, r.kind).Scan(&operation, &device); err != nil {
		return fiscalcore.RecordedRequest{}, fiscalcore.ExchangeResult{}, err
	}
	req, result, err := productionRecordTx(ctx, tx, r.actor, device, operation)
	if err != nil {
		return req, result, err
	}
	return req, result, tx.Commit()
}

func productionLatestEvidenceTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, beforeRevision int) (fiscalcore.ProductionEvidence, int, error) {
	var revision int
	var device, op string
	err := tx.QueryRowContext(ctx, `SELECT revision,device_id,operation_id FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND kind='issue' AND (?=0 OR revision<?) ORDER BY revision DESC LIMIT 1`, a.TenantID, a.StoreID, id, beforeRevision, beforeRevision).Scan(&revision, &device, &op)
	if errors.Is(err, sql.ErrNoRows) {
		return fiscalcore.ProductionEvidence{State: "prepared"}, 0, nil
	}
	if err != nil {
		return fiscalcore.ProductionEvidence{}, 0, err
	}
	original, result, err := productionRecordTx(ctx, tx, a, device, op)
	if err != nil {
		return fiscalcore.ProductionEvidence{}, revision, err
	}
	var qdev, qop string
	err = tx.QueryRowContext(ctx, `SELECT device_id,operation_id FROM fiscal_production_requests WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=? AND kind='query' ORDER BY rowid DESC LIMIT 1`, a.TenantID, a.StoreID, id, revision).Scan(&qdev, &qop)
	if errors.Is(err, sql.ErrNoRows) {
		evidence, e := fiscalcore.ReconcileProductionIssuance(original, result)
		if e != nil {
			evidence.State = "uncertain"
		}
		return evidence, revision, nil
	}
	if err != nil {
		return fiscalcore.ProductionEvidence{}, revision, err
	}
	query, qresult, err := productionRecordTx(ctx, tx, a, qdev, qop)
	if err != nil {
		return fiscalcore.ProductionEvidence{}, revision, err
	}
	proof, err := fiscalcore.ReconcileProductionNFe(original, query, qresult)
	out := fiscalcore.ProductionEvidence{State: "uncertain", Key: original.Key, OperationID: op, QueryOperationID: qop, Status: proof.Status, Reason: proof.Reason}
	if err != nil {
		return out, revision, nil
	}
	switch proof.Disposition {
	case "authorized_indicated_production":
		out.State = "authorized"
		out.Protocol = proof.Protocol
		out.ProcessedXML = proof.ProcessedXML
	case "canceled_indicated_production":
		out.State = "canceled_requires_review"
	case "denied_indicated_production":
		out.State = "denied_requires_review"
	case "not_found_indicated":
		out.State = "not_found_requires_review"
	}
	return out, revision, nil
}
func productionCorrectionAllowedTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) error {
	return productionCorrectionAllowedBeforeRevisionTx(ctx, tx, a, id, 0)
}
func productionCorrectionAllowedBeforeRevisionTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, revision int) error {
	e, _, err := productionLatestEvidenceTx(ctx, tx, a, id, revision)
	if err != nil {
		return err
	}
	if e.State != "prepared" && e.State != "rejected_requires_review" && e.State != "batch_rejected_requires_review" {
		return ErrConflict
	}
	return nil
}

func ReadProduction(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id, token string) (ProductionStatus, error) {
	var out ProductionStatus
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if _, err = productionIntentTx(ctx, tx, a, d, id, token, false, fiscalcore.ProductionBinding{}); err != nil {
		return out, err
	}
	out.Preparation, err = productionPreparationTx(ctx, tx, a, id, 0)
	if err != nil {
		return out, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_production_approvals WHERE tenant_id=? AND store_id=? AND intent_id=? AND revision=? AND invoice_sha256=?`, a.TenantID, a.StoreID, id, out.Preparation.Revision, out.Preparation.ReviewSHA256).Scan(&count); err != nil {
		return out, err
	}
	out.Approved = count == 1
	out.Evidence, out.EvidenceRevision, err = productionLatestEvidenceTx(ctx, tx, a, id, 0)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func mustProductionJSON(value any) []byte { raw, _ := json.Marshal(value); return raw }
