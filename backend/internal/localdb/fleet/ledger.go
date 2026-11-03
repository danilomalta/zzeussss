package fleet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func one(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, e := tx.ExecContext(ctx, q, args...)
	if e != nil {
		if strings.Contains(e.Error(), "constraint failed") {
			return ErrConflict
		}
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
func authorize(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, money bool) (bool, error) {
	if e := identity.CanOperateTx(ctx, tx, a, d, identity.ViewOrders); e != nil {
		return false, e
	}
	if money {
		if e := identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); e != nil {
			return false, e
		}
	}
	var role string
	e := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role)
	return role == "owner" || role == "manager", e
}
func assignee(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) error {
	if id == "" {
		return nil
	}
	if !ID(id) {
		return ErrInvalid
	}
	a.IdentityID = id
	_, e := authorize(ctx, tx, a, d, false)
	return e
}
func scan(s interface{ Scan(...any) error }) (Record, error) {
	var r Record
	var raw string
	e := s.Scan(&r.Kind, &r.ID, &r.AssetID, &r.State, &r.AssignedTo, &r.Revision, &raw, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	r.Data = json.RawMessage(raw)
	if errors.Is(e, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, e
}

const columns = `kind,id,COALESCE(asset_id,''),state,COALESCE(assigned_to,''),revision,data_json,created_by,created_at,updated_at`

func load(ctx context.Context, tx *sql.Tx, a identity.Scope, kind, id string) (Record, error) {
	return scan(tx.QueryRowContext(ctx, `SELECT `+columns+` FROM fleet_records WHERE tenant_id=? AND store_id=? AND kind=? AND id=?`, a.TenantID, a.StoreID, kind, id))
}
func proof(ctx context.Context, tx *sql.Tx, a identity.Scope, r Record) error {
	var device, op string
	var total, min, max int64
	if e := tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(revision),0),COALESCE(max(revision),0) FROM fleet_events WHERE tenant_id=? AND store_id=? AND kind=? AND record_id=?`, a.TenantID, a.StoreID, r.Kind, r.ID).Scan(&total, &min, &max); e != nil {
		return e
	}
	if total != r.Revision || min != 1 || max != r.Revision {
		return ErrConflict
	}
	if e := tx.QueryRowContext(ctx, `SELECT device_id,operation_id FROM fleet_events WHERE tenant_id=? AND store_id=? AND kind=? AND record_id=? AND revision=?`, a.TenantID, a.StoreID, r.Kind, r.ID, r.Revision).Scan(&device, &op); e != nil {
		return ErrConflict
	}
	out, e := receipt(ctx, tx, a, device, op, false)
	if e != nil {
		return e
	}
	if !bytes.Equal(encode(out.Record), encode(r)) {
		return ErrConflict
	}
	return nil
}
func receipt(ctx context.Context, tx *sql.Tx, a identity.Scope, device, op string, bound bool) (Result, error) {
	var out Result
	var actor, hash, raw, body string
	var restricted bool
	e := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash,request_json,response_json,restricted FROM fleet_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, device, op).Scan(&actor, &hash, &raw, &body, &restricted)
	if errors.Is(e, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if e != nil {
		return out, e
	}
	if bound && actor != a.IdentityID {
		return out, ErrNotFound
	}
	h := sha256.Sum256([]byte(raw))
	if hash != hex.EncodeToString(h[:]) || json.Unmarshal([]byte(body), &out) != nil || out.OperationID != op || out.Repeated {
		return out, ErrConflict
	}
	if bound && restricted {
		if _, e = authorize(ctx, tx, a, identity.DeviceContext{TenantID: a.TenantID, StoreID: a.StoreID, DeviceID: device}, true); e != nil {
			return out, e
		}
	}
	if out.Ack != nil {
		var n int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_notice_acks WHERE tenant_id=? AND store_id=? AND notice_id=? AND identity_id=? AND device_id=? AND operation_id=? AND created_at=?`, a.TenantID, a.StoreID, out.Ack.NoticeID, actor, device, op, out.Ack.ClientAcknowledgedAt).Scan(&n); e != nil {
			return out, e
		}
		if n != 1 {
			return out, ErrConflict
		}
		return out, nil
	}
	if out.Record == nil {
		return out, ErrConflict
	}
	var n int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_events WHERE tenant_id=? AND store_id=? AND kind=? AND record_id=? AND revision=? AND device_id=? AND operation_id=? AND actor_id=?`, a.TenantID, a.StoreID, out.Record.Kind, out.Record.ID, out.Record.Revision, device, op, actor).Scan(&n)
	if e != nil {
		return out, e
	}
	if n != 1 {
		return out, ErrConflict
	}
	return out, nil
}
func canonical(raw []byte) (json.RawMessage, error) {
	var values map[string]json.RawMessage
	d := json.NewDecoder(bytes.NewReader(raw))
	token, e := d.Token()
	if e != nil || token != json.Delim('{') {
		return nil, ErrInvalid
	}
	seen := map[string]bool{}
	for d.More() {
		t, e := d.Token()
		if e != nil {
			return nil, ErrInvalid
		}
		key, ok := t.(string)
		if !ok || seen[key] {
			return nil, ErrInvalid
		}
		seen[key] = true
		var v json.RawMessage
		if d.Decode(&v) != nil {
			return nil, ErrInvalid
		}
		if bytes.HasPrefix(bytes.TrimSpace(v), []byte("{")) || bytes.HasPrefix(bytes.TrimSpace(v), []byte("[")) {
			return nil, ErrInvalid
		}
	}
	if e = json.Unmarshal(raw, &values); e != nil {
		return nil, ErrInvalid
	}
	return encode(values), nil
}
func begin(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, restricted, managerOnly bool) (*sql.Tx, bool, error) {
	if db == nil || contracts == nil {
		return nil, false, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, false, e
	}
	manager, e := authorize(ctx, tx, a, d, restricted)
	if e == nil && managerOnly && !manager {
		e = identity.ErrDenied
	}
	if e == nil {
		e = contracts.RequireTx(ctx, tx, a, d, identity.ViewOrders, modules.Logistics)
	}
	if e != nil {
		tx.Rollback()
		return nil, false, e
	}
	return tx, manager, nil
}
func prepare(kind string, in Input) (Input, string, string, error) {
	if !ID(in.OperationID) || !ID(in.ID) || !text(in.Reason, 1000, false) || in.ExpectedRevision < 0 || in.ExpectedRevision >= MaxValue || !text(in.Action, 40, false) || len(in.Data) > 16384 {
		return in, "", "", ErrInvalid
	}
	raw, e := canonical(in.Data)
	if e != nil {
		return in, "", "", e
	}
	in.Data = raw
	b := encode(struct {
		Kind  string `json:"kind"`
		Input Input  `json:"input"`
	}{kind, in})
	h := sha256.Sum256(b)
	return in, string(b), hex.EncodeToString(h[:]), nil
}
func replay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, in Input, hash string) (Result, bool, error) {
	var prior, actor string
	e := tx.QueryRowContext(ctx, `SELECT request_hash,actor_id FROM fleet_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID).Scan(&prior, &actor)
	if errors.Is(e, sql.ErrNoRows) {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID).Scan(&count); err != nil {
			return Result{}, true, err
		}
		if count != 0 {
			return Result{}, true, ErrConflict
		}
		return Result{}, false, nil
	}
	if e != nil {
		return Result{}, true, e
	}
	if prior != hash || actor != a.IdentityID {
		return Result{}, true, ErrConflict
	}
	out, e := receipt(ctx, tx, a, d.DeviceID, in.OperationID, true)
	out.Repeated = true
	return out, true, e
}
func operation(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, in Input, raw, hash string, restricted bool, out Result, now string) error {
	return one(ctx, tx, `INSERT INTO fleet_operations VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, a.IdentityID, hash, raw, string(encode(out)), restricted, now)
}
func event(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, in Input, r Record, now string) error {
	if e := one(ctx, tx, `INSERT INTO fleet_events VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, r.Kind, r.ID, r.Revision, d.DeviceID, in.OperationID, in.Action, a.IdentityID, now); e != nil {
		return e
	}
	id, e := localdb.NewID()
	if e != nil {
		return e
	}
	payload := encode(struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		ActorID  string `json:"actor_id"`
		Kind     string `json:"kind"`
		RecordID string `json:"record_id"`
		State    string `json:"state"`
		Revision int64  `json:"revision"`
	}{a.TenantID, a.StoreID, a.IdentityID, r.Kind, r.ID, r.State, r.Revision})
	return one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'fleet.changed',1,?,?)`, id, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, r.Kind+":"+r.ID, string(payload), now)
}
func write(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, kind string, in Input, restricted, managerOnly bool, fn func(*sql.Tx, *Record, bool, string) error) (Result, error) {
	if !known(kind) {
		return Result{}, ErrInvalid
	}
	var e error
	in, raw, hash, e := prepare(kind, in)
	if e != nil {
		return Result{}, e
	}
	tx, manager, e := begin(ctx, db, c, a, d, restricted, managerOnly)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if out, exists, e := replay(ctx, tx, a, d, in, hash); exists {
		if e == nil {
			e = tx.Commit()
		}
		return out, e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	r := Record{Kind: kind, ID: in.ID, CreatedBy: a.IdentityID, CreatedAt: now}
	existing, e := load(ctx, tx, a, kind, in.ID)
	if in.ExpectedRevision == 0 {
		if e == nil {
			return Result{}, ErrConflict
		}
		if !errors.Is(e, ErrNotFound) {
			return Result{}, e
		}
		if in.Action != "create" {
			return Result{}, ErrInvalid
		}
	} else {
		if e != nil {
			return Result{}, e
		}
		if e = proof(ctx, tx, a, existing); e != nil {
			return Result{}, e
		}
		if existing.Revision != in.ExpectedRevision {
			return Result{}, ErrConflict
		}
		r = existing
	}
	if e = fn(tx, &r, manager, now); e != nil {
		return Result{}, e
	}
	r.Revision = in.ExpectedRevision + 1
	r.UpdatedAt = now
	if in.ExpectedRevision == 0 {
		e = one(ctx, tx, `INSERT INTO fleet_records(tenant_id,store_id,kind,id,asset_id,state,assigned_to,revision,data_json,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, kind, in.ID, nullable(r.AssetID), r.State, nullable(r.AssignedTo), r.Revision, string(r.Data), r.CreatedBy, r.CreatedAt, now)
	} else {
		e = one(ctx, tx, `UPDATE fleet_records SET asset_id=?,state=?,assigned_to=?,revision=?,data_json=?,updated_at=? WHERE tenant_id=? AND store_id=? AND kind=? AND id=? AND revision=?`, nullable(r.AssetID), r.State, nullable(r.AssignedTo), r.Revision, string(r.Data), now, a.TenantID, a.StoreID, kind, in.ID, in.ExpectedRevision)
	}
	if e != nil {
		return Result{}, e
	}
	out := Result{OperationID: in.OperationID, Record: &r}
	if e = operation(ctx, tx, a, d, in, raw, hash, restricted, out, now); e != nil {
		return out, e
	}
	if e = event(ctx, tx, a, d, in, r, now); e != nil {
		return out, e
	}
	if kind == "request" {
		if e = one(ctx, tx, `INSERT INTO fleet_notices(tenant_id,store_id,request_id,revision,recipient_id,created_at) VALUES(?,?,?,?,?,?)`, a.TenantID, a.StoreID, r.ID, r.Revision, nullable(r.AssignedTo), now); e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
