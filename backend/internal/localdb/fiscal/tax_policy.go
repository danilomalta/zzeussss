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
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type TaxPolicyInput struct {
	OperationID      string             `json:"operation_id"`
	ExpectedRevision int                `json:"expected_revision"`
	ProfileID        string             `json:"profile_id"`
	ProfileRevision  int                `json:"profile_revision"`
	Active           bool               `json:"active"`
	Rule             fiscalcore.TaxRule `json:"rule"`
	Case             fiscalcore.TaxCase `json:"case"`
	Sources          []string           `json:"sources"`
}
type TaxPolicy struct {
	Input     TaxPolicyInput `json:"input"`
	Revision  int            `json:"revision"`
	SHA256    string         `json:"sha256"`
	ActorID   string         `json:"actor_id"`
	CreatedAt string         `json:"created_at"`
	Repeated  bool           `json:"repeated"`
}

func taxHash(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func (in TaxPolicyInput) valid() bool {
	return idPattern.MatchString(in.OperationID) && idPattern.MatchString(in.Rule.ID) && idPattern.MatchString(in.Rule.ProductID) && idPattern.MatchString(in.ProfileID) && in.ExpectedRevision >= 0 && in.ExpectedRevision < 1000000 && in.ProfileRevision > 0 && fiscalcore.ValidateTaxPolicy(in.Rule, in.Case, in.Sources) == nil
}
func taxPolicyTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, revision int) (TaxPolicy, error) {
	var out TaxPolicy
	var body, profile, operation string
	var active bool
	var profileRevision int
	err := tx.QueryRowContext(ctx, `SELECT request_json,request_sha256,revision,actor_id,created_at,profile_id,profile_revision,active,operation_id FROM fiscal_tax_policies WHERE tenant_id=? AND store_id=? AND policy_id=? AND (revision=? OR ?=0) ORDER BY revision DESC LIMIT 1`, a.TenantID, a.StoreID, id, revision, revision).Scan(&body, &out.SHA256, &out.Revision, &out.ActorID, &out.CreatedAt, &profile, &profileRevision, &active, &operation)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if taxHash([]byte(body)) != out.SHA256 || json.Unmarshal([]byte(body), &out.Input) != nil {
		return TaxPolicy{}, ErrIntegrity
	}
	canonical, _ := json.Marshal(out.Input)
	if string(canonical) != body || !out.Input.valid() || out.Input.Rule.ID != id || out.Input.ExpectedRevision+1 != out.Revision || out.Input.ProfileID != profile || out.Input.ProfileRevision != profileRevision || out.Input.Active != active || out.Input.OperationID != operation {
		return TaxPolicy{}, ErrIntegrity
	}
	return out, nil
}

