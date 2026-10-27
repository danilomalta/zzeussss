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

func workshopPrepare(kind string, in Input) (Input, string, string, error) {
	if !workshopKind(kind) || !ID(in.OperationID) || !ID(in.ID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= MaxValue || !text(in.Reason, 1000, false) || !text(in.Action, 40, false) {
		return in, "", "", ErrInvalid
	}
	raw, e := workshopCanonical(in.Data)
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
func workshopReceipt(ctx context.Context, tx *sql.Tx, a identity.Scope, device, op string, bound bool) (WorkshopResult, error) {
	var out WorkshopResult
	var actor, kind, id, hash, raw, body string
	var rev int64
	e := tx.QueryRowContext(ctx, `SELECT actor_id,kind,id,revision,request_hash,request_json,response_json FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, device, op).Scan(&actor, &kind, &id, &rev, &hash, &raw, &body)
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
	if hash != hex.EncodeToString(h[:]) || json.Unmarshal([]byte(body), &out) != nil || out.Repeated || out.OperationID != op || out.Kind != kind || out.ID != id || out.Revision != rev || out.ActorID != actor || out.DeviceID != device {
		return out, ErrConflict
	}
	var request struct {
		Kind  string `json:"kind"`
		Input Input  `json:"input"`
	}
	if json.Unmarshal([]byte(raw), &request) != nil || request.Kind != kind || request.Input.ID != id || request.Input.OperationID != op || request.Input.ExpectedRevision+1 != rev {
		return out, ErrConflict
	}
	return out, nil
}
func workshopLoad(ctx context.Context, tx *sql.Tx, a identity.Scope, kind, id string) (WorkshopResult, error) {
	var out WorkshopResult
	var body, state, requestID string
	var rev int64
	e := tx.QueryRowContext(ctx, `SELECT revision,state,COALESCE(request_id,''),body_json FROM fleet_workshop_records WHERE tenant_id=? AND store_id=? AND kind=? AND id=?`, a.TenantID, a.StoreID, kind, id).Scan(&rev, &state, &requestID, &body)
	if errors.Is(e, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if e != nil {
		return out, e
	}
	if json.Unmarshal([]byte(body), &out) != nil || out.Kind != kind || out.ID != id || out.Revision != rev || out.State != state || out.RequestID != requestID || out.Repeated {
		return out, ErrConflict
	}
	var n, min, max int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(revision),0),COALESCE(max(revision),0) FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND kind=? AND id=?`, a.TenantID, a.StoreID, kind, id).Scan(&n, &min, &max); e != nil {
		return out, e
	}
	if n != rev || min != 1 || max != rev {
		return out, ErrConflict
	}
	proof, e := workshopReceipt(ctx, tx, a, out.DeviceID, out.OperationID, false)
	if e != nil {
		return out, e
	}
	if !bytes.Equal(encode(proof), encode(out)) {
		return out, ErrConflict
	}
	return out, nil
}
func WorkshopMutate(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, kind string, in Input) (WorkshopResult, error) {
	in, raw, hash, e := workshopPrepare(kind, in)
	if e != nil {
		return WorkshopResult{}, e
	}
	tx, manager, e := begin(ctx, db, c, a, d, false, oneOf(kind, "template", "binding"))
	if e != nil {
		return WorkshopResult{}, e
	}
	defer tx.Rollback()
	var baseCount int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID).Scan(&baseCount); e != nil {
		return WorkshopResult{}, e
	}
	if baseCount != 0 {
		return WorkshopResult{}, ErrConflict
	}
	var prior, actor string
	e = tx.QueryRowContext(ctx, `SELECT request_hash,actor_id FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID).Scan(&prior, &actor)
	if e == nil {
		if prior != hash || actor != a.IdentityID {
			return WorkshopResult{}, ErrConflict
		}
		out, e := workshopReceipt(ctx, tx, a, d.DeviceID, in.OperationID, true)
		if e == nil {
			e = tx.Commit()
		}
		out.Repeated = true
		return out, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return WorkshopResult{}, e
	}
	out := WorkshopResult{OperationID: in.OperationID, Kind: kind, ID: in.ID, Revision: in.ExpectedRevision + 1, ActorID: a.IdentityID, DeviceID: d.DeviceID, RecordedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	old, e := workshopLoad(ctx, tx, a, kind, in.ID)
	if in.ExpectedRevision == 0 {
		if e == nil {
			return out, ErrConflict
		}
		if !errors.Is(e, ErrNotFound) {
			return out, e
		}
	} else {
		if e != nil {
			return out, e
		}
		if old.Revision != in.ExpectedRevision {
			return out, ErrConflict
		}
		out.RequestID = old.RequestID
		out.Data = old.Data
		out.State = old.State
	}
	if e = workshopChange(ctx, tx, a, d, manager, in, old, &out); e != nil {
		return out, e
	}
	if e = one(ctx, tx, `INSERT INTO fleet_workshop_operations(tenant_id,store_id,device_id,operation_id,actor_id,kind,id,revision,request_hash,request_json,response_json) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, a.IdentityID, kind, in.ID, out.Revision, hash, raw, string(encode(out))); e != nil {
		return out, e
	}
	if in.ExpectedRevision == 0 {
		e = one(ctx, tx, `INSERT INTO fleet_workshop_records(tenant_id,store_id,kind,id,request_id,revision,state,body_json) VALUES(?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, kind, in.ID, nullable(out.RequestID), out.Revision, out.State, string(encode(out)))
	} else {
		e = one(ctx, tx, `UPDATE fleet_workshop_records SET revision=?,state=?,body_json=? WHERE tenant_id=? AND store_id=? AND kind=? AND id=? AND revision=?`, out.Revision, out.State, string(encode(out)), a.TenantID, a.StoreID, kind, in.ID, in.ExpectedRevision)
	}
	if e != nil {
		return out, e
	}
	eventID, e := localdb.NewID()
	if e != nil {
		return out, e
	}
	payload := encode(struct {
		Kind      string `json:"kind"`
		ID        string `json:"id"`
		RequestID string `json:"request_id"`
		Revision  int64  `json:"revision"`
		State     string `json:"state"`
	}{kind, in.ID, out.RequestID, out.Revision, out.State})
	if e = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'fleet.workshop.changed',1,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, "workshop:"+kind+":"+in.ID, string(payload), out.RecordedAt); e != nil {
		return out, e
	}
	return out, tx.Commit()
}
