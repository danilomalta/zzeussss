package accesspolicy

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"titansystem-backend/internal/localdb/identity"
)

type Department struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}
type Member struct {
	IdentityID   string `json:"identity_id"`
	DepartmentID string `json:"department_id"`
	Revision     int64  `json:"revision"`
}
type Rule struct {
	TargetKind string `json:"target_kind"`
	TargetID   string `json:"target_id"`
	Permission string `json:"permission"`
	Effect     string `json:"effect"`
	Revision   int64  `json:"revision"`
}
type Delegation struct {
	IdentityID   string `json:"identity_id"`
	DepartmentID string `json:"department_id"`
	Status       string `json:"status"`
	Revision     int64  `json:"revision"`
}
type Policy struct {
	TenantID    string       `json:"tenant_id"`
	StoreID     string       `json:"store_id"`
	Departments []Department `json:"departments"`
	Members     []Member     `json:"members"`
	Rules       []Rule       `json:"rules"`
	Delegations []Delegation `json:"delegations"`
}

func readScope(ctx context.Context, tx *sql.Tx, device identity.DeviceContext, token, department string) (identity.Scope, bool, error) {
	s, role, err := sessionTx(ctx, tx, device, token)
	if err != nil {
		return s.Actor, false, err
	}
	if department != "" && !validID(department) {
		return s.Actor, false, ErrInput
	}
	if role == "owner" {
		return s.Actor, true, nil
	}
	if department == "" {
		return s.Actor, false, ErrDenied
	}
	yes, err := delegated(ctx, tx, s.Actor, department)
	if err != nil {
		return s.Actor, false, err
	}
	if !yes {
		return s.Actor, false, ErrDenied
	}
	return s.Actor, false, nil
}

// Read returns a bounded configuration snapshot. A delegate must request ONE
// authorized department. Never return credentials or account access records.
func Read(ctx context.Context, db *sql.DB, device identity.DeviceContext, token, department string) (Policy, error) {
	out := Policy{TenantID: device.TenantID, StoreID: device.StoreID, Departments: []Department{}, Members: []Member{}, Rules: []Rule{}, Delegations: []Delegation{}}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	a, owner, err := readScope(ctx, tx, device, token, department)
	if err != nil {
		return out, err
	}
	queries := []struct {
		query string
		args  []any
		scan  func(*sql.Rows) error
		count func() int
	}{
		{`SELECT id,name,status,revision FROM access_departments WHERE tenant_id=? AND store_id=? AND (?='' OR id=?) ORDER BY id LIMIT 501`, []any{a.TenantID, a.StoreID, department, department}, func(r *sql.Rows) error {
			var x Department
			err := r.Scan(&x.ID, &x.Name, &x.Status, &x.Revision)
			out.Departments = append(out.Departments, x)
			return err
		}, func() int { return len(out.Departments) }},
		{`SELECT identity_id,department_id,revision FROM access_department_members WHERE tenant_id=? AND store_id=? AND (?='' OR department_id=?) ORDER BY identity_id LIMIT 501`, []any{a.TenantID, a.StoreID, department, department}, func(r *sql.Rows) error {
			var x Member
			err := r.Scan(&x.IdentityID, &x.DepartmentID, &x.Revision)
			out.Members = append(out.Members, x)
			return err
		}, func() int { return len(out.Members) }},
		{`SELECT target_kind,target_id,permission,effect,revision FROM access_rules r WHERE tenant_id=? AND store_id=? AND (?='' OR (target_kind='department' AND target_id=?) OR (target_kind='member' AND EXISTS (SELECT 1 FROM access_department_members dm WHERE dm.tenant_id=r.tenant_id AND dm.store_id=r.store_id AND dm.identity_id=r.target_id AND dm.department_id=?))) ORDER BY target_kind,target_id,permission LIMIT 501`, []any{a.TenantID, a.StoreID, department, department, department}, func(r *sql.Rows) error {
			var x Rule
			err := r.Scan(&x.TargetKind, &x.TargetID, &x.Permission, &x.Effect, &x.Revision)
			out.Rules = append(out.Rules, x)
			return err
		}, func() int { return len(out.Rules) }},
		{`SELECT identity_id,department_id,status,revision FROM access_delegations WHERE tenant_id=? AND store_id=? AND (?='' OR department_id=?) AND (? OR identity_id=?) ORDER BY department_id,identity_id LIMIT 501`, []any{a.TenantID, a.StoreID, department, department, owner, a.IdentityID}, func(r *sql.Rows) error {
			var x Delegation
			err := r.Scan(&x.IdentityID, &x.DepartmentID, &x.Status, &x.Revision)
			out.Delegations = append(out.Delegations, x)
			return err
		}, func() int { return len(out.Delegations) }},
	}
	for _, q := range queries {
		rows, err := tx.QueryContext(ctx, q.query, q.args...)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			if err = q.scan(rows); err != nil {
				rows.Close()
				return out, err
			}
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil {
			return out, err
		}
		if closeErr != nil {
			return out, closeErr
		}
		if q.count() > 500 {
			return Policy{}, ErrInput
		} // caller must narrow to a department, no silent truncation
	}
	return out, tx.Commit()
}

