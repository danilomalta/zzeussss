// Package companyregistration persists the customer's manual company profile.
// It neither issues a licence nor creates a fiscal authorization or owner.
package companyregistration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid company registration")
var ErrConflict = errors.New("company registration conflict")
var ErrMissing = errors.New("company registration not found")
var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

type Input struct {
	OperationID       string `json:"operation_id"`
	ExpectedRevision  int    `json:"expected_revision"`
	Stage             string `json:"stage"`
	CompanyName       string `json:"company_name"`
	Responsible       string `json:"responsible"`
	CNPJ              string `json:"cnpj"`
	LegalName         string `json:"legal_name"`
	UF                string `json:"uf"`
	StateRegistration string `json:"state_registration"`
	Municipality      string `json:"municipality_ibge"`
	CRT               string `json:"crt"`
	Provider          string `json:"provider"`
	Environment       string `json:"environment"`
}
type Record struct {
	Input     Input  `json:"input"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"created_at"`
	Repeated  bool   `json:"repeated"`
}

func text(s string, max int) bool {
	return utf8.ValidString(s) && len(s) <= max && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}
func (in Input) valid() bool {
	if !identifier.MatchString(in.OperationID) || in.ExpectedRevision < 0 || in.ExpectedRevision > 1000000 || !text(in.CompanyName, 180) || in.CompanyName == "" || !text(in.Responsible, 180) || in.Responsible == "" {
		return false
	}
	if !text(in.CNPJ, 14) || !text(in.LegalName, 180) || !text(in.UF, 2) || !text(in.StateRegistration, 20) || !text(in.Municipality, 7) || !text(in.CRT, 1) || !text(in.Provider, 16) || !text(in.Environment, 16) {
		return false
	}
	if in.Stage == "pre_registration" {
		return in.CNPJ == "" && in.LegalName == "" && in.UF == "" && in.StateRegistration == "" && in.Municipality == "" && in.CRT == "" && in.Provider == "" && in.Environment == ""
	}
	if in.Stage != "company_profile" || !fiscal.ValidCNPJ(in.CNPJ) || in.LegalName == "" || in.StateRegistration == "" || !regexp.MustCompile(`^[0-9]{7}$`).MatchString(in.Municipality) {
		return false
	}
	if !strings.Contains(" AC AL AP AM BA CE DF ES GO MA MT MS MG PA PB PR PE PI RJ RN RS RO RR SC SP SE TO ", " "+in.UF+" ") {
		return false
	}
	return (in.CRT == "1" || in.CRT == "2" || in.CRT == "3" || in.CRT == "4") && (in.Provider == "own" || in.Provider == "external") && (in.Environment == "homologation" || in.Environment == "production")
}
func authorize(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, token string) error {
	if err := fiscal.ValidateSaleSessionTx(ctx, tx, a, d, token); err != nil {
		return err
	}
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
		return identity.ErrDenied
	}
	if role != "owner" {
		return identity.ErrDenied
	}
	return nil
}
func Read(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, token, operation string) (Record, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, a, d, token); err != nil {
		return Record{}, err
	}
	if operation != "" && !identifier.MatchString(operation) {
		return Record{}, ErrInvalid
	}
	var out Record
	var raw string
	err = tx.QueryRowContext(ctx, `SELECT request_json,revision,created_at FROM company_registrations WHERE tenant_id=? AND (?='' OR (operation_id=? AND actor_id=? AND store_id=? AND device_id=?)) ORDER BY revision DESC LIMIT 1`, a.TenantID, operation, operation, a.IdentityID, a.StoreID, d.DeviceID).Scan(&raw, &out.Revision, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) && operation == "" {
		out.Input.Stage = "pre_registration"
		if err = tx.QueryRowContext(ctx, `SELECT t.name,i.display_name FROM tenants t JOIN identities i ON i.id=? WHERE t.id=?`, a.IdentityID, a.TenantID).Scan(&out.Input.CompanyName, &out.Input.Responsible); err != nil {
			return Record{}, err
		}
		return out, tx.Commit()
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrMissing
	}
	if err != nil {
		return Record{}, err
	}
	if json.Unmarshal([]byte(raw), &out.Input) != nil || !out.Input.valid() || out.Input.ExpectedRevision+1 != out.Revision {
		return Record{}, ErrConflict
	}
	return out, tx.Commit()
}
func Save(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, token string, in Input) (Record, error) {
	if !in.valid() {
		return Record{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	if err = authorize(ctx, tx, a, d, token); err != nil {
		return Record{}, err
	}
	raw, _ := json.Marshal(in)
	var old, actor, store, device, created string
	var revision int
	err = tx.QueryRowContext(ctx, `SELECT request_json,revision,created_at,actor_id,store_id,device_id FROM company_registrations WHERE tenant_id=? AND operation_id=?`, a.TenantID, in.OperationID).Scan(&old, &revision, &created, &actor, &store, &device)
	if err == nil {
		if old != string(raw) || actor != a.IdentityID || store != a.StoreID || device != d.DeviceID {
			return Record{}, ErrConflict
		}
		return Record{Input: in, Revision: revision, CreatedAt: created, Repeated: true}, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Record{}, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(max(revision),0) FROM company_registrations WHERE tenant_id=?`, a.TenantID).Scan(&revision); err != nil {
		return Record{}, err
	}
	if revision != in.ExpectedRevision {
		return Record{}, ErrConflict
	}
	if revision > 0 {
		var previous string
		if err = tx.QueryRowContext(ctx, `SELECT request_json FROM company_registrations WHERE tenant_id=? AND revision=?`, a.TenantID, revision).Scan(&previous); err != nil {
			return Record{}, err
		}
		var p Input
		if json.Unmarshal([]byte(previous), &p) != nil {
			return Record{}, ErrConflict
		}
		if p.Stage == "company_profile" && in.Stage == "pre_registration" {
			return Record{}, ErrConflict
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `INSERT INTO company_registrations VALUES(?,?,?,?,?,?,?,?)`, a.TenantID, revision+1, in.OperationID, a.IdentityID, a.StoreID, d.DeviceID, string(raw), now); err != nil {
		return Record{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE tenants SET name=? WHERE id=?`, in.CompanyName, a.TenantID); err != nil {
		return Record{}, err
	}
	return Record{Input: in, Revision: revision + 1, CreatedAt: now}, tx.Commit()
}
