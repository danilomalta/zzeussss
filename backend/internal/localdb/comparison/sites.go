// Package comparison stores local site configuration. It does not fetch websites.
package comparison

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrInvalid  = errors.New("configuracao de site invalida")
	ErrConflict = errors.New("site ou revisao em conflito")
	ErrMissing  = errors.New("operacao nao encontrada")
	ErrLimit    = errors.New("limite de sites atingido")
)

const MaxSites = 32
const maxRevision int64 = 9007199254740991

type Input struct {
	OperationID      string `json:"operation_id"`
	ID               string `json:"id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Name             string `json:"name"`
	Origin           string `json:"origin"`
	SearchTemplate   string `json:"search_template"`
	Status           string `json:"status"`
}
type Site struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Origin         string `json:"origin"`
	SearchTemplate string `json:"search_template"`
	Status         string `json:"status"`
	Revision       int64  `json:"revision"`
	ChangedAt      string `json:"changed_at"`
	Mode           string `json:"mode"`
}
type Result struct {
	OperationID string `json:"operation_id"`
	Site        Site   `json:"site"`
	Repeated    bool   `json:"repeated"`
}

func cleanText(s string, max int) bool {
	return len(s) > 0 && len(s) <= max && utf8.ValidString(s) && strings.TrimSpace(s) == s && !strings.ContainsFunc(s, unicode.IsControl)
}
func publicHostname(host string) bool {
	if len(host) > 253 || !strings.Contains(host, ".") || net.ParseIP(host) != nil || strings.HasSuffix(host, ".") {
		return false
	}
	for _, suffix := range []string{".local", ".internal", ".localhost", ".test", ".invalid", ".example", ".onion", ".arpa"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	for _, part := range strings.Split(host, ".") {
		if len(part) == 0 || len(part) > 63 || part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	// Numeric dotted hosts can otherwise be interpreted as IPv4 by browsers.
	last := host[strings.LastIndex(host, ".")+1:]
	for _, c := range last {
		if c >= 'a' && c <= 'z' {
			return true
		}
	}
	return false
}
func ValidOrigin(origin string) bool {
	u, e := url.Parse(origin)
	return e == nil && cleanText(origin, 512) && u.Scheme == "https" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Opaque == "" && u.Port() == "" && (u.Path == "" || u.Path == "/") && u.Host == strings.ToLower(u.Host) && publicHostname(u.Hostname()) && origin == "https://"+u.Hostname()
}
func ValidTemplate(origin, template string) bool {
	if template == "" {
		return true
	}
	if !ValidOrigin(origin) || !cleanText(template, 1600) || strings.Count(template, "{query}") != 1 {
		return false
	}
	substituted := strings.Replace(template, "{query}", "titan-product", 1)
	if strings.ContainsAny(substituted, "{}\\") {
		return false
	}
	u, e := url.Parse(substituted)
	base, _ := url.Parse(origin)
	return e == nil && u.Scheme == "https" && u.Host == base.Host && u.User == nil && u.Fragment == "" && u.Opaque == "" && u.Port() == ""
}
func Valid(in Input) bool {
	return cleanText(in.OperationID, 128) && cleanText(in.ID, 128) && cleanText(in.Name, 120) && in.ExpectedRevision >= 0 && in.ExpectedRevision < maxRevision && ValidOrigin(in.Origin) && ValidTemplate(in.Origin, in.SearchTemplate) && (in.Status == "active" || in.Status == "inactive")
}
func affected(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, e := tx.ExecContext(ctx, q, args...)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func Save(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in Input) (Result, error) {
	if db == nil || !Valid(in) {
		return Result{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if e = license.RequireTx(ctx, tx, a, d, identity.ManageStock, modules.Inventory); e != nil {
		return Result{}, e
	}
	encoded, e := json.Marshal(in)
	if e != nil {
		return Result{}, e
	}
	var original, snapshot, actor, store string
	e = tx.QueryRowContext(ctx, `SELECT input_json,result_json,actor_identity_id,store_id FROM comparison_site_operations WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&original, &snapshot, &actor, &store)
	if e == nil {
		if original != string(encoded) || actor != a.IdentityID || store != a.StoreID {
			return Result{}, ErrConflict
		}
		var out Result
		if e = json.Unmarshal([]byte(snapshot), &out); e != nil {
			return Result{}, e
		}
		out.Repeated = true
		return out, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Result{}, e
	}
	var revision int64
	e = tx.QueryRowContext(ctx, `SELECT revision FROM comparison_sites WHERE tenant_id=? AND id=?`, a.TenantID, in.ID).Scan(&revision)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return Result{}, e
	}
	if (errors.Is(e, sql.ErrNoRows) && in.ExpectedRevision != 0) || (e == nil && revision != in.ExpectedRevision) {
		return Result{}, ErrConflict
	}
	var used int
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM comparison_sites WHERE tenant_id=? AND origin=? AND id<>?`, a.TenantID, in.Origin, in.ID).Scan(&used); e != nil {
		return Result{}, e
	}
	if used > 0 {
		return Result{}, ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	out := Result{OperationID: in.OperationID, Site: Site{ID: in.ID, Name: in.Name, Origin: in.Origin, SearchTemplate: in.SearchTemplate, Status: in.Status, Revision: in.ExpectedRevision + 1, ChangedAt: now, Mode: "external_link"}}
	if in.ExpectedRevision == 0 {
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM comparison_sites WHERE tenant_id=?`, a.TenantID).Scan(&count); e != nil {
			return Result{}, e
		}
		if count >= MaxSites {
			return Result{}, ErrLimit
		}
		e = affected(ctx, tx, `INSERT INTO comparison_sites VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, in.ID, in.Name, in.Origin, in.SearchTemplate, in.Status, out.Site.Revision, a.IdentityID, now)
	} else {
		e = affected(ctx, tx, `UPDATE comparison_sites SET name=?,origin=?,search_template=?,status=?,revision=?,changed_by=?,changed_at=? WHERE tenant_id=? AND id=? AND revision=?`, in.Name, in.Origin, in.SearchTemplate, in.Status, out.Site.Revision, a.IdentityID, now, a.TenantID, in.ID, in.ExpectedRevision)
	}
	if e != nil {
		return Result{}, e
	}
	snap, e := json.Marshal(out)
	if e != nil {
		return Result{}, e
	}
	if e = affected(ctx, tx, `INSERT INTO comparison_site_operations VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, d.DeviceID, in.OperationID, a.StoreID, a.IdentityID, in.ID, string(encoded), string(snap), now); e != nil {
		return Result{}, e
	}
	return out, tx.Commit()
}
func Read(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (*sql.Tx, error) {
	if db == nil {
		return nil, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ViewCatalog); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) ([]Site, error) {
	tx, e := Read(ctx, db, a, d)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT id,name,origin,search_template,status,revision,changed_at FROM comparison_sites WHERE tenant_id=? ORDER BY name,id LIMIT 33`, a.TenantID)
	if e != nil {
		return nil, e
	}
	out := []Site{}
	for rows.Next() {
		var s Site
		s.Mode = "external_link"
		if e = rows.Scan(&s.ID, &s.Name, &s.Origin, &s.SearchTemplate, &s.Status, &s.Revision, &s.ChangedAt); e != nil {
			rows.Close()
			return nil, e
		}
		if !ValidOrigin(s.Origin) || !ValidTemplate(s.Origin, s.SearchTemplate) {
			rows.Close()
			return nil, ErrInvalid
		}
		out = append(out, s)
	}
	e = rows.Err()
	closeErr := rows.Close()
	if e != nil {
		return nil, e
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(out) > MaxSites {
		return nil, ErrLimit
	}
	return out, tx.Commit()
}
func Operation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, op string) (Result, error) {
	if !cleanText(op, 128) {
		return Result{}, ErrInvalid
	}
	tx, e := Read(ctx, db, a, d)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); e != nil {
		return Result{}, e
	}
	var snapshot string
	e = tx.QueryRowContext(ctx, `SELECT result_json FROM comparison_site_operations WHERE tenant_id=? AND device_id=? AND operation_id=? AND actor_identity_id=? AND store_id=?`, a.TenantID, d.DeviceID, op, a.IdentityID, a.StoreID).Scan(&snapshot)
	if errors.Is(e, sql.ErrNoRows) {
		return Result{}, ErrMissing
	}
	if e != nil {
		return Result{}, e
	}
	var out Result
	if e = json.Unmarshal([]byte(snapshot), &out); e != nil {
		return Result{}, e
	}
	return out, tx.Commit()
}