type AuditEvent struct {
	Reference    string `json:"reference"`
	Kind         string `json:"kind"`
	ActorID      string `json:"actor_id"`
	TargetID     string `json:"target_id"`
	DepartmentID string `json:"department_id"`
	Permission   string `json:"permission"`
	Before       string `json:"before"`
	After        string `json:"after"`
	Reason       string `json:"reason"`
	CreatedUnix  int64  `json:"created_unix"`
	Revision     int64  `json:"revision"`
}
type AuditPage struct {
	Items      []AuditEvent `json:"items"`
	NextCursor string       `json:"next_cursor"`
}
type cursor struct {
	Source     string `json:"source,omitempty"`
	Tenant     string `json:"tenant"`
	Store      string `json:"store"`
	Department string `json:"department"`
	Time       int64  `json:"time"`
	Reference  string `json:"reference"`
}

// Audit provides descending keyset pagination with a cursor bound to the scope.
// Owners see the eight supported administrative sources, delegates only policy
// events in the explicit department. No raw JSON payloads or hashes are read.
func Audit(ctx context.Context, db *sql.DB, device identity.DeviceContext, token, department, encoded string, limit int) (AuditPage, error) {
	return AuditFiltered(ctx, db, device, token, department, "", encoded, limit)
}

// AuditFiltered adds a source whitelist without exposing raw event payloads.
func AuditFiltered(ctx context.Context, db *sql.DB, device identity.DeviceContext, token, department, source, encoded string, limit int) (AuditPage, error) {
	out := AuditPage{Items: []AuditEvent{}}
	if !validSource(source) {
		return out, ErrInput
	}
	if department != "" && source != "" && source != "policy" {
		return out, ErrDenied
	}
	if limit < 1 || limit > 100 {
		return out, ErrInput
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	a, owner, err := readScope(ctx, tx, device, token, department)
	if err != nil {
		return out, err
	}
	cur := cursor{Source: source, Tenant: a.TenantID, Store: a.StoreID, Department: department, Time: 9223372036854775807, Reference: "~"}
	if encoded != "" {
		if len(encoded) > 2048 {
			return out, ErrInput
		}
		raw, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return out, ErrInput
		}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if dec.Decode(&cur) != nil || cur.Tenant != a.TenantID || cur.Store != a.StoreID || cur.Department != department || cur.Source != source || cur.Time < 0 || len(cur.Reference) > 512 || cur.Reference == "" {
			return out, ErrInput
		}
		canonical, _ := json.Marshal(cur)
		if base64.RawURLEncoding.EncodeToString(canonical) != encoded {
			return out, ErrInput
		}
	}
	query := `WITH events AS (
 SELECT 'policy' source,'policy:'||printf('%d:%s:%s',length(device_id),device_id,operation_id) ref,kind,actor_id,target_id,department_id,permission,before_value,after_value,reason,created_unix,revision
 FROM access_policy_events WHERE tenant_id=? AND store_id=? AND (?='' OR department_id=?)
 UNION ALL
 SELECT 'account','account:'||printf('%d:%s:%s',length(device_id),device_id,operation_id),kind,actor_id,target_id,'','','','',reason,created_unix,0
 FROM account_access_operations WHERE tenant_id=? AND store_id=? AND ? AND ?=''
 UNION ALL
 SELECT 'security','security:'||id,kind,identity_id,identity_id,'','','','','',created_unix,0
 FROM account_security_events WHERE tenant_id=? AND store_id=? AND ? AND ?=''
 UNION ALL
 SELECT 'staff','staff:'||printf('%d:%s:%s',length(device_id),device_id,operation_id),'staff_registered',created_by,identity_id,'','','','','',COALESCE(CAST(strftime('%s',created_at) AS INTEGER),0),0
 FROM staff_registrations WHERE tenant_id=? AND store_id=? AND ? AND ?=''
 UNION ALL
 SELECT 'invite','invite:'||e.id,'invite_'||e.action,e.actor_identity_id,i.target_identity_id,'','','','','',e.occurred_unix,0
 FROM membership_invite_events e JOIN membership_invites i ON i.id=e.invite_id WHERE i.tenant_id=? AND i.store_id=? AND ? AND ?=''
 UNION ALL
 SELECT 'pairing','pairing:'||e.id,'device_'||e.action,e.actor_ref,e.device_id,'','','','','',e.occurred_unix,0
 FROM device_pairing_events e JOIN device_pairings p ON p.tenant_id=e.tenant_id AND p.device_id=e.device_id WHERE e.tenant_id=? AND p.store_id=? AND ? AND ?=''
 UNION ALL
 SELECT 'peer','peer:'||id,'peer_'||action,actor_id,sender_device_id,'',event_type,'','','',COALESCE(CAST(strftime('%s',created_at) AS INTEGER),0),0
 FROM sync_peer_audit WHERE tenant_id=? AND store_id=? AND ? AND ?=''
 UNION ALL
 SELECT 'encryption','encryption:'||id,'encryption_key_approved',approved_by,device_id,'','','','','',COALESCE(CAST(strftime('%s',approved_at) AS INTEGER),0),revision
 FROM device_encryption_key_audit WHERE tenant_id=? AND store_id=? AND ? AND ?=''
 ) SELECT ref,kind,actor_id,target_id,department_id,permission,before_value,after_value,reason,created_unix,revision
 FROM events WHERE (?='' OR source=?) AND (created_unix<? OR (created_unix=? AND ref<?)) ORDER BY created_unix DESC,ref DESC LIMIT ?`
	args := []any{a.TenantID, a.StoreID, department, department}
	for i := 0; i < 7; i++ {
		args = append(args, a.TenantID, a.StoreID, owner, department)
	}
	args = append(args, source, source, cur.Time, cur.Time, cur.Reference, limit+1)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var x AuditEvent
		if err = rows.Scan(&x.Reference, &x.Kind, &x.ActorID, &x.TargetID, &x.DepartmentID, &x.Permission, &x.Before, &x.After, &x.Reason, &x.CreatedUnix, &x.Revision); err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return out, err
	}
	if closeErr != nil {
		return out, closeErr
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		last := out.Items[limit-1]
		raw, _ := json.Marshal(cursor{Source: source, Tenant: a.TenantID, Store: a.StoreID, Department: department, Time: last.CreatedUnix, Reference: last.Reference})
		out.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, tx.Commit()
}

// IsDenied covers the authentication distinction without exposing database errors.
func IsDenied(err error) bool { return errors.Is(err, ErrDenied) || errors.Is(err, identity.ErrDenied) }

func validSource(source string) bool {
	switch source {
	case "", "policy", "account", "security", "staff", "invite", "pairing", "peer", "encryption":
		return true
	}
	return false
}
