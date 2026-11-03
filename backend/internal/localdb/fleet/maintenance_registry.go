package fleet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

type MaintenanceRecord struct {
	OperationID string          `json:"operation_id"`
	Kind        string          `json:"kind"`
	ID          string          `json:"id"`
	Revision    int64           `json:"revision"`
	State       string          `json:"state"`
	Data        json.RawMessage `json:"data"`
	ActorID     string          `json:"actor_id"`
	DeviceID    string          `json:"device_id"`
	RecordedAt  string          `json:"recorded_at"`
	Repeated    bool            `json:"repeated"`
}
type MaintenanceSpecialty struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type MaintenanceTechnician struct {
	IdentityID   string   `json:"identity_id"`
	SpecialtyIDs []string `json:"specialty_ids"`
	Contact      string   `json:"contact"`
	Note         string   `json:"note"`
}
type MaintenanceAsset struct {
	AssetID   string `json:"asset_id"`
	AssetType string `json:"asset_type"`
	Sector    string `json:"sector"`
	Location  string `json:"location"`
	Note      string `json:"note"`
}
type MaintenanceClassification struct {
	RequestID               string `json:"request_id"`
	ExpectedRequestRevision int64  `json:"expected_request_revision"`
	SpecialtyID             string `json:"specialty_id"`
	Note                    string `json:"note"`
}
type MaintenancePage struct {
	Items  []MaintenanceRecord `json:"items"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}
type MaintenanceHistoryItem struct {
	Action string            `json:"action"`
	Reason string            `json:"reason"`
	Result MaintenanceRecord `json:"result"`
}
type MaintenanceHistory struct {
	Items  []MaintenanceHistoryItem `json:"items"`
	Total  int64                    `json:"total"`
	Limit  int                      `json:"limit"`
	Offset int                      `json:"offset"`
}

func maintenanceKind(k string) bool { return oneOf(k, "specialty", "technician", "asset", "request") }
func maintenanceReceipt(ctx context.Context, tx *sql.Tx, a identity.Scope, device, id string, bound bool) (MaintenanceRecord, error) {
	var out MaintenanceRecord
	var actor, kind, rid, hash, raw, body string
	var revision int64
	e := tx.QueryRowContext(ctx, `SELECT actor_id,kind,id,revision,request_hash,request_json,response_json FROM maintenance_registry_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, device, id).Scan(&actor, &kind, &rid, &revision, &hash, &raw, &body)
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
	if hash != hex.EncodeToString(h[:]) || json.Unmarshal([]byte(body), &out) != nil || out.Repeated || out.OperationID != id || out.Kind != kind || out.ID != rid || out.Revision != revision || out.ActorID != actor || out.DeviceID != device {
		return out, ErrConflict
	}
	var request struct {
		Kind  string `json:"kind"`
		Input Input  `json:"input"`
	}
	if json.Unmarshal([]byte(raw), &request) != nil || request.Kind != kind || request.Input.ID != rid || request.Input.OperationID != id || request.Input.ExpectedRevision+1 != revision {
		return out, ErrConflict
	}
	if !oneOf(out.State, "active", "inactive") || !oneOf(request.Input.Action, "create", "update", "enable", "disable") {
		return out, ErrConflict
	}
	if _, e := time.Parse(time.RFC3339Nano, out.RecordedAt); e != nil {
		return out, ErrConflict
	}
	if oneOf(request.Input.Action, "create", "update") {
		expected, e := workshopCanonical(request.Input.Data)
		actual, e2 := workshopCanonical(out.Data)
		if e != nil || e2 != nil || !bytes.Equal(expected, actual) {
			return out, ErrConflict
		}
	}
	if request.Input.Action == "create" && out.State != "active" || request.Input.Action == "enable" && out.State != "active" || request.Input.Action == "disable" && out.State != "inactive" {
		return out, ErrConflict
	}
	return out, nil
}
func maintenanceLoad(ctx context.Context, tx *sql.Tx, a identity.Scope, kind, id string) (MaintenanceRecord, error) {
	var out MaintenanceRecord
	var body, state string
	var rev int64
	e := tx.QueryRowContext(ctx, `SELECT revision,state,body_json FROM maintenance_registry WHERE tenant_id=? AND store_id=? AND kind=? AND id=?`, a.TenantID, a.StoreID, kind, id).Scan(&rev, &state, &body)
	if errors.Is(e, sql.ErrNoRows) {
		var n int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM maintenance_registry_operations WHERE tenant_id=? AND store_id=? AND kind=? AND id=?`, a.TenantID, a.StoreID, kind, id).Scan(&n); e != nil {
			return out, e
		}
		if n != 0 {
			return out, ErrConflict
		}
		return out, ErrNotFound
	}
	if e != nil {
		return out, e
	}
	if json.Unmarshal([]byte(body), &out) != nil || out.Kind != kind || out.ID != id || out.Revision != rev || out.State != state || out.Repeated {
		return out, ErrConflict
	}
	var n, min, max int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(revision),0),COALESCE(max(revision),0) FROM maintenance_registry_operations WHERE tenant_id=? AND store_id=? AND kind=? AND id=?`, a.TenantID, a.StoreID, kind, id).Scan(&n, &min, &max); e != nil {
		return out, e
	}
	if n != rev || min != 1 || max != rev {
		return out, ErrConflict
	}
	proof, e := maintenanceReceipt(ctx, tx, a, out.DeviceID, out.OperationID, false)
	if e != nil {
		return out, e
	}
	if !bytes.Equal(encode(proof), encode(out)) {
		return out, ErrConflict
	}
	return out, nil
}
func MaintenanceWrite(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, kind string, in Input) (MaintenanceRecord, error) {
	if !maintenanceKind(kind) || !ID(in.ID) || !ID(in.OperationID) || !text(in.Reason, 1000, false) || in.ExpectedRevision < 0 || in.ExpectedRevision >= MaxValue || !oneOf(in.Action, "create", "update", "enable", "disable") || in.ExpectedRevision == 0 && in.Action != "create" || in.ExpectedRevision > 0 && in.Action == "create" {
		return MaintenanceRecord{}, ErrInvalid
	}
	data, e := workshopCanonical(in.Data)
	if e != nil {
		return MaintenanceRecord{}, e
	}
	in.Data = data
	raw := encode(struct {
		Kind  string `json:"kind"`
		Input Input  `json:"input"`
	}{kind, in})
	h := sha256.Sum256(raw)
	hash := hex.EncodeToString(h[:])
	tx, manager, e := begin(ctx, db, c, a, d, false, kind != "request" || in.ExpectedRevision > 0)
	if e != nil {
		return MaintenanceRecord{}, e
	}
	defer tx.Rollback()
	var prior, actor string
	e = tx.QueryRowContext(ctx, `SELECT request_hash,actor_id FROM maintenance_registry_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID).Scan(&prior, &actor)
	if e == nil {
		if prior != hash || actor != a.IdentityID {
			return MaintenanceRecord{}, ErrConflict
		}
		out, e := maintenanceReceipt(ctx, tx, a, d.DeviceID, in.OperationID, true)
		if e == nil {
			e = tx.Commit()
		}
		out.Repeated = true
		return out, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return MaintenanceRecord{}, e
	}
	old, e := maintenanceLoad(ctx, tx, a, kind, in.ID)
	if in.ExpectedRevision == 0 {
		if e == nil {
			return MaintenanceRecord{}, ErrConflict
		}
		if !errors.Is(e, ErrNotFound) {
			return MaintenanceRecord{}, e
		}
	} else {
		if e != nil {
			return old, e
		}
		if old.Revision != in.ExpectedRevision {
			return old, ErrConflict
		}
	}
	out := MaintenanceRecord{OperationID: in.OperationID, Kind: kind, ID: in.ID, Revision: in.ExpectedRevision + 1, State: old.State, Data: old.Data, ActorID: a.IdentityID, DeviceID: d.DeviceID, RecordedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if e = maintenanceChange(ctx, tx, a, d, manager, in, &out); e != nil {
		return out, e
	}
	if e = one(ctx, tx, `INSERT INTO maintenance_registry_operations VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, a.IdentityID, kind, in.ID, out.Revision, hash, string(raw), string(encode(out))); e != nil {
		return out, e
	}
	asset, request, person := "", "", ""
	switch kind {
	case "asset":
		asset = in.ID
	case "request":
		request = in.ID
	case "technician":
		person = in.ID
	}
	if in.ExpectedRevision == 0 {
		e = one(ctx, tx, `INSERT INTO maintenance_registry(tenant_id,store_id,kind,id,revision,state,body_json,asset_id,request_id,identity_id) VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, kind, in.ID, out.Revision, out.State, string(encode(out)), nullable(asset), nullable(request), nullable(person))
	} else {
		e = one(ctx, tx, `UPDATE maintenance_registry SET revision=?,state=?,body_json=? WHERE tenant_id=? AND store_id=? AND kind=? AND id=? AND revision=?`, out.Revision, out.State, string(encode(out)), a.TenantID, a.StoreID, kind, in.ID, in.ExpectedRevision)
	}
	if e != nil {
		return out, e
	}
	event, e := localdb.NewID()
	if e != nil {
		return out, e
	}
	payload := encode(struct {
		Kind     string `json:"kind"`
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
		State    string `json:"state"`
	}{kind, in.ID, out.Revision, out.State})
	if e = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'maintenance.registry.changed',1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, "maintenance:"+kind+":"+in.ID, string(payload), out.RecordedAt); e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func maintenanceChange(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, manager bool, in Input, out *MaintenanceRecord) error {
	if oneOf(in.Action, "enable", "disable") {
		if empty(in.Data) != nil {
			return ErrInvalid
		}
		if in.Action == "enable" {
			out.State = "active"
		} else {
			out.State = "inactive"
		}
		return nil
	}
	switch out.Kind {
	case "specialty":
		var v MaintenanceSpecialty
		if decode(in.Data, &v, "name", "description") != nil || !text(v.Name, 120, false) || !text(v.Description, 1000, true) {
			return ErrInvalid
		}
		out.Data = encode(v)
	case "technician":
		var v MaintenanceTechnician
		if decode(in.Data, &v, "identity_id", "specialty_ids", "contact", "note") != nil || v.IdentityID != in.ID || !text(v.Contact, 240, true) || !text(v.Note, 1000, true) || len(v.SpecialtyIDs) < 1 || len(v.SpecialtyIDs) > 20 {
			return ErrInvalid
		}
		if e := assignee(ctx, tx, a, d, v.IdentityID); e != nil {
			return e
		}
		seen := map[string]bool{}
		for _, id := range v.SpecialtyIDs {
			if !ID(id) || seen[id] {
				return ErrInvalid
			}
			seen[id] = true
			r, e := maintenanceLoad(ctx, tx, a, "specialty", id)
			if e != nil {
				return e
			}
			if r.State != "active" {
				return ErrConflict
			}
		}
		out.Data = encode(v)
	case "asset":
		var v MaintenanceAsset
		if decode(in.Data, &v, "asset_id", "asset_type", "sector", "location", "note") != nil || v.AssetID != in.ID || !oneOf(v.AssetType, "machine", "equipment", "installation", "vehicle") || !text(v.Sector, 160, false) || !text(v.Location, 240, false) || !text(v.Note, 1000, true) {
			return ErrInvalid
		}
		if _, _, e := assetTx(ctx, tx, a, v.AssetID); e != nil {
			return e
		}
		out.Data = encode(v)
	case "request":
		var v MaintenanceClassification
		if decode(in.Data, &v, "request_id", "expected_request_revision", "specialty_id", "note") != nil || v.RequestID != in.ID || !validRequestRevision(v.RequestID, v.ExpectedRequestRevision) || !ID(v.SpecialtyID) || !text(v.Note, 1000, true) {
			return ErrInvalid
		}
		r, e := workshopRequest(ctx, tx, a, v.RequestID, v.ExpectedRequestRevision)
		if e != nil {
			return e
		}
		if oneOf(r.State, "released", "cancelled") {
			return ErrConflict
		}
		var request Request
		if json.Unmarshal(r.Data, &request) != nil {
			return ErrConflict
		}
		if !manager && request.RequesterID != a.IdentityID && r.AssignedTo != a.IdentityID {
			return identity.ErrDenied
		}
		specialty, e := maintenanceLoad(ctx, tx, a, "specialty", v.SpecialtyID)
		if e != nil {
			return e
		}
		if specialty.State != "active" {
			return ErrConflict
		}
		out.Data = encode(v)
	default:
		return ErrInvalid
	}
	if in.Action == "create" {
		out.State = "active"
	}
	return nil
}