// Publication is an explicit owner approval, not automatic verification of law.
// Every revision preserves provenance and belongs to one profile and store.
func PublishTaxPolicy(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in TaxPolicyInput, token string) (TaxPolicy, error) {
	var out TaxPolicy
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
	var old, actor, store, id string
	var rev int
	err = tx.QueryRowContext(ctx, `SELECT request_json,actor_id,store_id,policy_id,revision FROM fiscal_tax_policies WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&old, &actor, &store, &id, &rev)
	if err == nil {
		if old != string(body) || actor != a.IdentityID || store != a.StoreID {
			return out, ErrConflict
		}
		out, err = taxPolicyTx(ctx, tx, a, id, rev)
		if err != nil {
			return out, err
		}
		out.Repeated = true
		return out, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	var current int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(max(revision),0) FROM fiscal_tax_policies WHERE tenant_id=? AND store_id=? AND policy_id=?`, a.TenantID, a.StoreID, in.Rule.ID).Scan(&current); err != nil {
		return out, err
	}
	if current != in.ExpectedRevision {
		return out, ErrConflict
	}
	p, err := profileTx(ctx, tx, a, in.ProfileID, 0)
	if err != nil {
		return out, err
	}
	if p.Revision != in.ProfileRevision {
		return out, ErrConflict
	}
	if p.Input.State != in.Rule.OriginUF || p.Input.Municipality != in.Case.OriginMunicipality || p.Input.CRT != in.Rule.CRT || p.Input.Model != in.Rule.Model {
		return out, ErrInvalid
	}
	var product int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM products WHERE tenant_id=? AND id=?`, a.TenantID, in.Rule.ProductID).Scan(&product); err != nil {
		return out, err
	}
	if product != 1 {
		return out, ErrNotFound
	}
	// Reject ambiguous active latest policies. Historical revisions stay readable.
	rows, err := tx.QueryContext(ctx, `SELECT p.policy_id,p.revision FROM fiscal_tax_policies p WHERE p.tenant_id=? AND p.store_id=? AND p.active=1 AND p.policy_id<>? AND p.profile_id=? AND p.revision=(SELECT max(q.revision) FROM fiscal_tax_policies q WHERE q.tenant_id=p.tenant_id AND q.store_id=p.store_id AND q.policy_id=p.policy_id)`, a.TenantID, a.StoreID, in.Rule.ID, in.ProfileID)
	if err != nil {
		return out, err
	}
	type ref struct {
		id       string
		revision int
	}
	refs := []ref{}
	for rows.Next() {
		var v ref
		if err = rows.Scan(&v.id, &v.revision); err != nil {
			rows.Close()
			return out, err
		}
		refs = append(refs, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if in.Active {
		for _, v := range refs {
			other, e := taxPolicyTx(ctx, tx, a, v.id, v.revision)
			if e != nil {
				return out, e
			}
			r := other.Input.Rule
			if other.Input.Case == in.Case && r.ProductID == in.Rule.ProductID && r.OriginUF == in.Rule.OriginUF && r.DestinationUF == in.Rule.DestinationUF && r.CRT == in.Rule.CRT && r.Model == in.Rule.Model && r.ValidFrom <= in.Rule.ValidUntil && in.Rule.ValidFrom <= r.ValidUntil {
				return out, ErrConflict
			}
		}
	}
	out = TaxPolicy{Input: in, Revision: current + 1, SHA256: taxHash(body), ActorID: a.IdentityID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_tax_policies(tenant_id,store_id,policy_id,revision,device_id,actor_id,operation_id,profile_id,profile_revision,active,request_json,request_sha256,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.Rule.ID, out.Revision, d.DeviceID, a.IdentityID, in.OperationID, in.ProfileID, in.ProfileRevision, in.Active, string(body), out.SHA256, out.CreatedAt)
	if err != nil {
		return TaxPolicy{}, err
	}
	return out, tx.Commit()
}
func ReadTaxPolicy(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, revision int) (TaxPolicy, error) {
	if !idPattern.MatchString(id) || revision < 0 {
		return TaxPolicy{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return TaxPolicy{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return TaxPolicy{}, err
	}
	out, err := taxPolicyTx(ctx, tx, a, id, revision)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

type TaxQuoteInput struct {
	OperationID    string                `json:"operation_id"`
	PolicyID       string                `json:"policy_id"`
	PolicyRevision int                   `json:"policy_revision"`
	Context        fiscalcore.TaxContext `json:"context"`
	Case           fiscalcore.TaxCase    `json:"case"`
	Item           fiscalcore.TaxItem    `json:"item"`
}
type TaxQuote struct {
	Input        TaxQuoteInput        `json:"input"`
	Result       fiscalcore.TaxResult `json:"result"`
	PolicySHA256 string               `json:"policy_sha256"`
	ResultSHA256 string               `json:"result_sha256"`
	ActorID      string               `json:"actor_id"`
	CreatedAt    string               `json:"created_at"`
	Repeated     bool                 `json:"repeated"`
}

func taxQuoteTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, operation string) (TaxQuote, error) {
	var out TaxQuote
	var body, result, hash, id string
	var revision int
	err := tx.QueryRowContext(ctx, `SELECT request_json,request_sha256,result_json,result_sha256,policy_sha256,policy_id,policy_revision,actor_id,created_at FROM fiscal_tax_quotes WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, operation).Scan(&body, &hash, &result, &out.ResultSHA256, &out.PolicySHA256, &id, &revision, &out.ActorID, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if taxHash([]byte(body)) != hash || taxHash([]byte(result)) != out.ResultSHA256 || json.Unmarshal([]byte(body), &out.Input) != nil || json.Unmarshal([]byte(result), &out.Result) != nil {
		return TaxQuote{}, ErrIntegrity
	}
	canonical, _ := json.Marshal(out.Input)
	calculated, _ := json.Marshal(out.Result)
	if string(canonical) != body || string(calculated) != result || out.Input.OperationID != operation || out.Input.PolicyID != id || out.Input.PolicyRevision != revision {
		return TaxQuote{}, ErrIntegrity
	}
	p, err := taxPolicyTx(ctx, tx, a, id, revision)
	if err != nil {
		return TaxQuote{}, err
	}
	if p.SHA256 != out.PolicySHA256 || out.Input.Case != p.Input.Case {
		return TaxQuote{}, ErrIntegrity
	}
	check, err := fiscalcore.Calculate(out.Input.Context, out.Input.Item, []fiscalcore.TaxRule{p.Input.Rule})
	if err != nil {
		return TaxQuote{}, ErrIntegrity
	}
	checked, _ := json.Marshal(check)
	if string(checked) != result {
		return TaxQuote{}, ErrIntegrity
	}
	return out, nil
}

