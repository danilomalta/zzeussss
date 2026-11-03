package fleet

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

func workshopRequest(ctx context.Context, tx *sql.Tx, a identity.Scope, id string, rev int64) (Record, error) {
	if !validRequestRevision(id, rev) {
		return Record{}, ErrInvalid
	}
	r, e := load(ctx, tx, a, "request", id)
	if e != nil {
		return r, e
	}
	if e = proof(ctx, tx, a, r); e != nil {
		return r, e
	}
	if r.Revision != rev {
		return r, ErrConflict
	}
	return r, nil
}
func workshopChange(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, manager bool, in Input, old WorkshopResult, out *WorkshopResult) error {
	if in.ExpectedRevision == 0 && in.Action != "create" || in.ExpectedRevision > 0 && in.Action == "create" {
		return ErrInvalid
	}
	switch out.Kind {
	case "template":
		if in.Action == "create" {
			v, e := templateDecode(in.Data)
			if e != nil {
				return e
			}
			out.Data = encode(v)
			out.State = "active"
			return nil
		}
		if in.Action != "disable" || old.State != "active" || empty(in.Data) != nil {
			return ErrInvalid
		}
		out.State = "inactive"
		return nil
	case "binding":
		if !oneOf(in.Action, "create", "replace") {
			return ErrInvalid
		}
		var v BindingInput
		if decode(in.Data, &v, "request_id", "expected_request_revision", "template_id") != nil || !validRequestRevision(v.RequestID, v.ExpectedRequestRevision) || !ID(v.TemplateID) || in.ID != v.RequestID {
			return ErrInvalid
		}
		r, e := workshopRequest(ctx, tx, a, v.RequestID, v.ExpectedRequestRevision)
		if e != nil {
			return e
		}
		if !oneOf(r.State, "open", "triaged", "approved") {
			return ErrConflict
		}
		template, e := workshopLoad(ctx, tx, a, "template", v.TemplateID)
		if e != nil {
			return e
		}
		if template.State != "active" {
			return ErrConflict
		}
		var config CheckTemplate
		if json.Unmarshal(template.Data, &config) != nil {
			return ErrConflict
		}
		out.RequestID = v.RequestID
		out.State = "required"
		out.Data = encode(CheckBinding{v, config})
		return nil
	case "inspection":
		if in.Action != "create" {
			return ErrInvalid
		}
		v, e := inspectionDecode(in.Data)
		if e != nil {
			return e
		}
		r, e := workshopRequest(ctx, tx, a, v.RequestID, v.ExpectedRequestRevision)
		if e != nil {
			return e
		}
		if !manager && r.AssignedTo != a.IdentityID {
			return identity.ErrDenied
		}
		if !oneOf(r.State, "in_service", "inspection") {
			return ErrConflict
		}
		binding, e := workshopLoad(ctx, tx, a, "binding", v.RequestID)
		if e != nil {
			return e
		}
		if binding.Revision != v.ExpectedBindingRevision {
			return ErrConflict
		}
		var policy CheckBinding
		if json.Unmarshal(binding.Data, &policy) != nil {
			return ErrConflict
		}
		if len(v.Answers) != len(policy.Template.Items) {
			return ErrInvalid
		}
		byID := map[string]CheckAnswer{}
		for _, answer := range v.Answers {
			byID[answer.ItemID] = answer
		}
		passed := true
		ordered := []CheckAnswer{}
		for _, item := range policy.Template.Items {
			answer, ok := byID[item.ID]
			if !ok {
				return ErrInvalid
			}
			if answer.Outcome == "fail" || item.Required && answer.Outcome != "pass" {
				passed = false
			}
			ordered = append(ordered, answer)
		}
		v.Answers = ordered
		epoch, e := workshopEpoch(ctx, tx, a, r)
		if e != nil {
			return e
		}
		if epoch == 0 {
			return ErrConflict
		}
		out.RequestID = v.RequestID
		out.State = "failed"
		if passed {
			out.State = "passed"
		}
		out.Data = encode(Inspection{v, policy.TemplateID, epoch, passed})
		return nil
	case "labor":
		if in.Action == "create" {
			v, seconds, e := laborDecode(in.Data)
			if e != nil {
				return e
			}
			r, e := workshopRequest(ctx, tx, a, v.RequestID, v.ExpectedRequestRevision)
			if e != nil {
				return e
			}
			if !manager && r.AssignedTo != a.IdentityID {
				return identity.ErrDenied
			}
			if !oneOf(r.State, "in_service", "waiting_parts", "inspection") {
				return ErrConflict
			}
			if e = laborOverlap(ctx, tx, a, a.IdentityID, v.StartedAt, v.EndedAt); e != nil {
				return e
			}
			out.RequestID = v.RequestID
			out.State = "declared"
			out.Data = encode(Labor{v, a.IdentityID, seconds})
			return nil
		}
		if empty(in.Data) != nil {
			return ErrInvalid
		}
		var prior Labor
		if json.Unmarshal(old.Data, &prior) != nil {
			return ErrConflict
		}
		r, e := load(ctx, tx, a, "request", old.RequestID)
		if e != nil {
			return e
		}
		if e = proof(ctx, tx, a, r); e != nil {
			return e
		}
		switch in.Action {
		case "confirm", "reject":
			if !manager {
				return identity.ErrDenied
			}
			if old.State != "declared" {
				return ErrConflict
			}
			if in.Action == "confirm" {
				out.State = "confirmed"
			} else {
				out.State = "rejected"
			}
		case "void":
			if old.State == "voided" {
				return ErrConflict
			}
			if !manager && (prior.WorkerID != a.IdentityID || old.State != "declared" || !oneOf(r.State, "in_service", "waiting_parts", "inspection")) {
				return identity.ErrDenied
			}
			out.State = "voided"
		default:
			return ErrInvalid
		}
		return nil
	}
	return ErrInvalid
}
func workshopRows(ctx context.Context, tx *sql.Tx, a identity.Scope, where string, args []any, limit int) ([]WorkshopResult, error) {
	var count int
	base := `tenant_id=? AND store_id=? AND ` + where
	params := append([]any{a.TenantID, a.StoreID}, args...)
	if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_workshop_records WHERE `+base, params...).Scan(&count); e != nil {
		return nil, e
	}
	if count > limit {
		return nil, ErrWorkshopLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT kind,id FROM fleet_workshop_records WHERE `+base+` ORDER BY kind,id`, params...)
	if e != nil {
		return nil, e
	}
	type key struct{ kind, id string }
	keys := []key{}
	for rows.Next() {
		var k key
		if e = rows.Scan(&k.kind, &k.id); e != nil {
			rows.Close()
			return nil, e
		}
		keys = append(keys, k)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	all := []WorkshopResult{}
	for _, k := range keys {
		r, e := workshopLoad(ctx, tx, a, k.kind, k.id)
		if e != nil {
			return nil, e
		}
		all = append(all, r)
	}
	return all, nil
}
func laborOverlap(ctx context.Context, tx *sql.Tx, a identity.Scope, worker, from, to string) error {
	all, e := workshopRows(ctx, tx, a, `kind='labor' AND json_extract(body_json,'$.data.worker_id')=?`, []any{worker}, MaxWorkshopSource)
	if e != nil {
		return e
	}
	var opCount int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND kind='labor' AND revision=1 AND json_extract(response_json,'$.data.worker_id')=?`, a.TenantID, a.StoreID, worker).Scan(&opCount); e != nil {
		return e
	}
	if opCount != len(all) {
		return ErrConflict
	}
	s, _ := time.Parse(time.RFC3339, from)
	end, _ := time.Parse(time.RFC3339, to)
	for _, r := range all {
		var v Labor
		if json.Unmarshal(r.Data, &v) != nil {
			return ErrConflict
		}
		if v.WorkerID != worker || !oneOf(r.State, "declared", "confirmed") {
			continue
		}
		start, e := time.Parse(time.RFC3339, v.StartedAt)
		if e != nil {
			return ErrConflict
		}
		stop, e := time.Parse(time.RFC3339, v.EndedAt)
		if e != nil {
			return ErrConflict
		}
		if start.Before(end) && stop.After(s) {
			return ErrConflict
		}
	}
	return nil
}

// A new start, reassignment or reopening invalidates prior technical evidence.
func workshopEpoch(ctx context.Context, tx *sql.Tx, a identity.Scope, r Record) (int64, error) {
	if r.Revision > MaxWorkshopSource {
		return 0, ErrWorkshopLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT device_id,operation_id,action,revision FROM fleet_events WHERE tenant_id=? AND store_id=? AND kind='request' AND record_id=? ORDER BY revision`, a.TenantID, a.StoreID, r.ID)
	if e != nil {
		return 0, e
	}
	type event struct {
		device, op, action string
		rev                int64
	}
	events := []event{}
	for rows.Next() {
		var v event
		if e = rows.Scan(&v.device, &v.op, &v.action, &v.rev); e != nil {
			rows.Close()
			return 0, e
		}
		events = append(events, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, e
	}
	if int64(len(events)) != r.Revision {
		return 0, ErrConflict
	}
	var epoch int64
	for _, v := range events {
		out, e := receipt(ctx, tx, a, v.device, v.op, false)
		if e != nil {
			return 0, e
		}
		if out.Record == nil || out.Record.ID != r.ID || out.Record.Kind != "request" || out.Record.Revision != v.rev {
			return 0, ErrConflict
		}
		var raw string
		if e = tx.QueryRowContext(ctx, `SELECT request_json FROM fleet_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, v.device, v.op).Scan(&raw); e != nil {
			return 0, e
		}
		var request struct {
			Input Input `json:"input"`
		}
		if json.Unmarshal([]byte(raw), &request) != nil || request.Input.Action != v.action {
			return 0, ErrConflict
		}
		if oneOf(v.action, "start", "assign", "reopen") {
			epoch = v.rev
		}
	}
	return epoch, nil
}
func inspectionGate(ctx context.Context, tx *sql.Tx, a identity.Scope, r Record) (bool, error) {
	binding, e := workshopLoad(ctx, tx, a, "binding", r.ID)
	if errors.Is(e, ErrNotFound) {
		var n int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND kind='binding' AND id=?`, a.TenantID, a.StoreID, r.ID).Scan(&n); e != nil {
			return false, e
		}
		if n != 0 {
			return false, ErrConflict
		}
		return true, nil
	}
	if e != nil {
		return false, e
	}
	all, e := workshopRows(ctx, tx, a, `kind='inspection' AND request_id=?`, []any{r.ID}, MaxWorkshopSource)
	if e != nil {
		return false, e
	}
	var n int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND kind='inspection' AND revision=1 AND json_extract(response_json,'$.request_id')=?`, a.TenantID, a.StoreID, r.ID).Scan(&n); e != nil {
		return false, e
	}
	if n != len(all) {
		return false, ErrConflict
	}
	epoch, e := workshopEpoch(ctx, tx, a, r)
	if e != nil {
		return false, e
	}
	var latest WorkshopResult
	var seq int64
	for _, v := range all {
		var n int64
		if e = tx.QueryRowContext(ctx, `SELECT seq FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND kind='inspection' AND id=? AND revision=1`, a.TenantID, a.StoreID, v.ID).Scan(&n); e != nil {
			return false, e
		}
		if n > seq {
			seq = n
			latest = v
		}
	}
	if seq == 0 {
		return false, nil
	}
	var result Inspection
	if json.Unmarshal(latest.Data, &result) != nil {
		return false, ErrConflict
	}
	return latest.State == "passed" && result.Passed && result.ExpectedBindingRevision == binding.Revision && result.EpochRevision == epoch, nil
}