// Quote is calculation evidence only. It neither emits a document nor changes a
// sale, stock, cash or tax ledger. Recovery never recalculates with newer rules.
func CreateTaxQuote(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in TaxQuoteInput, token string) (TaxQuote, error) {
	var out TaxQuote
	if !idPattern.MatchString(in.OperationID) || !idPattern.MatchString(in.PolicyID) || in.PolicyRevision < 1 {
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
	old, err := taxQuoteTx(ctx, tx, a, d, in.OperationID)
	body, _ := json.Marshal(in)
	if err == nil {
		prior, _ := json.Marshal(old.Input)
		if string(prior) != string(body) || old.ActorID != a.IdentityID {
			return out, ErrConflict
		}
		old.Repeated = true
		return old, tx.Commit()
	}
	if !errors.Is(err, ErrNotFound) {
		return out, err
	}
	p, err := taxPolicyTx(ctx, tx, a, in.PolicyID, 0)
	if err != nil {
		return out, err
	}
	if p.Revision != in.PolicyRevision || !p.Input.Active {
		return out, ErrConflict
	}
	profile, err := profileTx(ctx, tx, a, p.Input.ProfileID, 0)
	if err != nil {
		return out, err
	}
	if profile.Revision != p.Input.ProfileRevision {
		return out, ErrConflict
	}
	if in.Case != p.Input.Case {
		return out, ErrInvalid
	}
	result, err := fiscalcore.Calculate(in.Context, in.Item, []fiscalcore.TaxRule{p.Input.Rule})
	if err != nil {
		return out, ErrInvalid
	}
	encoded, _ := json.Marshal(result)
	out = TaxQuote{Input: in, Result: result, PolicySHA256: p.SHA256, ResultSHA256: taxHash(encoded), ActorID: a.IdentityID, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_tax_quotes(tenant_id,store_id,operation_id,device_id,actor_id,policy_id,policy_revision,request_json,request_sha256,policy_sha256,result_json,result_sha256,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.OperationID, d.DeviceID, a.IdentityID, in.PolicyID, in.PolicyRevision, string(body), taxHash(body), p.SHA256, string(encoded), out.ResultSHA256, out.CreatedAt)
	if err != nil {
		return TaxQuote{}, err
	}
	return out, tx.Commit()
}
func ReadTaxQuote(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, operation string) (TaxQuote, error) {
	if !idPattern.MatchString(operation) {
		return TaxQuote{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return TaxQuote{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return TaxQuote{}, err
	}
	out, err := taxQuoteTx(ctx, tx, a, d, operation)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
